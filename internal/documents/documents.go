package documents

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/Epistemic-Technology/academic-mcp/models"
	"github.com/Epistemic-Technology/zotero/zotero"
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

	// DOCX: ZIP file starting with PK (0x504B) containing specific XML files
	if len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B &&
		(data[2] == 0x03 || data[2] == 0x05 || data[2] == 0x07) {
		// Check if it contains word/ directory (simple heuristic)
		if bytes.Contains(data[:min(len(data), 1024)], []byte("word/")) {
			return "docx"
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
	var data []byte
	var err error

	if sourceInfo.ZoteroID != "" {
		zoteroAPIKey := os.Getenv("ZOTERO_API_KEY")
		libraryID := os.Getenv("ZOTERO_LIBRARY_ID")
		data, err = GetFromZotero(ctx, sourceInfo.ZoteroID, zoteroAPIKey, libraryID)
		if err != nil {
			return models.DocumentData{}, err
		}
	} else if sourceInfo.URL != "" {
		data, err = GetFromURL(ctx, sourceInfo.URL)
		if err != nil {
			return models.DocumentData{}, err
		}
	} else {
		return models.DocumentData{}, errors.New("no data provided")
	}

	if data == nil {
		return models.DocumentData{}, errors.New("no data retrieved")
	}

	// Detect document type from content
	docType := DetectDocumentType(data)

	return models.DocumentData{
		Data: data,
		Type: docType,
	}, nil
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
