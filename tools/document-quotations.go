package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/Epistemic-Technology/academic-mcp/internal/llm"
	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/operations"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/Epistemic-Technology/academic-mcp/models"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DocumentQuotationsInput struct {
	ZoteroID      string `json:"zotero_id,omitempty"`
	URL           string `json:"url,omitempty"`
	RawData       []byte `json:"raw_data,omitempty"`
	DocType       string `json:"doc_type,omitempty"`
	MaxQuotations *int   `json:"max_quotations,omitempty"` // Default: 10, 0 = unlimited, nil = use default
}

type DocumentQuotationsQuery struct {
	// For single document: use these fields directly
	ZoteroID      string `json:"zotero_id,omitempty"`
	URL           string `json:"url,omitempty"`
	RawData       []byte `json:"raw_data,omitempty"`
	DocType       string `json:"doc_type,omitempty"`
	MaxQuotations *int   `json:"max_quotations,omitempty"` // Default: 10, 0 = unlimited, nil = use default
	// For multiple documents: use this field
	Documents []DocumentQuotationsInput `json:"documents,omitempty"`
}

type DocumentQuotationsResult struct {
	DocumentID     string             `json:"document_id,omitempty"`
	ResourcePaths  []string           `json:"resource_paths,omitempty"`
	Title          string             `json:"title,omitempty"`
	Citekey        string             `json:"citekey,omitempty"`
	Quotations     []models.Quotation `json:"quotations,omitempty"`
	QuotationCount int                `json:"quotation_count"`
	Error          string             `json:"error,omitempty"`
}

type DocumentQuotationsResponse struct {
	Results []DocumentQuotationsResult `json:"results"`
	Count   int                        `json:"count"`
}

func DocumentQuotationsTool() *mcp.Tool {
	inputschema, err := jsonschema.For[DocumentQuotationsQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "document-quotations",
		Description: "Extract representative quotations from one or more documents (PDF, HTML, Markdown, plain text, or DOCX). The document is parsed and summarized first, then an LLM identifies significant quotations with page numbers (for paginated documents). The document type is automatically detected, but can be overridden with the doc_type parameter. Use max_quotations to limit results (default: 10, 0 = unlimited). If more quotations are found than the max, a second LLM pass prioritizes the most significant ones. For multiple documents, use the 'documents' field. Multiple documents are processed concurrently.",
		InputSchema: inputschema,
	}
}

