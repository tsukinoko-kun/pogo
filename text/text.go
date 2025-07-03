package text

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/encoding/unicode/utf32"
)

type Encoding int

const (
	UTF8 Encoding = iota
	UTF16LE
	UTF16BE
	UTF32LE
	UTF32BE
	Windows1252
	ShiftJIS
	Unknown
)

var encodingNames = map[Encoding]string{
	UTF8:        "UTF-8",
	UTF16LE:     "UTF-16 LE",
	UTF16BE:     "UTF-16 BE",
	UTF32LE:     "UTF-32 LE",
	UTF32BE:     "UTF-32 BE",
	Windows1252: "Windows-1252",
	ShiftJIS:    "Shift JIS",
	Unknown:     "Unknown",
}

func (e Encoding) String() string {
	if name, ok := encodingNames[e]; ok {
		return name
	}
	return "Unknown"
}

// Text represents text content with its original encoding information.
type Text struct {
	content  string   // UTF-8 content
	encoding Encoding // original encoding
	hasBOM   bool     // whether original had BOM
}

// NewText creates a new Text with UTF-8 encoding and no BOM.
func NewText(content string) *Text {
	return &Text{
		content:  content,
		encoding: UTF8,
		hasBOM:   false,
	}
}

// NewTextWithEncoding creates a new Text with the given encoding and no BOM.
func NewTextWithEncoding(content string, encoding Encoding) *Text {
	return &Text{
		content:  content,
		encoding: encoding,
		hasBOM:   false,
	}
}

// Content returns the UTF-8 string content.
func (t *Text) Content() string {
	if t == nil {
		return ""
	}
	return t.content
}

// Encoding returns the detected or original encoding.
func (t *Text) Encoding() Encoding {
	return t.encoding
}

// HasBOM returns whether the original text had a BOM.
func (t *Text) HasBOM() bool {
	return t.hasBOM
}

// String returns the UTF-8 content.
func (t *Text) String() string {
	if t == nil {
		return ""
	}
	return t.content
}

func (t *Text) Utf8Reader() io.Reader {
	return bytes.NewReader([]byte(t.content))
}

// BOM signatures
var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
	bomUTF32LE = []byte{0xFF, 0xFE, 0x00, 0x00}
	bomUTF32BE = []byte{0x00, 0x00, 0xFE, 0xFF}
)

// ReadFrom reads text from an io.Reader and detects its encoding.
func ReadFrom(r io.Reader) (*Text, error) {
	// Read initial buffer for detection
	buf := make([]byte, 0, 1024)
	initial := make([]byte, 64) // Read first 64 bytes for detection

	n, err := r.Read(initial)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read initial bytes: %w", err)
	}

	buf = append(buf, initial[:n]...)

	// If we didn't get EOF, read the rest
	if err != io.EOF {
		rest, readErr := io.ReadAll(r)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read remaining bytes: %w", readErr)
		}
		buf = append(buf, rest...)
	}

	// Handle empty content
	if len(buf) == 0 {
		return &Text{
			content:  "",
			encoding: UTF8,
			hasBOM:   false,
		}, nil
	}

	return decodeBytes(buf)
}

// decodeBytes detects encoding and decodes bytes to UTF-8 string.
func decodeBytes(data []byte) (*Text, error) {
	if len(data) == 0 {
		return &Text{content: "", encoding: UTF8, hasBOM: false}, nil
	}

	if bytes.HasPrefix(data, bomUTF32LE) {
		return decodeUTF32LE(data[4:], true)
	}
	if bytes.HasPrefix(data, bomUTF32BE) {
		return decodeUTF32BE(data[4:], true)
	}
	if bytes.HasPrefix(data, bomUTF16LE) {
		return decodeUTF16LE(data[2:], true)
	}
	if bytes.HasPrefix(data, bomUTF16BE) {
		return decodeUTF16BE(data[2:], true)
	}
	if bytes.HasPrefix(data, bomUTF8) {
		return decodeUTF8(data[3:], true)
	}

	if len(data) >= 2 && hasUTF16LEPattern(data) {
		return decodeUTF16LE(data, false)
	}

	if len(data) >= 2 && hasUTF16BEPattern(data) {
		return decodeUTF16BE(data, false)
	}

	if len(data) >= 4 {
		if hasUTF32LEPattern(data) {
			return decodeUTF32LE(data, false)
		}
		if hasUTF32BEPattern(data) {
			return decodeUTF32BE(data, false)
		}
	}

	if utf8.Valid(data) {
		return decodeUTF8(data, false)
	}

	return nil, errors.New("failed to detect encoding")
}

