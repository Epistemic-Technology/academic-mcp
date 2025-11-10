package tools

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/operations"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
)

type DocumentParseQuery struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
	DocType  string `json:"doc_type,omitempty"`
}

type DocumentParseResponse struct {
	DocumentID    string   `json:"document_id"`
	ResourcePaths []string `json:"resource_paths"`
	Title         string   `json:"title,omitempty"`
	Citekey       string   `json:"citekey,omitempty"`
	PageCount     int      `json:"page_count"`
	RefCount      int      `json:"reference_count"`
	ImageCount    int      `json:"image_count"`
	TableCount    int      `json:"table_count"`
}

func DocumentParseTool() *mcp.Tool {
	inputschema, err := jsonschema.For[DocumentParseQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "document-parse",
		Description: "Parse a document (PDF, HTML, Markdown, plain text, or DOCX) using OpenAI's vision capabilities to extract structured data including metadata, content, references, images, and tables. The document type is automatically detected, but can be overridden with the doc_type parameter.",
		InputSchema: inputschema,
	}
}

func DocumentParseToolHandler(ctx context.Context, req *mcp.CallToolRequest, query DocumentParseQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *DocumentParseResponse, error) {
	log.Info("document-parse tool called")
	// Use the shared helper to get or parse the document
	docID, parsedItem, err := operations.GetOrParseDocument(ctx, query.ZoteroID, query.URL, query.RawData, query.DocType, store, log)
	if err != nil {
		log.Error("document-parse tool failed: %v", err)
		return nil, nil, err
	}

	// Calculate resource paths for accessing the document content
	resourcePaths := storage.CalculateResourcePaths(docID, parsedItem)

	// Format the response with document metadata and statistics
	responseData := &DocumentParseResponse{
		DocumentID:    docID,
		ResourcePaths: resourcePaths,
		Title:         parsedItem.Metadata.Title,
		Citekey:       parsedItem.Metadata.Citekey,
		PageCount:     len(parsedItem.Pages),
		RefCount:      len(parsedItem.References),
		ImageCount:    len(parsedItem.Images),
		TableCount:    len(parsedItem.Tables),
	}

	return nil, responseData, nil
}
