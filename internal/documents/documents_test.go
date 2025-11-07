package documents

import (
	"archive/zip"
	"bytes"
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

// createTestZip creates a ZIP archive with the given files for testing
func createTestZip(files map[string]string) ([]byte, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for filename, content := range files {
		f, err := w.Create(filename)
		if err != nil {
			return nil, err
		}
		_, err = f.Write([]byte(content))
		if err != nil {
			return nil, err
		}
	}

	err := w.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestExtractHTMLFromZip(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		expectError bool
		expected    string
	}{
		{
			name: "ZIP with index.html",
			files: map[string]string{
				"index.html": "<html><body>Main page</body></html>",
				"style.css":  "body { color: red; }",
				"image.png":  "fake image data",
			},
			expectError: false,
			expected:    "<html><body>Main page</body></html>",
		},
		{
			name: "ZIP with other.html but no index.html",
			files: map[string]string{
				"page.html": "<html><body>Other page</body></html>",
				"style.css": "body { color: blue; }",
			},
			expectError: false,
			expected:    "<html><body>Other page</body></html>",
		},
		{
			name: "ZIP with multiple HTML files (prefers index.html)",
			files: map[string]string{
				"index.html": "<html><body>Index page</body></html>",
				"other.html": "<html><body>Other page</body></html>",
			},
			expectError: false,
			expected:    "<html><body>Index page</body></html>",
		},
		{
			name: "ZIP with no HTML files",
			files: map[string]string{
				"data.json": `{"key": "value"}`,
				"style.css": "body { color: green; }",
			},
			expectError: true,
		},
		{
			name: "ZIP with .htm extension",
			files: map[string]string{
				"page.htm": "<html><body>HTM page</body></html>",
			},
			expectError: false,
			expected:    "<html><body>HTM page</body></html>",
		},
		{
			name: "ZIP with subdirectory",
			files: map[string]string{
				"subdir/index.html": "<html><body>Nested page</body></html>",
			},
			expectError: false,
			expected:    "<html><body>Nested page</body></html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zipData, err := createTestZip(tt.files)
			if err != nil {
				t.Fatalf("Failed to create test ZIP: %v", err)
			}

			result, err := ExtractHTMLFromZip(zipData)

			if tt.expectError {
				if err == nil {
					t.Errorf("ExtractHTMLFromZip() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("ExtractHTMLFromZip() unexpected error: %v", err)
				}
				if string(result) != tt.expected {
					t.Errorf("ExtractHTMLFromZip() = %q, want %q", string(result), tt.expected)
				}
			}
		})
	}
}

func TestIsZoteroSnapshotZip(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{
			name: "ZIP with HTML file",
			files: map[string]string{
				"index.html": "<html><body>Test</body></html>",
				"style.css":  "body { }",
			},
			expected: true,
		},
		{
			name: "ZIP with .htm file",
			files: map[string]string{
				"page.htm": "<html><body>Test</body></html>",
			},
			expected: true,
		},
		{
			name: "ZIP without HTML files",
			files: map[string]string{
				"data.json": `{"key": "value"}`,
				"image.png": "fake image",
			},
			expected: false,
		},
		{
			name: "ZIP with HTML in subdirectory",
			files: map[string]string{
				"assets/page.html": "<html><body>Test</body></html>",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zipData, err := createTestZip(tt.files)
			if err != nil {
				t.Fatalf("Failed to create test ZIP: %v", err)
			}

			result := isZoteroSnapshotZip(zipData)
			if result != tt.expected {
				t.Errorf("isZoteroSnapshotZip() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectDocumentType_ZoteroSnapshot(t *testing.T) {
	// Create a test ZIP with HTML
	zipData, err := createTestZip(map[string]string{
		"index.html": "<html><body>Zotero snapshot</body></html>",
		"style.css":  "body { color: black; }",
	})
	if err != nil {
		t.Fatalf("Failed to create test ZIP: %v", err)
	}

	result := DetectDocumentType(zipData)
	if result != "zotero-snapshot" {
		t.Errorf("DetectDocumentType() for Zotero snapshot = %v, want zotero-snapshot", result)
	}
}
