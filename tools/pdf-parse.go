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

	// Build list of available resource paths
	resourcePaths := []string{
		fmt.Sprintf("pdf://%s", docID),
		fmt.Sprintf("pdf://%s/metadata", docID),
		fmt.Sprintf("pdf://%s/pages", docID),
	}

	// Add first and last page paths to indicate page numbering scheme
	if len(parsedItem.PageNumbers) > 0 {
		firstPage := parsedItem.PageNumbers[0]
		lastPage := parsedItem.PageNumbers[len(parsedItem.PageNumbers)-1]
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/pages/%s", docID, firstPage),
			fmt.Sprintf("pdf://%s/pages/%s", docID, lastPage),
		)
	}

	// Add page template
	resourcePaths = append(resourcePaths, fmt.Sprintf("pdf://%s/pages/{sourcePageNumber}", docID))

	if len(parsedItem.References) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/references", docID),
			fmt.Sprintf("pdf://%s/references/{refIndex}", docID),
		)
	}

	if len(parsedItem.Images) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/images", docID),
			fmt.Sprintf("pdf://%s/images/{imageIndex}", docID),
		)
	}

	if len(parsedItem.Tables) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/tables", docID),
			fmt.Sprintf("pdf://%s/tables/{tableIndex}", docID),
		)
	}

	if len(parsedItem.Footnotes) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/footnotes", docID),
			fmt.Sprintf("pdf://%s/footnotes/{footnoteIndex}", docID),
		)
	}

	if len(parsedItem.Endnotes) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/endnotes", docID),
			fmt.Sprintf("pdf://%s/endnotes/{endnoteIndex}", docID),
		)
	}

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
