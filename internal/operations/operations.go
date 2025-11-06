package operations

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Epistemic-Technology/academic-mcp/internal/documents"
	"github.com/Epistemic-Technology/academic-mcp/internal/llm"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/Epistemic-Technology/academic-mcp/models"
)

// GetOrParseDocument retrieves a parsed document from storage if it exists,
// or fetches and parses it if it doesn't. This function encapsulates the
// common logic shared by tools that need parsed documents.
//
// Supports multiple document types: PDF, HTML, Markdown, plain text, and DOCX.
// The document type is automatically detected from the content.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - zoteroID: Optional Zotero item ID (mutually exclusive with URL and rawData)
//   - url: Optional URL to fetch document from (mutually exclusive with zoteroID and rawData)
//   - rawData: Optional raw document bytes (mutually exclusive with zoteroID and URL)
//   - docType: Optional document type override (e.g., "pdf", "html", "md", "txt"). If empty, type will be auto-detected.
//   - store: Storage backend for checking existence and retrieving/storing documents
//
// Returns:
//   - documentID: The generated document ID
//   - parsedItem: The parsed document with all extracted data
//   - error: Any error encountered during the process
func GetOrParseDocument(ctx context.Context, zoteroID, url string, rawData []byte, docType string, store storage.Store) (string, *models.ParsedItem, error) {
	// Prepare source info
	sourceInfo := &models.SourceInfo{
		ZoteroID: zoteroID,
		URL:      url,
	}

	// Get document data from appropriate source
	var data models.DocumentData
	var err error
	if rawData != nil {
		// If docType is provided, use it; otherwise auto-detect
		detectedType := docType
		if detectedType == "" {
			detectedType = documents.DetectDocumentType(rawData)
		}
		data = models.DocumentData{
			Data: rawData,
			Type: detectedType,
		}
	} else {
		data, err = documents.GetData(ctx, *sourceInfo)
		if err != nil {
			return "", nil, fmt.Errorf("failed to fetch document data: %w", err)
		}
		// Override detected type if docType parameter is provided
		if docType != "" {
			data.Type = docType
		}
	}

	// Generate document ID
	docID := storage.GenerateDocumentID(sourceInfo, data)

	// Check if document already exists in store
	exists, err := store.DocumentExists(ctx, docID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to check document existence: %w", err)
	}

	var parsedItem *models.ParsedItem

	if exists {
		// Document already parsed, retrieve from store
		parsedItem, err = store.GetParsedItem(ctx, docID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to retrieve existing document: %w", err)
		}
	} else {
		// Document needs to be parsed
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return "", nil, errors.New("OPENAI_API_KEY environment variable not set")
		}

		// Parse document using type-specific parser (PDF, HTML, Markdown, Text, etc.)
		parsedItem, err = llm.ParseDocument(ctx, apiKey, data)
		if err != nil {
			return "", nil, fmt.Errorf("failed to parse document: %w", err)
		}

		// Store the newly parsed document
		err = store.StoreParsedItem(ctx, docID, parsedItem, sourceInfo)
		if err != nil {
			return "", nil, fmt.Errorf("failed to store parsed item: %w", err)
		}
	}

	return docID, parsedItem, nil
}

// GetOrParsePDF is a convenience wrapper around GetOrParseDocument for PDF-specific use cases.
// Deprecated: Use GetOrParseDocument instead for better multi-format support.
func GetOrParsePDF(ctx context.Context, zoteroID, url string, rawData []byte, store storage.Store) (string, *models.ParsedItem, error) {
	return GetOrParseDocument(ctx, zoteroID, url, rawData, "pdf", store)
}
