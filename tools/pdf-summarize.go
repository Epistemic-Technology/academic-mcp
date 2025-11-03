package tools

import (
	"context"
	"errors"
	"os"

	"github.com/Epistemic-Technology/academic-mcp/internal/llm"
	"github.com/Epistemic-Technology/academic-mcp/internal/operations"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PDFSummarizeQuery struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
}

type PDFSummarizeResponse struct {
	DocumentID    string   `json:"document_id,omitempty"`
	ResourcePaths []string `json:"resource_paths,omitempty"`
	Title         string   `json:"title,omitempty"`
	Summary       string   `json:"summary,omitempty"`
}

func PDFSummarizeTool() *mcp.Tool {
	inputschema, err := jsonschema.For[PDFSummarizeQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "academic-mcp.pdf-summarize",
		Description: "Summarize a PDF document",
		InputSchema: inputschema,
	}
}

func PDFSummarizeToolHandler(ctx context.Context, req *mcp.CallToolRequest, query PDFSummarizeQuery, store storage.Store) (*mcp.CallToolResult, *PDFSummarizeResponse, error) {
	// Use the shared helper to get or parse the PDF document
	docID, parsedItem, err := operations.GetOrParsePDF(ctx, query.ZoteroID, query.URL, query.RawData, store)
	if err != nil {
		return nil, nil, err
	}

	// Calculate resource paths for accessing the document content
	resourcePaths := storage.CalculateResourcePaths(docID, parsedItem)

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, nil, errors.New("OPENAI_API_KEY environment variable not set")
	}

	summary, err := llm.SummarizeItem(ctx, apiKey, parsedItem)
	if err != nil {
		return nil, nil, err
	}

	responseData := &PDFSummarizeResponse{
		DocumentID:    docID,
		ResourcePaths: resourcePaths,
		Title:         parsedItem.Metadata.Title,
		Summary:       summary,
	}

	return nil, responseData, nil
}
