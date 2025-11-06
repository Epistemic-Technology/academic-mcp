package documents

import (
	"testing"
)

func TestDetectDocumentType(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "PDF document",
			data:     []byte("%PDF-1.4\nsome pdf content"),
			expected: "pdf",
		},
		{
			name:     "HTML with DOCTYPE",
			data:     []byte("<!DOCTYPE html><html><body>test</body></html>"),
			expected: "html",
		},
		{
			name:     "HTML with lowercase DOCTYPE",
			data:     []byte("<!doctype html><html><body>test</body></html>"),
			expected: "html",
		},
		{
			name:     "HTML without DOCTYPE",
			data:     []byte("<html><body>test</body></html>"),
			expected: "html",
		},
		{
			name:     "HTML with whitespace",
			data:     []byte("  \n  <!DOCTYPE html><html><body>test</body></html>"),
			expected: "html",
		},
		{
			name:     "DOCX (ZIP with word/ directory)",
			data:     append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("word/document.xml")...),
			expected: "docx",
		},
		{
			name:     "ZIP file (not DOCX)",
			data:     []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00},
			expected: "zip",
		},
		{
			name:     "Markdown with heading",
			data:     []byte("# Title\n\nSome markdown content"),
			expected: "md",
		},
		{
			name:     "Markdown with code block",
			data:     []byte("```go\nfunc main() {}\n```"),
			expected: "md",
		},
		{
			name:     "Plain text",
			data:     []byte("This is just plain text content"),
			expected: "txt",
		},
		{
			name:     "Binary data",
			data:     []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
			expected: "unknown",
		},
		{
			name:     "Empty data",
			data:     []byte{},
			expected: "unknown",
		},
		{
			name:     "Very short data",
			data:     []byte("ab"),
			expected: "txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectDocumentType(tt.data)
			if result != tt.expected {
				t.Errorf("DetectDocumentType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsLikelyText(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "Plain text",
			data:     []byte("This is plain text with spaces and punctuation!"),
			expected: true,
		},
		{
			name:     "Text with newlines",
			data:     []byte("Line 1\nLine 2\nLine 3"),
			expected: true,
		},
		{
			name:     "Text with tabs",
			data:     []byte("Column1\tColumn2\tColumn3"),
			expected: true,
		},
		{
			name:     "Binary with null byte",
			data:     []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F, 0x00, 0x57, 0x6F, 0x72, 0x6C, 0x64},
			expected: false,
		},
		{
			name:     "Mostly binary data",
			data:     []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
			expected: false,
		},
		{
			name:     "Mixed text and non-printable (but mostly text)",
			data:     append([]byte("This is mostly text "), []byte{0x7F, 0x1B}...),
			expected: true,
		},
		{
			name:     "Empty data",
			data:     []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLikelyText(tt.data)
			if result != tt.expected {
				t.Errorf("isLikelyText() = %v, want %v", result, tt.expected)
			}
		})
	}
}
