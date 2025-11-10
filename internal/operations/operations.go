package operations

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Epistemic-Technology/academic-mcp/internal/documents"
	"github.com/Epistemic-Technology/academic-mcp/internal/llm"
	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
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
func GetOrParseDocument(ctx context.Context, zoteroID, url string, rawData []byte, docType string, store storage.Store, log logger.Logger) (string, *models.ParsedItem, error) {
	if zoteroID != "" {
		log.Info("Processing document from Zotero: %s", zoteroID)
	} else if url != "" {
		log.Info("Processing document from URL: %s", url)
	} else {
		log.Info("Processing document from raw data (%d bytes)", len(rawData))
	}
	// Prepare source info
	sourceInfo := &models.SourceInfo{
		ZoteroID: zoteroID,
		URL:      url,
	}

	// Get document data from appropriate source
	var data models.DocumentData
	var externalMetadata *models.ItemMetadata
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
		// No external metadata for raw data
		externalMetadata = nil
	} else {
		// Fetch both data and external metadata (if available)
		data, externalMetadata, err = documents.GetDataWithMetadata(ctx, *sourceInfo)
		if err != nil {
			return "", nil, fmt.Errorf("failed to fetch document data: %w", err)
		}
		// Override detected type if docType parameter is provided
		if docType != "" {
			data.Type = docType
		}

		// Log metadata fetch result
		if externalMetadata != nil {
			log.Info("Retrieved external metadata from %s for document", externalMetadata.MetadataSource)
		} else {
			log.Debug("No external metadata available")
		}
	}

	// Generate document ID
	docID := storage.GenerateDocumentID(sourceInfo, data)

	// Check if document already exists in store
	exists, err := store.DocumentExists(ctx, docID)
	if err != nil {
		log.Error("Failed to check document existence: %v", err)
		return "", nil, fmt.Errorf("failed to check document existence: %w", err)
	}

	var parsedItem *models.ParsedItem

	if exists {
		log.Info("Document %s already exists, retrieving from storage", docID)
		// Document already parsed, retrieve from store
		parsedItem, err = store.GetParsedItem(ctx, docID)
		if err != nil {
			log.Error("Failed to retrieve existing document %s: %v", docID, err)
			return "", nil, fmt.Errorf("failed to retrieve existing document: %w", err)
		}
	} else {
		log.Info("Document %s not found, parsing new document (type: %s)", docID, data.Type)
		// Document needs to be parsed
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			log.Error("OPENAI_API_KEY environment variable not set")
			return "", nil, errors.New("OPENAI_API_KEY environment variable not set")
		}

		// Parse document using type-specific parser (PDF, HTML, Markdown, Text, etc.)
		parsedItem, err = llm.ParseDocument(ctx, apiKey, data, log)
		if err != nil {
			log.Error("Failed to parse document: %v", err)
			return "", nil, fmt.Errorf("failed to parse document: %w", err)
		}

		// Merge external metadata with extracted metadata (if external metadata is available)
		if externalMetadata != nil {
			log.Info("Merging external metadata with extracted metadata")
			parsedItem.Metadata = *documents.MergeMetadata(externalMetadata, &parsedItem.Metadata)
		} else if parsedItem.Metadata.MetadataSource == "" {
			// Mark as extracted if no external metadata
			parsedItem.Metadata.MetadataSource = "extracted"
		}

		// Store the newly parsed document
		err = store.StoreParsedItem(ctx, docID, parsedItem, sourceInfo)
		if err != nil {
			log.Error("Failed to store parsed document: %v", err)
			return "", nil, fmt.Errorf("failed to store parsed item: %w", err)
		}
		log.Info("Successfully parsed and stored document %s", docID)
	}

	return docID, parsedItem, nil
}

// GetOrParsePDF is a convenience wrapper around GetOrParseDocument for PDF-specific use cases.
// Deprecated: Use GetOrParseDocument instead for better multi-format support.
func GetOrParsePDF(ctx context.Context, zoteroID, url string, rawData []byte, store storage.Store, log logger.Logger) (string, *models.ParsedItem, error) {
	return GetOrParseDocument(ctx, zoteroID, url, rawData, "pdf", store, log)
}
