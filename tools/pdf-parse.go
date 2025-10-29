package tools

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Epistemic-Technology/academic-mcp/internal/llm"
	"github.com/Epistemic-Technology/academic-mcp/internal/pdf"
	"github.com/Epistemic-Technology/academic-mcp/models"
)

type PDFParseQuery struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
}

type PDFParseResponse struct {
	ParsedItem   *models.ParsedItem `json:"parsed_item"`
	ResourcePath string             `json:"resource_path,omitempty"`
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

func PDFParseToolHandler(ctx context.Context, req *mcp.CallToolRequest, query PDFParseQuery) (*mcp.CallToolResult, *PDFParseResponse, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, nil, errors.New("OPENAI_API_KEY environment variable not set")
	}

	var data models.PdfData
	var err error

	if query.RawData != nil {
		data = query.RawData
	} else if query.ZoteroID != "" {
		zoteroAPIKey := os.Getenv("ZOTERO_API_KEY")
		libraryID := os.Getenv("ZOTERO_LIBRARY_ID")
		data, err = pdf.GetFromZotero(ctx, query.ZoteroID, zoteroAPIKey, libraryID)
		if err != nil {
			return nil, nil, err
		}
	} else if query.URL != "" {
		data, err = pdf.GetFromURL(ctx, query.URL)
		if err != nil {
			return nil, nil, err
		}
	} else {
		return nil, nil, errors.New("no data provided")
	}

	if data == nil {
		return nil, nil, errors.New("no data retrieved")
	}

	parsedItem, err := llm.ParsePDF(ctx, apiKey, data)
	if err != nil {
		return nil, nil, err
	}

	// Create response message
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully parsed PDF document. Extracted metadata, %d pages, %d references, %d images, and %d tables.",
					len(parsedItem.Pages),
					len(parsedItem.References),
					len(parsedItem.Images),
					len(parsedItem.Tables)),
			},
		},
	}

	// Determine resource path based on input
	resourcePath := "parsed_pdf"
	if query.ZoteroID != "" {
		resourcePath = "parsed_pdf_" + query.ZoteroID
	}

	responseData := &PDFParseResponse{
		ParsedItem:   parsedItem,
		ResourcePath: resourcePath,
	}

	return result, responseData, nil
}