// Heuristic functions
func hasUTF16LEPattern(data []byte) bool {
	if len(data) < 4 || len(data)%2 != 0 {
		return false
	}

	nullCount := 0
	totalChecked := 0
	for i := 1; i < len(data) && i < 50; i += 2 {
		if data[i] == 0 {
			nullCount++
		}
		totalChecked++
	}

	// Need at least 2 characters and more than 30% null bytes in odd positions
	return totalChecked >= 2 && nullCount > totalChecked/3
}

func hasUTF16BEPattern(data []byte) bool {
	nullCount := 0
	for i := 0; i < len(data) && i < 100; i += 2 {
		if data[i] == 0 {
			nullCount++
		}
	}
	return len(data) > 10 && nullCount > len(data)/20 // At least 5% null bytes in even positions
}

func hasUTF32LEPattern(data []byte) bool {
	nullCount := 0
	for i := 1; i < len(data) && i < 100; i += 4 {
		if i+2 < len(data) && data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 {
			nullCount++
		}
	}
	return len(data) > 20 && nullCount > 0
}

func hasUTF32BEPattern(data []byte) bool {
	nullCount := 0
	for i := 0; i < len(data) && i < 100; i += 4 {
		if i+2 < len(data) && data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 {
			nullCount++
		}
	}
	return len(data) > 20 && nullCount > 0
}

// Decode functions
func decodeUTF8(data []byte, hasBOM bool) (*Text, error) {
	return &Text{
		content:  string(data),
		encoding: UTF8,
		hasBOM:   hasBOM,
	}, nil
}

func decodeUTF16LE(data []byte, hasBOM bool) (*Text, error) {
	decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
	decoded, err := decoder.Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("UTF-16 LE decode error: %w", err)
	}
	return &Text{
		content:  string(decoded),
		encoding: UTF16LE,
		hasBOM:   hasBOM,
	}, nil
}

func decodeUTF16BE(data []byte, hasBOM bool) (*Text, error) {
	decoder := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()
	decoded, err := decoder.Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("UTF-16 BE decode error: %w", err)
	}
	return &Text{
		content:  string(decoded),
		encoding: UTF16BE,
		hasBOM:   hasBOM,
	}, nil
}

func decodeUTF32LE(data []byte, hasBOM bool) (*Text, error) {
	decoder := utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM).NewDecoder()
	decoded, err := decoder.Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("UTF-32 LE decode error: %w", err)
	}
	return &Text{
		content:  string(decoded),
		encoding: UTF32LE,
		hasBOM:   hasBOM,
	}, nil
}

func decodeUTF32BE(data []byte, hasBOM bool) (*Text, error) {
	decoder := utf32.UTF32(utf32.BigEndian, utf32.IgnoreBOM).NewDecoder()
	decoded, err := decoder.Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("UTF-32 BE decode error: %w", err)
	}
	return &Text{
		content:  string(decoded),
		encoding: UTF32BE,
		hasBOM:   hasBOM,
	}, nil
}

