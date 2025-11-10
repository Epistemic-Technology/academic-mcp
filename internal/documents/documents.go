package documents

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Epistemic-Technology/academic-mcp/models"
	"github.com/Epistemic-Technology/zotero/zotero"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
)

// DetectDocumentType determines the type of document from the raw data
// by checking magic bytes/headers
func DetectDocumentType(data []byte) string {
	if len(data) == 0 {
		return "unknown"
	}

	// For very short data, check if it's text
	if len(data) < 4 {
		if isLikelyText(data) {
			return "txt"
		}
		return "unknown"
	}

	// PDF: starts with %PDF
	if bytes.HasPrefix(data, []byte("%PDF")) {
		return "pdf"
	}

	// HTML: check for common HTML markers
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("<!DOCTYPE html")) ||
		bytes.HasPrefix(trimmed, []byte("<!doctype html")) ||
		bytes.HasPrefix(trimmed, []byte("<html")) ||
		bytes.HasPrefix(trimmed, []byte("<HTML")) {
		return "html"
	}

	// ZIP-based formats: ZIP file starting with PK (0x504B)
	if len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B &&
		(data[2] == 0x03 || data[2] == 0x05 || data[2] == 0x07) {
		// Check if it contains word/ directory (DOCX)
		if bytes.Contains(data[:min(len(data), 1024)], []byte("word/")) {
			return "docx"
		}
		// Check if it's a Zotero web snapshot (ZIP containing HTML)
		if isZoteroSnapshotZip(data) {
			return "zotero-snapshot"
		}
		return "zip"
	}

	// Plain text / Markdown (if it's valid UTF-8 and has no binary characters)
	if isLikelyText(data) {
		// Simple markdown detection: look for common markdown patterns
		if bytes.Contains(data[:min(len(data), 1024)], []byte("# ")) ||
			bytes.Contains(data[:min(len(data), 1024)], []byte("## ")) ||
			bytes.Contains(data[:min(len(data), 1024)], []byte("```")) {
			return "md"
		}
		return "txt"
	}

	return "unknown"
}

// isLikelyText checks if the data is likely plain text (no binary content)
func isLikelyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	sampleSize := min(len(data), 512)
	sample := data[:sampleSize]

	// Check for null bytes (strong indicator of binary content)
	if bytes.Contains(sample, []byte{0}) {
		return false
	}

	// Count printable vs non-printable characters
	printable := 0
	for _, b := range sample {
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			printable++
		}
	}

	// If more than 90% is printable, likely text
	return float64(printable)/float64(len(sample)) > 0.9
}

// GetData retrieves document data from a source and detects its type
func GetData(ctx context.Context, sourceInfo models.SourceInfo) (models.DocumentData, error) {
	docData, _, err := GetDataWithMetadata(ctx, sourceInfo)
	return docData, err
}

// GetDataWithMetadata retrieves document data from a source and detects its type,
// also returning external metadata if available (e.g., from Zotero).
// Returns the document data and external metadata (nil if not available).
func GetDataWithMetadata(ctx context.Context, sourceInfo models.SourceInfo) (models.DocumentData, *models.ItemMetadata, error) {
	var data []byte
	var err error
	var externalMetadata *models.ItemMetadata

	if sourceInfo.ZoteroID != "" {
		zoteroAPIKey := os.Getenv("ZOTERO_API_KEY")
		libraryID := os.Getenv("ZOTERO_LIBRARY_ID")

		// Fetch document data
		data, err = GetFromZotero(ctx, sourceInfo.ZoteroID, zoteroAPIKey, libraryID)
		if err != nil {
			return models.DocumentData{}, nil, err
		}

		// Fetch external metadata from Zotero
		externalMetadata, err = FetchZoteroMetadata(ctx, sourceInfo.ZoteroID, zoteroAPIKey, libraryID)
		if err != nil {
			// Log error but don't fail - we can still parse without external metadata
			// The error will be logged by the caller
			externalMetadata = nil
		}
	} else if sourceInfo.URL != "" {
		data, err = GetFromURL(ctx, sourceInfo.URL)
		if err != nil {
			return models.DocumentData{}, nil, err
		}
	} else {
		return models.DocumentData{}, nil, errors.New("no data provided")
	}

	if data == nil {
		return models.DocumentData{}, nil, errors.New("no data retrieved")
	}

	// Detect document type from content
	docType := DetectDocumentType(data)

	// If it's a Zotero web snapshot ZIP, extract the HTML automatically
	if docType == "zotero-snapshot" {
		htmlData, err := ExtractHTMLFromZip(data)
		if err != nil {
			return models.DocumentData{}, nil, fmt.Errorf("failed to extract HTML from Zotero snapshot: %w", err)
		}
		// Return the extracted HTML with type "html"
		return models.DocumentData{
			Data: htmlData,
			Type: "html",
		}, externalMetadata, nil
	}

	return models.DocumentData{
		Data: data,
		Type: docType,
	}, externalMetadata, nil
}

