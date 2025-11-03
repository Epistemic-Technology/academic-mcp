package tools

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Epistemic-Technology/academic-mcp/internal/operations"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
)

type PDFParseQuery struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
}

type PDFParseResponse struct {
	DocumentID    string   `json:"document_id"`
	ResourcePaths []string `json:"resource_paths"`
	Title         string   `json:"title,omitempty"`
	PageCount     int      `json:"page_count"`
	RefCount      int      `json:"reference_count"`
	ImageCount    int      `json:"image_count"`
	TableCount    int      `json:"table_count"`
}

func PDFParseTool() *mcp.Tool {
	inputschema, err := jsonschema.For[PDFParseQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "pdf-parse",
		Description: "Parse a PDF file using OpenAI's vision capabilities to extract structured data including metadata, content, references, images, and tables",
		InputSchema: inputschema,
	}
}

func PDFParseToolHandler(ctx context.Context, req *mcp.CallToolRequest, query PDFParseQuery, store storage.Store) (*mcp.CallToolResult, *PDFParseResponse, error) {
	// Use the shared helper to get or parse the PDF document
	docID, parsedItem, err := operations.GetOrParsePDF(ctx, query.ZoteroID, query.URL, query.RawData, store)
	if err != nil {
		return nil, nil, err
	}

	// Calculate resource paths for accessing the document content
	resourcePaths := storage.CalculateResourcePaths(docID, parsedItem)

	// Format the response with document metadata and statistics
	responseData := &PDFParseResponse{
		DocumentID:    docID,
		ResourcePaths: resourcePaths,
		Title:         parsedItem.Metadata.Title,
		PageCount:     len(parsedItem.Pages),
		RefCount:      len(parsedItem.References),
		ImageCount:    len(parsedItem.Images),
		TableCount:    len(parsedItem.Tables),
	}

	return nil, responseData, nil
}
