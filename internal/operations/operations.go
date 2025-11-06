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

// GetOrParsePDF retrieves a parsed PDF document from storage if it exists,
// or fetches and parses it if it doesn't. This function encapsulates the
// common logic shared by tools that need parsed PDF documents.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - zoteroID: Optional Zotero item ID (mutually exclusive with URL and rawData)
//   - url: Optional URL to fetch PDF from (mutually exclusive with zoteroID and rawData)
//   - rawData: Optional raw PDF bytes (mutually exclusive with zoteroID and URL)
//   - store: Storage backend for checking existence and retrieving/storing documents
//
// Returns:
//   - documentID: The generated document ID
//   - parsedItem: The parsed document with all extracted data
//   - error: Any error encountered during the process
func GetOrParsePDF(ctx context.Context, zoteroID, url string, rawData []byte, store storage.Store) (string, *models.ParsedItem, error) {
	// Prepare source info
	sourceInfo := &models.SourceInfo{
		ZoteroID: zoteroID,
		URL:      url,
	}

	// Get PDF data from appropriate source
	var data models.DocumentData
	var err error
	if rawData != nil {
		data = models.DocumentData{
			Data: rawData,
			Type: "pdf",
		}
	} else {
		data, err = documents.GetData(ctx, *sourceInfo)
		if err != nil {
			return "", nil, fmt.Errorf("failed to fetch PDF data: %w", err)
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

		parsedItem, err = llm.ParsePDF(ctx, apiKey, data)
		if err != nil {
			return "", nil, fmt.Errorf("failed to parse PDF: %w", err)
		}

		// Store the newly parsed document
		err = store.StoreParsedItem(ctx, docID, parsedItem, sourceInfo)
		if err != nil {
			return "", nil, fmt.Errorf("failed to store parsed item: %w", err)
		}
	}

	return docID, parsedItem, nil
}