// WriteTo writes the text to an io.Writer using its original encoding.
func (t *Text) WriteTo(w io.Writer) (int64, error) {
	var data []byte
	var err error

	// Encode back to original encoding
	switch t.encoding {
	case UTF8:
		data = []byte(t.content)
	case UTF16LE:
		data, err = encodeUTF16LE(t.content)
	case UTF16BE:
		data, err = encodeUTF16BE(t.content)
	case UTF32LE:
		data, err = encodeUTF32LE(t.content)
	case UTF32BE:
		data, err = encodeUTF32BE(t.content)
	case Windows1252:
		data, err = encodeWindows1252(t.content)
	case ShiftJIS:
		data, err = encodeShiftJIS(t.content)
	default:
		return 0, errors.New("unsupported encoding for writing")
	}

	if err != nil {
		return 0, fmt.Errorf("encoding error: %w", err)
	}

	written := int64(0)

	// Write BOM if it was present originally
	if t.hasBOM {
		var bom []byte
		switch t.encoding {
		case UTF8:
			bom = bomUTF8
		case UTF16LE:
			bom = bomUTF16LE
		case UTF16BE:
			bom = bomUTF16BE
		case UTF32LE:
			bom = bomUTF32LE
		case UTF32BE:
			bom = bomUTF32BE
		}

		if bom != nil {
			n, err := w.Write(bom)
			if err != nil {
				return int64(n), err
			}
			written += int64(n)
		}
	}

	// Write content
	n, err := w.Write(data)
	written += int64(n)
	return written, err
}

type errorReader struct {
	err error
}

func (r errorReader) Read(p []byte) (int, error) {
	return 0, r.err
}

// Reader returns an io.Reader that reads the text content in its original encoding.
func (t *Text) Reader() io.Reader {
	var data []byte
	var err error

	// Encode back to original encoding
	switch t.encoding {
	case UTF8:
		data = []byte(t.content)
	case UTF16LE:
		data, err = encodeUTF16LE(t.content)
	case UTF16BE:
		data, err = encodeUTF16BE(t.content)
	case UTF32LE:
		data, err = encodeUTF32LE(t.content)
	case UTF32BE:
		data, err = encodeUTF32BE(t.content)
	case Windows1252:
		data, err = encodeWindows1252(t.content)
	case ShiftJIS:
		data, err = encodeShiftJIS(t.content)
	default:
		return errorReader{errors.New("unsupported encoding for writing")}
	}

	if err != nil {
		return errorReader{fmt.Errorf("encoding error: %w", err)}
	}

	// Write BOM if it was present originally
	if t.hasBOM {
		var bom []byte
		switch t.encoding {
		case UTF8:
			bom = bomUTF8
		case UTF16LE:
			bom = bomUTF16LE
		case UTF16BE:
			bom = bomUTF16BE
		case UTF32LE:
			bom = bomUTF32LE
		case UTF32BE:
			bom = bomUTF32BE
		}

		if bom != nil {
			return io.MultiReader(bytes.NewReader(bom), bytes.NewReader(data))
		}
	}

	return bytes.NewReader(data)
}

// Encode functions
func encodeUTF16LE(content string) ([]byte, error) {
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	return encoder.Bytes([]byte(content))
}

func encodeUTF16BE(content string) ([]byte, error) {
	encoder := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()
	return encoder.Bytes([]byte(content))
}

func encodeUTF32LE(content string) ([]byte, error) {
	encoder := utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM).NewEncoder()
	return encoder.Bytes([]byte(content))
}

func encodeUTF32BE(content string) ([]byte, error) {
	encoder := utf32.UTF32(utf32.BigEndian, utf32.IgnoreBOM).NewEncoder()
	return encoder.Bytes([]byte(content))
}

func encodeWindows1252(content string) ([]byte, error) {
	encoder := charmap.Windows1252.NewEncoder()
	return encoder.Bytes([]byte(content))
}

func encodeShiftJIS(content string) ([]byte, error) {
	encoder := japanese.ShiftJIS.NewEncoder()
	return encoder.Bytes([]byte(content))
}