// GetFromURL fetches document data from a URL
func GetFromURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// GetFromZotero fetches document data from a Zotero library
func GetFromZotero(ctx context.Context, zoteroID string, apiKey string, libraryID string) ([]byte, error) {
	client := zotero.NewClient(libraryID, zotero.LibraryTypeUser, zotero.WithAPIKey(apiKey))
	data, err := client.File(ctx, zoteroID)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ExtractHTMLFromZip attempts to extract HTML content from a ZIP archive
// (typically used for Zotero web page snapshots). It looks for the main HTML file
// and returns its contents. Returns error if no HTML file is found.
func ExtractHTMLFromZip(data []byte) ([]byte, error) {
	// Create a reader from the byte slice
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open ZIP archive: %w", err)
	}

	// Look for HTML files in the archive
	// Priority: index.html, *.html (first found)
	var htmlFile *zip.File
	var indexFile *zip.File

	for _, file := range reader.File {
		// Skip directories
		if file.FileInfo().IsDir() {
			continue
		}

		fileName := filepath.Base(file.Name)
		lowerName := strings.ToLower(fileName)

		// Check if it's an HTML file
		if strings.HasSuffix(lowerName, ".html") || strings.HasSuffix(lowerName, ".htm") {
			// Prefer index.html if found
			if lowerName == "index.html" || lowerName == "index.htm" {
				indexFile = file
				break
			}
			// Keep first HTML file as fallback
			if htmlFile == nil {
				htmlFile = file
			}
		}
	}

	// Use index.html if found, otherwise use first HTML file
	targetFile := indexFile
	if targetFile == nil {
		targetFile = htmlFile
	}

	if targetFile == nil {
		return nil, errors.New("no HTML file found in ZIP archive")
	}

	// Extract the HTML content
	rc, err := targetFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open HTML file in ZIP: %w", err)
	}
	defer rc.Close()

	htmlData, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTML file from ZIP: %w", err)
	}

	return htmlData, nil
}

// isZoteroSnapshotZip checks if a ZIP archive appears to be a Zotero web snapshot
// by looking for HTML files inside
func isZoteroSnapshotZip(data []byte) bool {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}

	// Look for any HTML files
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		lowerName := strings.ToLower(filepath.Base(file.Name))
		if strings.HasSuffix(lowerName, ".html") || strings.HasSuffix(lowerName, ".htm") {
			return true
		}
	}

	return false
}

// PreprocessHTML converts HTML to markdown to reduce context window usage.
// This strips unnecessary markup, scripts, styling, and images while preserving
// document structure (headings, lists, tables, links).
func PreprocessHTML(htmlData []byte) (string, error) {
	// Create converter with base and commonmark plugins
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)

	// Remove images to avoid embedding large base64 SVG/image data
	conv.Register.TagType("img", converter.TagTypeRemove, converter.PriorityStandard)

	// Convert HTML bytes to markdown string
	markdown, err := conv.ConvertReader(bytes.NewReader(htmlData))
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to markdown: %w", err)
	}

	return string(markdown), nil
}
