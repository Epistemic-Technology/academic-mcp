package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/operations"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
)

type DocumentParseInput struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
	DocType  string `json:"doc_type,omitempty"`
}

type DocumentParseQuery struct {
	// For single document: use these fields directly
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
	DocType  string `json:"doc_type,omitempty"`
	// For multiple documents: use this field
	Documents []DocumentParseInput `json:"documents,omitempty"`
}

type DocumentParseResult struct {
	DocumentID    string   `json:"document_id"`
	ResourcePaths []string `json:"resource_paths"`
	Title         string   `json:"title,omitempty"`
	Citekey       string   `json:"citekey,omitempty"`
	PageCount     int      `json:"page_count"`
	RefCount      int      `json:"reference_count"`
	ImageCount    int      `json:"image_count"`
	TableCount    int      `json:"table_count"`
	Error         string   `json:"error,omitempty"`
}

type DocumentParseResponse struct {
	Results []DocumentParseResult `json:"results"`
	Count   int                   `json:"count"`
}

func DocumentParseTool() *mcp.Tool {
	inputschema, err := jsonschema.For[DocumentParseQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "document-parse",
		Description: "Parse one or more documents (PDF, HTML, Markdown, plain text, or DOCX) using OpenAI's vision capabilities to extract structured data including metadata, content, references, images, and tables. The document type is automatically detected, but can be overridden with the doc_type parameter. For multiple documents, use the 'documents' field. Multiple documents are processed concurrently.",
		InputSchema: inputschema,
	}
}

func DocumentParseToolHandler(ctx context.Context, req *mcp.CallToolRequest, query DocumentParseQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *DocumentParseResponse, error) {
	log.Info("document-parse tool called")

	// Determine if this is a single document or batch request
	var inputs []DocumentParseInput
	if len(query.Documents) > 0 {
		// Batch mode
		inputs = query.Documents
		log.Info("Processing batch of %d documents", len(inputs))
	} else {
		// Single document mode (backward compatible)
		inputs = []DocumentParseInput{{
			ZoteroID: query.ZoteroID,
			URL:      query.URL,
			RawData:  query.RawData,
			DocType:  query.DocType,
		}}
		log.Info("Processing single document")
	}

	// Process documents concurrently
	results := make([]DocumentParseResult, len(inputs))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, input := range inputs {
		wg.Add(1)
		go func(idx int, inp DocumentParseInput) {
			defer wg.Done()

			// Check if context is cancelled before starting
			select {
			case <-ctx.Done():
				mu.Lock()
				results[idx] = DocumentParseResult{
					ResourcePaths: []string{},
					Error:         fmt.Sprintf("cancelled: %v", ctx.Err()),
				}
				mu.Unlock()
				return
			default:
			}

			// Use the shared helper to get or parse the document
			docID, parsedItem, err := operations.GetOrParseDocument(ctx, inp.ZoteroID, inp.URL, inp.RawData, inp.DocType, store, log)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				log.Error("Failed to parse document %d: %v", idx, err)
				results[idx] = DocumentParseResult{
					ResourcePaths: []string{},
					Error:         fmt.Sprintf("failed to parse: %v", err),
				}
				return
			}

			// Calculate resource paths for accessing the document content
			resourcePaths := storage.CalculateResourcePaths(docID, parsedItem)

			// Format the result with document metadata and statistics
			results[idx] = DocumentParseResult{
				DocumentID:    docID,
				ResourcePaths: resourcePaths,
				Title:         parsedItem.Metadata.Title,
				Citekey:       parsedItem.Metadata.Citekey,
				PageCount:     len(parsedItem.Pages),
				RefCount:      len(parsedItem.References),
				ImageCount:    len(parsedItem.Images),
				TableCount:    len(parsedItem.Tables),
			}
		}(i, input)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check if context was cancelled
	if ctx.Err() != nil {
		log.Error("document-parse tool cancelled: %v", ctx.Err())
		return nil, nil, ctx.Err()
	}

	responseData := &DocumentParseResponse{
		Results: results,
		Count:   len(results),
	}

	log.Info("Successfully processed %d documents", len(results))
	return nil, responseData, nil
}
