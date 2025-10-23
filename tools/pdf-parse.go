package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Epistemic-Technology/zotero/zotero"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

type PDFParseQuery struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
}

type PDFParseResponse struct {
	ParsedItem   models.ParsedItem `json:"parsed_item"`
	ResourcePath string            `json:"resource_path,omitempty"`
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

	client := openai.NewClient(option.WithAPIKey(apiKey))

	// Get PDF data
	var data []byte
	var err error

	if query.RawData != nil {
		data = query.RawData
	} else if query.ZoteroID != "" {
		zoteroAPIKey := os.Getenv("ZOTERO_API_KEY")
		libraryID := os.Getenv("ZOTERO_LIBRARY_ID")
		data, err = getFromZotero(ctx, query.ZoteroID, zoteroAPIKey, libraryID)
		if err != nil {
			return nil, nil, err
		}
	} else if query.URL != "" {
		data, err = getFromURL(ctx, query.URL)
		if err != nil {
			return nil, nil, err
		}
	} else {
		return nil, nil, errors.New("no data provided")
	}

	if data == nil {
		return nil, nil, errors.New("no data retrieved")
	}

	// Convert PDF data to base64
	base64PDF := base64.StdEncoding.EncodeToString(data)

	// Create JSON schema for structured output
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"metadata": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":            map[string]any{"type": "string"},
					"authors":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"publication_date": map[string]any{"type": "string"},
					"publication":      map[string]any{"type": "string"},
					"doi":              map[string]any{"type": "string"},
					"abstract":         map[string]any{"type": "string"},
				},
				"required":             []string{"title", "authors", "publication_date", "publication", "doi", "abstract"},
				"additionalProperties": false,
			},
			"pages": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"references": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"reference_text": map[string]any{"type": "string"},
						"doi":            map[string]any{"type": "string"},
					},
					"required":             []string{"reference_text", "doi"},
					"additionalProperties": false,
				},
			},
			"images": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"image_url": map[string]any{"type": "string"},
						"caption":   map[string]any{"type": "string"},
					},
					"required":             []string{"image_url", "caption"},
					"additionalProperties": false,
				},
			},
			"tables": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_id":    map[string]any{"type": "string"},
						"table_title": map[string]any{"type": "string"},
						"table_data":  map[string]any{"type": "string"},
					},
					"required":             []string{"table_id", "table_title", "table_data"},
					"additionalProperties": false,
				},
			},
		},
		"additionalProperties": false,
		"required":             []string{"metadata", "pages", "references", "images", "tables"},
	}

	// Create response with structured output using Responses API
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ChatModelGPT5Mini,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentUnionParam{
							OfInputFile: &responses.ResponseInputFileParam{
								FileData: openai.String("data:application/pdf;base64," + base64PDF),
								Filename: openai.String("document.pdf"),
							},
						},
						responses.ResponseInputContentParamOfInputText(`Parse this academic paper and extract the following information in the specified JSON structure:

1. **metadata**: Extract bibliographic information
   - title: The paper's title
   - authors: Array of author names
   - publication_date: Date of publication
   - publication: Journal or conference name
   - doi: Digital Object Identifier if available
   - abstract: The paper's abstract

2. **pages**: Array of text content from each page (as strings)

3. **references**: Array of bibliography entries
   - reference_text: Full reference text
   - doi: DOI if identifiable in the reference

4. **images**: Array of figures/images found
   - image_url: Description of the image location or identifier
   - caption: Image caption text

5. **tables**: Array of tables found
   - table_id: Table identifier (e.g., "Table 1")
   - table_title: Table title/caption
   - table_data: Table content in a structured text format

Please be thorough and accurate in your extraction.`),
					},
					"user",
				),
			},
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("parsed_item", schema),
		},
	})

	if err != nil {
		return nil, nil, err
	}

	// Parse the structured response
	var parsedItem models.ParsedItem
	outputText := response.OutputText()
	if outputText == "" {
		return nil, nil, errors.New("no output text returned from response")
	}
	if err := json.Unmarshal([]byte(outputText), &parsedItem); err != nil {
		return nil, nil, errors.New("failed to parse structured response: " + err.Error())
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

func getFromURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func getFromZotero(ctx context.Context, zoteroID string, apiKey string, libraryID string) ([]byte, error) {
	client := zotero.NewClient(libraryID, zotero.LibraryTypeUser, zotero.WithAPIKey(apiKey))
	data, err := client.File(ctx, zoteroID)
	if err != nil {
		return nil, err
	}
	return data, nil
}
