package text

import (
	"bytes"
	"fmt"
	"io"
)

// IsTextReader detects if an io.Reader probably contains text content.
// It returns a new io.Reader that preserves all original data and a boolean
// indicating if the content is likely text. Empty content is considered text.
// Detection is similar to Git's approach.
func IsTextReader(r io.Reader) (io.Reader, bool, error) {
	// Read buffer for detection (Git typically uses 8000 bytes)
	buf := make([]byte, 8192)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return nil, false, fmt.Errorf("failed to read for text detection: %w", err)
	}

	// Trim buffer to actual read size
	buf = buf[:n]

	// Create new reader that returns detection bytes first, then continues with original
	newReader := io.MultiReader(bytes.NewReader(buf), r)

	// Detect if content is text
	isText := isTextContent(buf)

	return newReader, isText, nil
}

// isTextContent determines if the given bytes represent text content.
// Uses Git-like heuristics: primarily looks for null bytes, but considers
// encoding patterns for UTF-16/UTF-32.
func isTextContent(data []byte) bool {
	// Empty is considered text
	if len(data) == 0 {
		return true
	}

	// Check for BOMs first - if present, it's definitely text
	if bytes.HasPrefix(data, bomUTF8) ||
		bytes.HasPrefix(data, bomUTF16LE) ||
		bytes.HasPrefix(data, bomUTF16BE) ||
		bytes.HasPrefix(data, bomUTF32LE) ||
		bytes.HasPrefix(data, bomUTF32BE) {
		return true
	}

	// Count null bytes and their positions
	nullCount := 0
	nullPositions := make([]int, 0)

	for i, b := range data {
		if b == 0 {
			nullCount++
			nullPositions = append(nullPositions, i)
		}
	}

	// No null bytes - definitely text
	if nullCount == 0 {
		return true
	}

	// If more than 75% null bytes, probably binary
	// UTF-32 has a maximum of 3/4 null bytes
	if float64(nullCount)/float64(len(data)) > 0.75 {
		return false
	}

	// Check if null bytes follow UTF-16 LE pattern (odd positions)
	if len(data) >= 4 && nullCount > 1 {
		utf16LEPattern := true
		for _, pos := range nullPositions {
			if pos%2 == 0 { // null in even position breaks UTF-16 LE pattern
				utf16LEPattern = false
				break
			}
		}
		if utf16LEPattern && float64(nullCount) > float64(len(data))/10 {
			return true // Likely UTF-16 LE
		}
	}

	// Check if null bytes follow UTF-16 BE pattern (even positions)
	if len(data) >= 4 && nullCount > 1 {
		utf16BEPattern := true
		for _, pos := range nullPositions {
			if pos%2 == 1 { // null in odd position breaks UTF-16 BE pattern
				utf16BEPattern = false
				break
			}
		}
		if utf16BEPattern && float64(nullCount) > float64(len(data))/10 {
			return true // Likely UTF-16 BE
		}
	}

	// Check for UTF-32 patterns (nulls in groups of 3)
	if len(data) >= 8 && nullCount >= 3 {
		// UTF-32 LE: nulls at positions 1,2,3 then 5,6,7 etc
		utf32LEPattern := true
		for i := 1; i < len(data)-2 && utf32LEPattern; i += 4 {
			if i+2 < len(data) {
				if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 {
					continue // Good UTF-32 LE pattern
				}
			}
			utf32LEPattern = false
		}
		if utf32LEPattern {
			return true
		}

		// UTF-32 BE: nulls at positions 0,1,2 then 4,5,6 etc
		utf32BEPattern := true
		for i := 0; i < len(data)-2 && utf32BEPattern; i += 4 {
			if i+2 < len(data) {
				if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 {
					continue // Good UTF-32 BE pattern
				}
			}
			utf32BEPattern = false
		}
		if utf32BEPattern {
			return true
		}
	}

	// Git's approach: if we have null bytes that don't fit UTF-16/32 patterns,
	// and they appear in the first reasonable chunk, it's probably binary
	if nullCount > 0 {
		// Check if nulls appear early (first 1KB) - more likely to be binary
		earlyNulls := 0
		checkLen := len(data)
		if checkLen > 1024 {
			checkLen = 1024
		}

		for i := 0; i < checkLen; i++ {
			if data[i] == 0 {
				earlyNulls++
			}
		}

		// If we have null bytes in first 1KB that don't follow encoding patterns,
		// it's probably binary
		if earlyNulls > 0 {
			return false
		}
	}

	// Default to text if we can't determine otherwise
	return true
}
