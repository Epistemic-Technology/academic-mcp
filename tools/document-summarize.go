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

type DocumentSummarizeInput struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
	DocType  string `json:"doc_type,omitempty"`
}

type DocumentSummarizeQuery struct {
	// For single document: use these fields directly
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
	DocType  string `json:"doc_type,omitempty"`
	// For multiple documents: use this field
	Documents []DocumentSummarizeInput `json:"documents,omitempty"`
}

type DocumentSummarizeResult struct {
	DocumentID    string   `json:"document_id,omitempty"`
	ResourcePaths []string `json:"resource_paths,omitempty"`
	Title         string   `json:"title,omitempty"`
	Citekey       string   `json:"citekey,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type DocumentSummarizeResponse struct {
	Results []DocumentSummarizeResult `json:"results"`
	Count   int                       `json:"count"`
}

func DocumentSummarizeTool() *mcp.Tool {
	inputschema, err := jsonschema.For[DocumentSummarizeQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "document-summarize",
		Description: "Summarize one or more documents (PDF, HTML, Markdown, plain text, or DOCX) using OpenAI's GPT-5 Mini. If the document hasn't been parsed yet, it will automatically parse it first. The document type is automatically detected, but can be overridden with the doc_type parameter. For multiple documents, use the 'documents' field. Multiple documents are processed concurrently.",
		InputSchema: inputschema,
	}
}

func DocumentSummarizeToolHandler(ctx context.Context, req *mcp.CallToolRequest, query DocumentSummarizeQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *DocumentSummarizeResponse, error) {
	log.Info("document-summarize tool called")

	// Check for OpenAI API key early
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Error("OPENAI_API_KEY environment variable not set")
		return nil, nil, errors.New("OPENAI_API_KEY environment variable not set")
	}

	// Determine if this is a single document or batch request
	var inputs []DocumentSummarizeInput
	if len(query.Documents) > 0 {
		// Batch mode
		inputs = query.Documents
		log.Info("Processing batch of %d documents", len(inputs))
	} else {
		// Single document mode (backward compatible)
		inputs = []DocumentSummarizeInput{{
			ZoteroID: query.ZoteroID,
			URL:      query.URL,
			RawData:  query.RawData,
			DocType:  query.DocType,
		}}
		log.Info("Processing single document")
	}

	// Process documents concurrently
	results := make([]DocumentSummarizeResult, len(inputs))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, input := range inputs {
		wg.Add(1)
		go func(idx int, inp DocumentSummarizeInput) {
			defer wg.Done()

			// Check if context is cancelled before starting
			select {
			case <-ctx.Done():
				mu.Lock()
				results[idx] = DocumentSummarizeResult{
					Error: fmt.Sprintf("cancelled: %v", ctx.Err()),
				}
				mu.Unlock()
				return
			default:
			}

			// Use the shared helper to get or parse the document
			docID, parsedItem, err := operations.GetOrParseDocument(ctx, inp.ZoteroID, inp.URL, inp.RawData, inp.DocType, store, log)
			if err != nil {
				log.Error("Failed to get or parse document %d: %v", idx, err)
				mu.Lock()
				results[idx] = DocumentSummarizeResult{
					Error: fmt.Sprintf("failed to parse: %v", err),
				}
				mu.Unlock()
				return
			}

			// Calculate resource paths for accessing the document content
			resourcePaths := storage.CalculateResourcePaths(docID, parsedItem)

			// Check if summary already exists
			if parsedItem.Summary != "" {
				log.Info("Document %s already has a summary, returning cached summary", docID)
				mu.Lock()
				results[idx] = DocumentSummarizeResult{
					DocumentID:    docID,
					ResourcePaths: resourcePaths,
					Title:         parsedItem.Metadata.Title,
					Citekey:       parsedItem.Metadata.Citekey,
					Summary:       parsedItem.Summary,
				}
				mu.Unlock()
				return
			}

			log.Info("Generating summary for document %s", docID)
			summary, err := llm.SummarizeItem(ctx, apiKey, parsedItem, log)
			if err != nil {
				log.Error("Failed to generate summary for document %s: %v", docID, err)
				mu.Lock()
				results[idx] = DocumentSummarizeResult{
					DocumentID: docID,
					Title:      parsedItem.Metadata.Title,
					Error:      fmt.Sprintf("failed to generate summary: %v", err),
				}
				mu.Unlock()
				return
			}

			// Update the parsed item with the summary
			parsedItem.Summary = summary

			// Store the updated parsed item (with summary) back to the database
			sourceInfo := &models.SourceInfo{
				ZoteroID: inp.ZoteroID,
				URL:      inp.URL,
			}
			err = store.StoreParsedItem(ctx, docID, parsedItem, sourceInfo)
			if err != nil {
				log.Error("Failed to store summary for document %s: %v", docID, err)
				mu.Lock()
				results[idx] = DocumentSummarizeResult{
					DocumentID: docID,
					Title:      parsedItem.Metadata.Title,
					Summary:    summary,
					Error:      fmt.Sprintf("warning: summary generated but not stored: %v", err),
				}
				mu.Unlock()
				return
			}

			log.Info("Successfully generated and stored summary for document %s", docID)

			mu.Lock()
			results[idx] = DocumentSummarizeResult{
				DocumentID:    docID,
				ResourcePaths: resourcePaths,
				Title:         parsedItem.Metadata.Title,
				Citekey:       parsedItem.Metadata.Citekey,
				Summary:       summary,
			}
			mu.Unlock()
		}(i, input)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check if context was cancelled
	if ctx.Err() != nil {
		log.Error("document-summarize tool cancelled: %v", ctx.Err())
		return nil, nil, ctx.Err()
	}

	responseData := &DocumentSummarizeResponse{
		Results: results,
		Count:   len(results),
	}

	log.Info("Successfully processed %d documents", len(results))
	return nil, responseData, nil
}