func DocumentQuotationsToolHandler(ctx context.Context, req *mcp.CallToolRequest, query DocumentQuotationsQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *DocumentQuotationsResponse, error) {
	log.Info("document-quotations tool called")

	// Check for OpenAI API key early
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Error("OPENAI_API_KEY environment variable not set")
		return nil, nil, errors.New("OPENAI_API_KEY environment variable not set")
	}

	// Determine if this is a single document or batch request
	var inputs []DocumentQuotationsInput
	if len(query.Documents) > 0 {
		// Batch mode
		inputs = query.Documents
		log.Info("Processing batch of %d documents", len(inputs))
	} else {
		// Single document mode (backward compatible)
		inputs = []DocumentQuotationsInput{{
			ZoteroID:      query.ZoteroID,
			URL:           query.URL,
			RawData:       query.RawData,
			DocType:       query.DocType,
			MaxQuotations: query.MaxQuotations,
		}}
		log.Info("Processing single document")
	}

	// Process documents concurrently
	results := make([]DocumentQuotationsResult, len(inputs))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, input := range inputs {
		wg.Add(1)
		go func(idx int, inp DocumentQuotationsInput) {
			defer wg.Done()

			// Check if context is cancelled before starting
			select {
			case <-ctx.Done():
				mu.Lock()
				results[idx] = DocumentQuotationsResult{
					Error: fmt.Sprintf("cancelled: %v", ctx.Err()),
				}
				mu.Unlock()
				return
			default:
			}

			// Set default max quotations if not specified
			maxQuotations := 10 // default
			if inp.MaxQuotations != nil {
				maxQuotations = *inp.MaxQuotations
				if maxQuotations < 0 {
					maxQuotations = 10 // Negative values default to 10
				}
			}

			// Use the shared helper to get or parse the document
			docID, parsedItem, err := operations.GetOrParseDocument(ctx, inp.ZoteroID, inp.URL, inp.RawData, inp.DocType, store, log)
			if err != nil {
				log.Error("Failed to get or parse document %d: %v", idx, err)
				mu.Lock()
				results[idx] = DocumentQuotationsResult{
					Error: fmt.Sprintf("failed to parse: %v", err),
				}
				mu.Unlock()
				return
			}

			// Calculate resource paths for accessing the document content
			resourcePaths := storage.CalculateResourcePaths(docID, parsedItem)

			// Check if quotations already exist for this document
			if len(parsedItem.Quotations) > 0 {
				log.Info("Document %s already has %d quotations, returning existing quotations", docID, len(parsedItem.Quotations))
				mu.Lock()
				results[idx] = DocumentQuotationsResult{
					DocumentID:     docID,
					ResourcePaths:  resourcePaths,
					Title:          parsedItem.Metadata.Title,
					Citekey:        parsedItem.Metadata.Citekey,
					Quotations:     parsedItem.Quotations,
					QuotationCount: len(parsedItem.Quotations),
				}
				mu.Unlock()
				return
			}

			// Generate summary first (needed for quotation extraction context)
			log.Info("Generating summary for document %s", docID)
			summary, err := llm.SummarizeItem(ctx, apiKey, parsedItem, log)
			if err != nil {
				log.Error("Failed to generate summary for document %s: %v", docID, err)
				mu.Lock()
				results[idx] = DocumentQuotationsResult{
					DocumentID: docID,
					Title:      parsedItem.Metadata.Title,
					Error:      fmt.Sprintf("failed to generate summary: %v", err),
				}
				mu.Unlock()
				return
			}

			// Extract quotations using the summary as context
			log.Info("Extracting quotations for document %s (max: %d)", docID, maxQuotations)
			quotations, err := llm.ExtractQuotations(ctx, apiKey, parsedItem, summary, maxQuotations, log)
			if err != nil {
				log.Error("Failed to extract quotations for document %s: %v", docID, err)
				mu.Lock()
				results[idx] = DocumentQuotationsResult{
					DocumentID: docID,
					Title:      parsedItem.Metadata.Title,
					Error:      fmt.Sprintf("failed to extract quotations: %v", err),
				}
				mu.Unlock()
				return
			}

			// Update the parsed item with quotations
			parsedItem.Quotations = quotations

			// Store the updated parsed item (with quotations) back to the database
			sourceInfo := &models.SourceInfo{
				ZoteroID: inp.ZoteroID,
				URL:      inp.URL,
			}
			err = store.StoreParsedItem(ctx, docID, parsedItem, sourceInfo)
			if err != nil {
				log.Error("Failed to store quotations for document %s: %v", docID, err)
				mu.Lock()
				results[idx] = DocumentQuotationsResult{
					DocumentID:     docID,
					Title:          parsedItem.Metadata.Title,
					Quotations:     quotations,
					QuotationCount: len(quotations),
					Error:          fmt.Sprintf("warning: quotations extracted but not stored: %v", err),
				}
				mu.Unlock()
				return
			}

			log.Info("Successfully extracted and stored %d quotations for document %s", len(quotations), docID)

			mu.Lock()
			results[idx] = DocumentQuotationsResult{
				DocumentID:     docID,
				ResourcePaths:  resourcePaths,
				Title:          parsedItem.Metadata.Title,
				Citekey:        parsedItem.Metadata.Citekey,
				Quotations:     quotations,
				QuotationCount: len(quotations),
			}
			mu.Unlock()
		}(i, input)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check if context was cancelled
	if ctx.Err() != nil {
		log.Error("document-quotations tool cancelled: %v", ctx.Err())
		return nil, nil, ctx.Err()
	}

	responseData := &DocumentQuotationsResponse{
		Results: results,
		Count:   len(results),
	}

	log.Info("Successfully processed %d documents", len(results))
	return nil, responseData, nil
}
