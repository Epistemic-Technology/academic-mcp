package tools

import (
	"context"
	"errors"
	"os"

	"github.com/Epistemic-Technology/academic-mcp/internal/llm"
	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/operations"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DocumentSummarizeQuery struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
	DocType  string `json:"doc_type,omitempty"`
}

type DocumentSummarizeResponse struct {
	DocumentID    string   `json:"document_id,omitempty"`
	ResourcePaths []string `json:"resource_paths,omitempty"`
	Title         string   `json:"title,omitempty"`
	Summary       string   `json:"summary,omitempty"`
}

func DocumentSummarizeTool() *mcp.Tool {
	inputschema, err := jsonschema.For[DocumentSummarizeQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "document-summarize",
		Description: "Summarize a document (PDF, HTML, Markdown, plain text, or DOCX) using OpenAI's GPT-5 Mini. If the document hasn't been parsed yet, it will automatically parse it first. The document type is automatically detected, but can be overridden with the doc_type parameter.",
		InputSchema: inputschema,
	}
}

func DocumentSummarizeToolHandler(ctx context.Context, req *mcp.CallToolRequest, query DocumentSummarizeQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *DocumentSummarizeResponse, error) {
	log.Info("document-summarize tool called")
	// Use the shared helper to get or parse the document
	docID, parsedItem, err := operations.GetOrParseDocument(ctx, query.ZoteroID, query.URL, query.RawData, query.DocType, store, log)
	if err != nil {
		log.Error("Failed to get or parse document: %v", err)
		return nil, nil, err
	}

	// Calculate resource paths for accessing the document content
	resourcePaths := storage.CalculateResourcePaths(docID, parsedItem)

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Error("OPENAI_API_KEY environment variable not set")
		return nil, nil, errors.New("OPENAI_API_KEY environment variable not set")
	}

	log.Info("Generating summary for document %s", docID)
	summary, err := llm.SummarizeItem(ctx, apiKey, parsedItem, log)
	if err != nil {
		log.Error("Failed to generate summary: %v", err)
		return nil, nil, err
	}

	responseData := &DocumentSummarizeResponse{
		DocumentID:    docID,
		ResourcePaths: resourcePaths,
		Title:         parsedItem.Metadata.Title,
		Summary:       summary,
	}

	return nil, responseData, nil
}
