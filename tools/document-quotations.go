package tools

import (
	"context"
	"errors"
	"os"

	"github.com/Epistemic-Technology/academic-mcp/internal/llm"
	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/operations"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/Epistemic-Technology/academic-mcp/models"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DocumentQuotationsQuery struct {
	ZoteroID      string `json:"zotero_id,omitempty"`
	URL           string `json:"url,omitempty"`
	RawData       []byte `json:"raw_data,omitempty"`
	DocType       string `json:"doc_type,omitempty"`
	MaxQuotations *int   `json:"max_quotations,omitempty"` // Default: 10, 0 = unlimited, nil = use default
}

type DocumentQuotationsResponse struct {
	DocumentID     string             `json:"document_id,omitempty"`
	ResourcePaths  []string           `json:"resource_paths,omitempty"`
	Title          string             `json:"title,omitempty"`
	Quotations     []models.Quotation `json:"quotations,omitempty"`
	QuotationCount int                `json:"quotation_count"`
}

func DocumentQuotationsTool() *mcp.Tool {
	inputschema, err := jsonschema.For[DocumentQuotationsQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "document-quotations",
		Description: "Extract representative quotations from a document (PDF, HTML, Markdown, plain text, or DOCX). The document is parsed and summarized first, then an LLM identifies significant quotations with page numbers (for paginated documents). The document type is automatically detected, but can be overridden with the doc_type parameter. Use max_quotations to limit results (default: 10, 0 = unlimited). If more quotations are found than the max, a second LLM pass prioritizes the most significant ones.",
		InputSchema: inputschema,
	}
}

func DocumentQuotationsToolHandler(ctx context.Context, req *mcp.CallToolRequest, query DocumentQuotationsQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *DocumentQuotationsResponse, error) {
	log.Info("document-quotations tool called")

	// Set default max quotations if not specified
	// nil = use default (10), 0 = unlimited, positive = specific limit
	maxQuotations := 10 // default
	if query.MaxQuotations != nil {
		maxQuotations = *query.MaxQuotations
		if maxQuotations < 0 {
			maxQuotations = 10 // Negative values default to 10
		}
	}

	// Use the shared helper to get or parse the document
	docID, parsedItem, err := operations.GetOrParseDocument(ctx, query.ZoteroID, query.URL, query.RawData, query.DocType, store, log)
	if err != nil {
		log.Error("Failed to get or parse document: %v", err)
		return nil, nil, err
	}

	// Check if quotations already exist for this document
	if len(parsedItem.Quotations) > 0 {
		log.Info("Document %s already has %d quotations, returning existing quotations", docID, len(parsedItem.Quotations))
		resourcePaths := storage.CalculateResourcePaths(docID, parsedItem)
		responseData := &DocumentQuotationsResponse{
			DocumentID:     docID,
			ResourcePaths:  resourcePaths,
			Title:          parsedItem.Metadata.Title,
			Quotations:     parsedItem.Quotations,
			QuotationCount: len(parsedItem.Quotations),
		}
		return nil, responseData, nil
	}

	// Get OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Error("OPENAI_API_KEY environment variable not set")
		return nil, nil, errors.New("OPENAI_API_KEY environment variable not set")
	}

	// Generate summary first (needed for quotation extraction context)
	log.Info("Generating summary for document %s", docID)
	summary, err := llm.SummarizeItem(ctx, apiKey, parsedItem, log)
	if err != nil {
		log.Error("Failed to generate summary: %v", err)
		return nil, nil, err
	}

	// Extract quotations using the summary as context
	log.Info("Extracting quotations for document %s (max: %d)", docID, maxQuotations)
	quotations, err := llm.ExtractQuotations(ctx, apiKey, parsedItem, summary, maxQuotations, log)
	if err != nil {
		log.Error("Failed to extract quotations: %v", err)
		return nil, nil, err
	}

	// Update the parsed item with quotations
	parsedItem.Quotations = quotations

	// Store the updated parsed item (with quotations) back to the database
	sourceInfo := &models.SourceInfo{
		ZoteroID: query.ZoteroID,
		URL:      query.URL,
	}
	err = store.StoreParsedItem(ctx, docID, parsedItem, sourceInfo)
	if err != nil {
		log.Error("Failed to store quotations: %v", err)
		return nil, nil, err
	}

	log.Info("Successfully extracted and stored %d quotations for document %s", len(quotations), docID)

	// Calculate resource paths for accessing the document content
	resourcePaths := storage.CalculateResourcePaths(docID, parsedItem)

	responseData := &DocumentQuotationsResponse{
		DocumentID:     docID,
		ResourcePaths:  resourcePaths,
		Title:          parsedItem.Metadata.Title,
		Quotations:     quotations,
		QuotationCount: len(quotations),
	}

	return nil, responseData, nil
}
