package text

import (
	"bytes"
	"testing"
)

func TestEmptyContent(t *testing.T) {
	text, err := ReadFrom(bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if text.Content() != "" {
		t.Errorf("Expected empty content, got %q", text.Content())
	}

	if text.Encoding() != UTF8 {
		t.Errorf("Expected UTF8 encoding, got %v", text.Encoding())
	}

	if text.HasBOM() {
		t.Errorf("Expected no BOM, got true")
	}
}

func TestUTF8WithoutBOM(t *testing.T) {
	content := "Hello, 世界!"
	data := []byte(content)

	text, err := ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if text.Content() != content {
		t.Errorf("Expected content %q, got %q", content, text.Content())
	}

	if text.Encoding() != UTF8 {
		t.Errorf("Expected UTF8 encoding, got %v", text.Encoding())
	}

	if text.HasBOM() {
		t.Errorf("Expected no BOM, got true")
	}
}

func TestUTF8WithBOM(t *testing.T) {
	content := "Hello, 世界!"
	data := append(bomUTF8, []byte(content)...)

	text, err := ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if text.Content() != content {
		t.Errorf("Expected content %q, got %q", content, text.Content())
	}

	if text.Encoding() != UTF8 {
		t.Errorf("Expected UTF8 encoding, got %v", text.Encoding())
	}

	if !text.HasBOM() {
		t.Errorf("Expected BOM, got false")
	}
}

func TestUTF16LEWithBOM(t *testing.T) {
	// "Hello" in UTF-16 LE
	content := "Hello"
	data := []byte{0xFF, 0xFE, // BOM
		0x48, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F, 0x00}

	text, err := ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if text.Content() != content {
		t.Errorf("Expected content %q, got %q", content, text.Content())
	}

	if text.Encoding() != UTF16LE {
		t.Errorf("Expected UTF16LE encoding, got %v", text.Encoding())
	}

	if !text.HasBOM() {
		t.Errorf("Expected BOM, got false")
	}
}

func TestUTF16BEWithBOM(t *testing.T) {
	// "Hello" in UTF-16 BE
	content := "Hello"
	data := []byte{0xFE, 0xFF, // BOM
		0x00, 0x48, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F}

	text, err := ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if text.Content() != content {
		t.Errorf("Expected content %q, got %q", content, text.Content())
	}

	if text.Encoding() != UTF16BE {
		t.Errorf("Expected UTF16BE encoding, got %v", text.Encoding())
	}

	if !text.HasBOM() {
		t.Errorf("Expected BOM, got false")
	}
}

func TestUTF16LEWithoutBOM(t *testing.T) {
	// "Hello" in UTF-16 LE (without BOM, should be detected by null pattern)
	content := "Hello"
	data := []byte{0x48, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F, 0x00}

	text, err := ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if text.Content() != content {
		t.Errorf("Expected content %q, got %q", content, text.Content())
	}

	if text.Encoding() != UTF16LE {
		t.Errorf("Expected UTF16LE encoding, got %v", text.Encoding())
	}

	if text.HasBOM() {
		t.Errorf("Expected no BOM, got true")
	}
}

func TestUTF32LEWithBOM(t *testing.T) {
	// "Hi" in UTF-32 LE
	content := "Hi"
	data := []byte{0xFF, 0xFE, 0x00, 0x00, // BOM
		0x48, 0x00, 0x00, 0x00, 0x69, 0x00, 0x00, 0x00}

	text, err := ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if text.Content() != content {
		t.Errorf("Expected content %q, got %q", content, text.Content())
	}

	if text.Encoding() != UTF32LE {
		t.Errorf("Expected UTF32LE encoding, got %v", text.Encoding())
	}

	if !text.HasBOM() {
		t.Errorf("Expected BOM, got false")
	}
}

func TestUTF32BEWithBOM(t *testing.T) {
	// "Hi" in UTF-32 BE
	content := "Hi"
	data := []byte{0x00, 0x00, 0xFE, 0xFF, // BOM
		0x00, 0x00, 0x00, 0x48, 0x00, 0x00, 0x00, 0x69}

	text, err := ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if text.Content() != content {
		t.Errorf("Expected content %q, got %q", content, text.Content())
	}

	if text.Encoding() != UTF32BE {
		t.Errorf("Expected UTF32BE encoding, got %v", text.Encoding())
	}

	if !text.HasBOM() {
		t.Errorf("Expected BOM, got false")
	}
}

func TestNewText(t *testing.T) {
	content := "Test content"
	text := NewText(content)

	if text.Content() != content {
		t.Errorf("Expected content %q, got %q", content, text.Content())
	}

	if text.Encoding() != UTF8 {
		t.Errorf("Expected UTF8 encoding, got %v", text.Encoding())
	}

	if text.HasBOM() {
		t.Errorf("Expected no BOM, got true")
	}
}

func TestSmallContent(t *testing.T) {
	// Test with very small content (single character)
	content := "A"
	data := []byte(content)

	text, err := ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if text.Content() != content {
		t.Errorf("Expected content %q, got %q", content, text.Content())
	}

	if text.Encoding() != UTF8 {
		t.Errorf("Expected UTF8 encoding, got %v", text.Encoding())
	}

	if text.HasBOM() {
		t.Errorf("Expected no BOM, got true")
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that writing produces the same bytes as reading
	testCases := []struct {
		name string
		data []byte
	}{
		{"UTF-8 with BOM", append(bomUTF8, []byte("Hello")...)},
		{"UTF-8 without BOM", []byte("Hello")},
		{"UTF-16 LE with BOM", []byte{0xFF, 0xFE, 0x48, 0x00, 0x69, 0x00}},
		{"Empty", []byte{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read
			text, err := ReadFrom(bytes.NewReader(tc.data))
			if err != nil {
				t.Fatalf("Expected no error reading, got %v", err)
			}

			// Write
			var buf bytes.Buffer
			_, err = text.WriteTo(&buf)
			if err != nil {
				t.Fatalf("Expected no error writing, got %v", err)
			}

			// Compare
			if !bytes.Equal(tc.data, buf.Bytes()) {
				t.Errorf("Round-trip failed:\nOriginal: %v\nResult:   %v", tc.data, buf.Bytes())
			}
		})
	}
}
