package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/Epistemic-Technology/academic-mcp/internal/pdf"
	"github.com/Epistemic-Technology/academic-mcp/models"
)

func ParsePDFPage(ctx context.Context, apiKey string, page *models.PdfPageData) (*models.ParsedPage, error) {
	outputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type": "string",
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
						"image_url":         map[string]any{"type": "string"},
						"image_description": map[string]any{"type": "string"},
						"caption":           map[string]any{"type": "string"},
					},
					"required":             []string{"image_url", "image_description", "caption"},
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
		"required":             []string{"content", "references", "images", "tables"},
	}
	client := openai.NewClient(option.WithAPIKey(apiKey))
	encodedPageData := base64.StdEncoding.EncodeToString([]byte(*page))
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ChatModelGPT5Mini,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentUnionParam{
							OfInputFile: &responses.ResponseInputFileParam{
								FileData: openai.String("data:application/pdf;base64," + encodedPageData),
								Filename: openai.String("page.pdf"),
							},
						},
						responses.ResponseInputContentParamOfInputText(`Parse this page from an academic paper and extract it into the specified JSON structure. 1. Extract the main textual content of the page. This should exclude, any headers, footers, image captions, tables, and any other elements not part of the main content. Any columns should be concatenated in normal reading order. 2. If there are any bibliographic references (not in-text citations, but full bibliographic entries), extract those into the "references" array. 3. If there are any images on the page, extract the captions and textual descriptions of those images into the "images" array. 4. If there are any tables on the page, extract the table IDs, titles, and data into the "tables" array.`),
					},
					"user",
				),
			},
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("parsed_page", outputSchema),
		},
	},
	)
	if err != nil {
		return nil, err
	}
	var parsedPage models.ParsedPage
	outputText := response.OutputText()
	err = json.Unmarshal([]byte(outputText), &parsedPage)
	if err != nil {
		return nil, err
	}
	return &parsedPage, nil
}

func ParsePDF(ctx context.Context, apiKey string, pdfData models.PdfData) (*models.ParsedItem, error) {
	// Split the PDF into individual pages
	pages, err := pdf.SplitPdf(pdfData)
	if err != nil {
		return nil, err
	}

	// Create channels for results and errors
	type pageResult struct {
		pageNum int
		parsed  *models.ParsedPage
		err     error
	}
	results := make(chan pageResult, len(pages))

	// Process each page in parallel
	for i, page := range pages {
		go func(pageNum int, pageData *models.PdfPageData) {
			parsed, err := ParsePDFPage(ctx, apiKey, pageData)
			results <- pageResult{
				pageNum: pageNum,
				parsed:  parsed,
				err:     err,
			}
		}(i, &page)
	}

	// Collect results
	parsedPages := make([]*models.ParsedPage, len(pages))
	for range len(pages) {
		result := <-results
		if result.err != nil {
			return nil, result.err
		}
		parsedPages[result.pageNum] = result.parsed
	}
	close(results)

	// Stitch everything together
	var parsedItem models.ParsedItem
	parsedItem.Pages = make([]string, 0, len(parsedPages))
	parsedItem.References = make([]models.Reference, 0)
	parsedItem.Images = make([]models.Image, 0)
	parsedItem.Tables = make([]models.Table, 0)

	// Aggregate data from all pages
	for _, page := range parsedPages {
		if page != nil {
			parsedItem.Pages = append(parsedItem.Pages, page.Content)
			parsedItem.References = append(parsedItem.References, page.References...)
			parsedItem.Images = append(parsedItem.Images, page.Images...)
			parsedItem.Tables = append(parsedItem.Tables, page.Tables...)
		}
	}

	return &parsedItem, nil
}
