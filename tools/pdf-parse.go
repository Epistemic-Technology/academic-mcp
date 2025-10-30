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
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/Epistemic-Technology/academic-mcp/models"
)

type PDFParseQuery struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
}

type PDFParseResponse struct {
	DocumentID   string `json:"document_id"`
	ResourcePath string `json:"resource_path"`
	Title        string `json:"title,omitempty"`
	PageCount    int    `json:"page_count"`
	RefCount     int    `json:"reference_count"`
	ImageCount   int    `json:"image_count"`
	TableCount   int    `json:"table_count"`
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
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, nil, errors.New("OPENAI_API_KEY environment variable not set")
	}

	var data models.PdfData
	var err error
	sourceInfo := &models.SourceInfo{
		ZoteroID: query.ZoteroID,
		URL:      query.URL,
	}

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

	// Store the parsed item in the database
	docID, err := store.StoreParsedItem(ctx, parsedItem, sourceInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to store parsed item: %w", err)
	}

	// Format authors string
	authorsStr := "Unknown"
	if len(parsedItem.Metadata.Authors) > 0 {
		if len(parsedItem.Metadata.Authors) == 1 {
			authorsStr = parsedItem.Metadata.Authors[0]
		} else if len(parsedItem.Metadata.Authors) <= 3 {
			authorsStr = fmt.Sprintf("%s", parsedItem.Metadata.Authors)
		} else {
			authorsStr = fmt.Sprintf("%s et al.", parsedItem.Metadata.Authors[0])
		}
	}

	// Format publication date
	pubDateStr := ""
	if parsedItem.Metadata.PublicationDate != "" {
		pubDateStr = fmt.Sprintf(" (%s)", parsedItem.Metadata.PublicationDate)
	}

	// Format title
	titleStr := parsedItem.Metadata.Title
	if titleStr == "" {
		titleStr = "Unknown Title"
	}

	// Create response message
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully parsed and stored PDF document (ID: %s)\n\nTitle: %s\nAuthors: %s%s\n\nExtracted metadata, %d pages, %d references, %d images, and %d tables.\n\nAccess content via resources:\n- pdf://%s/metadata\n- pdf://%s/pages\n- pdf://%s/references\n- pdf://%s/images\n- pdf://%s/tables",
					docID,
					titleStr,
					authorsStr,
					pubDateStr,
					len(parsedItem.Pages),
					len(parsedItem.References),
					len(parsedItem.Images),
					len(parsedItem.Tables),
					docID, docID, docID, docID, docID),
			},
		},
	}

	responseData := &PDFParseResponse{
		DocumentID:   docID,
		ResourcePath: fmt.Sprintf("pdf://%s", docID),
		Title:        parsedItem.Metadata.Title,
		PageCount:    len(parsedItem.Pages),
		RefCount:     len(parsedItem.References),
		ImageCount:   len(parsedItem.Images),
		TableCount:   len(parsedItem.Tables),
	}

	return result, responseData, nil
}
