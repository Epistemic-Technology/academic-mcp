package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/Epistemic-Technology/academic-mcp/internal/documents"
	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/models"
)

var (

	// parsedDocumentSchema is the unified JSON schema for parsing all document types
	// For non-PDF documents: page_number_info fields will be empty/zero values
	// For text-only documents: images and tables arrays will be empty
	parsedDocumentSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"metadata": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{
						"type": "string",
					},
					"authors": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
					"publication_date": map[string]any{
						"type": "string",
					},
					"publication": map[string]any{
						"type": "string",
					},
					"doi": map[string]any{
						"type": "string",
					},
					"abstract": map[string]any{
						"type": "string",
					},
				},
				"required":             []string{"title", "authors", "publication_date", "publication", "doi", "abstract"},
				"additionalProperties": false,
			},
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
			"footnotes": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"marker":       map[string]any{"type": "string"},
						"text":         map[string]any{"type": "string"},
						"page_number":  map[string]any{"type": "string"},
						"in_text_page": map[string]any{"type": "string"},
					},
					"required":             []string{"marker", "text", "page_number", "in_text_page"},
					"additionalProperties": false,
				},
			},
			"endnotes": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"marker":      map[string]any{"type": "string"},
						"text":        map[string]any{"type": "string"},
						"page_number": map[string]any{"type": "string"},
					},
					"required":             []string{"marker", "text", "page_number"},
					"additionalProperties": false,
				},
			},
			"page_number_info": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_number": map[string]any{
						"type": "string",
					},
					"confidence": map[string]any{
						"type":    "number",
						"minimum": 0.0,
						"maximum": 1.0,
					},
					"location": map[string]any{
						"type": "string",
					},
					"page_range_info": map[string]any{
						"type": "string",
					},
				},
				"required":             []string{"page_number", "confidence", "location", "page_range_info"},
				"additionalProperties": false,
			},
		},
		"additionalProperties": false,
		"required":             []string{"metadata", "content", "references", "images", "tables", "footnotes", "endnotes", "page_number_info"},
	}
)

func ParsePDFPage(ctx context.Context, apiKey string, page *models.DocumentPageData) (*models.ParsedPage, error) {
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
						responses.ResponseInputContentParamOfInputText(`Parse this page from an academic paper and extract it into the specified JSON structure.

1. If there is document metadata on the page (title, authors, publication date, publication, doi, abstract), extract those into the "metadata" object.

2. Extract the main textual content of the page.
	- Use markdown syntax to format the text.
	- This should exclude any headers, footers, image captions, tables, and any other elements not part of the main content.
	- Any columns should be concatenated in normal reading order.
	- Footnote or endnote references (normally as superscripts) should be included in the main text using square brackets eg. [1].
	- Try to identify section headings (for example by font size or weight).

3. If there are any bibliographic references (not in-text citations, but full bibliographic entries), extract those into the "references" array. Note that footnotes are not references. We're looking for a bibliography or works cited section or similar.

4. If there are any images on the page, extract the captions and textual descriptions of those images into the "images" array.

5. If there are any tables on the page, extract the table IDs, titles, and data into the "tables" array.

6. If there are any footnotes on this page (notes appearing at the bottom of the page), extract them into the "footnotes" array:
   - "marker": The footnote marker/number (e.g., "1", "2", "*", "†", "a")
   - "text": The full text of the footnote
   - "page_number": The page number where this footnote appears (use the detected page number from step 8)
   - "in_text_page": The page number where the footnote marker appears in the main text (usually the same as page_number, but could differ)

7. If there are any endnotes on this page (notes collected at the end of a chapter/document), extract them into the "endnotes" array:
   - "marker": The endnote marker/number (e.g., "1", "2", "i", "ii")
   - "text": The full text of the endnote
   - "page_number": The page number where this endnote definition appears

   IMPORTANT: Distinguish between footnotes and endnotes:
   - Footnotes appear at the bottom of the same page as their marker
   - Endnotes are collected in a dedicated section, often at the end of chapters or the document
   - Do NOT confuse bibliographic references with footnotes or endnotes

8. Extract page numbering information into "page_number_info":
   - "page_number": The printed page number visible on this page (e.g., "125", "iv", "A-3"). Look in headers, footers, margins, and corners. If no page number is visible, use an empty string "".
   - "confidence": Your confidence level (0.0-1.0) that the page number is correct. Use 1.0 for clearly printed numbers, 0.5-0.8 for ambiguous cases, and 0.0 if no number is found.
   - "location": Where the page number appears (e.g., "bottom center", "top right", "footer", "none" if not found).
   - "page_range_info": Any page range information from the header or title page (e.g., "Pages 125-150" or "pp. 42-68"). Use empty string "" if none found.

IMPORTANT for page numbers: Be conservative. Only report page numbers with high confidence. Consider that:
- The first page may be unnumbered (title page or cover)
- Chapter first pages are often unnumbered
- Pages with full-bleed images may be unnumbered
- Blank pages may be unnumbered
- Do not confuse section numbers, figure numbers, or other numbers with page numbers`),
					},
					"user",
				),
			},
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("parsed_page", parsedDocumentSchema),
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

// ParseDocument parses a document based on its type and returns a ParsedItem
func ParseDocument(ctx context.Context, apiKey string, docData models.DocumentData, log logger.Logger) (*models.ParsedItem, error) {
	log.Info("Parsing document of type: %s", docData.Type)
	switch docData.Type {
	case "pdf":
		return parsePDF(ctx, apiKey, docData, log)
	case "html":
		return parseHTML(ctx, apiKey, docData, log)
	case "md":
		return parseMarkdown(ctx, apiKey, docData, log)
	case "txt":
		return parseText(ctx, apiKey, docData, log)
	case "docx":
		// TODO: Implement DOCX parsing
		log.Error("Unsupported document type: docx")
		return nil, errors.New("unsupported document type: docx")
	default:
		log.Error("Unsupported document type: %s", docData.Type)
		return nil, errors.New("unsupported document type")
	}
}

// parsePDF parses a PDF document and returns a ParsedItem
func parsePDF(ctx context.Context, apiKey string, pdfData models.DocumentData, log logger.Logger) (*models.ParsedItem, error) {
	// Split the PDF into individual pages
	pages, err := documents.SplitPdf(pdfData)
	if err != nil {
		log.Error("Failed to split PDF into pages: %v", err)
		return nil, err
	}

	log.Info("Processing PDF with %d pages (parallel)", len(pages))

	// Create channels for results and errors
	type pageResult struct {
		pageNum int
		parsed  *models.ParsedPage
		err     error
	}
	results := make(chan pageResult, len(pages))

	// Process each page in parallel
	for i, page := range pages {
		go func(pageNum int, pageData *models.DocumentPageData) {
			log.Debug("Calling OpenAI API for page %d", pageNum+1)
			parsed, err := ParsePDFPage(ctx, apiKey, pageData)
			if err != nil {
				log.Error("Failed to parse page %d: %v", pageNum+1, err)
			}
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

	log.Info("Successfully parsed all %d pages", len(pages))

	// Validate and determine page numbering scheme
	pageNumbers := validatePageNumbers(parsedPages)

	// Stitch everything together
	var parsedItem models.ParsedItem
	parsedItem.Pages = make([]string, 0, len(parsedPages))
	parsedItem.PageNumbers = pageNumbers
	parsedItem.References = make([]models.Reference, 0)
	parsedItem.Images = make([]models.Image, 0)
	parsedItem.Tables = make([]models.Table, 0)
	parsedItem.Footnotes = make([]models.Footnote, 0)
	parsedItem.Endnotes = make([]models.Endnote, 0)

	// Aggregate data from all pages
	for _, page := range parsedPages {
		if page != nil {
			if page.Metadata.Title != "" && parsedItem.Metadata.Title == "" {
				parsedItem.Metadata.Title = page.Metadata.Title
			}
			if len(page.Metadata.Authors) > 0 && len(parsedItem.Metadata.Authors) == 0 {
				parsedItem.Metadata.Authors = page.Metadata.Authors
			}
			if page.Metadata.PublicationDate != "" && parsedItem.Metadata.PublicationDate == "" {
				parsedItem.Metadata.PublicationDate = page.Metadata.PublicationDate
			}
			if page.Metadata.Publication != "" && parsedItem.Metadata.Publication == "" {
				parsedItem.Metadata.Publication = page.Metadata.Publication
			}
			if page.Metadata.DOI != "" && parsedItem.Metadata.DOI == "" {
				parsedItem.Metadata.DOI = page.Metadata.DOI
			}
			if page.Metadata.Abstract != "" && parsedItem.Metadata.Abstract == "" {
				parsedItem.Metadata.Abstract = page.Metadata.Abstract
			}

			parsedItem.Pages = append(parsedItem.Pages, page.Content)
			parsedItem.References = append(parsedItem.References, page.References...)
			parsedItem.Images = append(parsedItem.Images, page.Images...)
			parsedItem.Tables = append(parsedItem.Tables, page.Tables...)
			parsedItem.Footnotes = append(parsedItem.Footnotes, page.Footnotes...)
			parsedItem.Endnotes = append(parsedItem.Endnotes, page.Endnotes...)
		}
	}

	return &parsedItem, nil
}

// parseHTML parses an HTML document and returns a ParsedItem
func parseHTML(ctx context.Context, apiKey string, htmlData models.DocumentData, log logger.Logger) (*models.ParsedItem, error) {
	log.Info("Parsing HTML document")
	dataPreview := string(htmlData.Data)
	if len(dataPreview) > 200 {
		dataPreview = dataPreview[:200] + "..."
	}
	log.Debug("Calling OpenAI API with data length: %d and data preview: %s", len(dataPreview), dataPreview)
	client := openai.NewClient(option.WithAPIKey(apiKey))
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ChatModelGPT5Mini,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(`Parse this HTML document from an academic paper and extract it into the specified JSON structure.

1. Extract document metadata (title, authors, publication date, publication, doi, abstract) if present in the HTML.

2. Extract the main textual content of the document.
   - Convert to markdown syntax to format the text.
   - Exclude navigation, headers, footers, sidebars, and other non-content elements.
   - Include section headings using appropriate markdown heading levels.
   - Preserve footnote/endnote references using square brackets eg. [1].

3. If there are bibliographic references (not in-text citations, but full bibliographic entries), extract those into the "references" array.

4. If there are images, extract captions and alt text into the "images" array. Use the img src attribute for image_url.

5. If there are tables (HTML <table> elements), extract their content into the "tables" array.

6. If there are footnotes (typically marked with <sup> or similar), extract them into the "footnotes" array. Use empty strings for page_number and in_text_page fields since HTML doesn't have page numbers.

7. If there are endnotes (notes at the end of the document), extract them into the "endnotes" array. Use empty string for page_number field.

8. For page_number_info, use empty string for page_number, 0.0 for confidence, "none" for location, and empty string for page_range_info since HTML documents don't have page numbers.

HTML Content:
` + string(htmlData.Data)),
					},
					"user",
				),
			},
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("parsed_html", parsedDocumentSchema),
		},
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Metadata   models.ItemMetadata `json:"metadata"`
		Content    string              `json:"content"`
		References []models.Reference  `json:"references"`
		Images     []models.Image      `json:"images"`
		Tables     []models.Table      `json:"tables"`
		Footnotes  []models.Footnote   `json:"footnotes"`
		Endnotes   []models.Endnote    `json:"endnotes"`
	}

	outputText := response.OutputText()
	err = json.Unmarshal([]byte(outputText), &result)
	if err != nil {
		return nil, err
	}

	return &models.ParsedItem{
		Metadata:    result.Metadata,
		Pages:       []string{result.Content},
		PageNumbers: []string{"1"},
		References:  result.References,
		Images:      result.Images,
		Tables:      result.Tables,
		Footnotes:   result.Footnotes,
		Endnotes:    result.Endnotes,
	}, nil
}

// parseMarkdown parses a Markdown document and returns a ParsedItem
func parseMarkdown(ctx context.Context, apiKey string, mdData models.DocumentData, log logger.Logger) (*models.ParsedItem, error) {
	log.Info("Parsing Markdown document")
	log.Debug("Calling OpenAI API for Markdown parsing")
	client := openai.NewClient(option.WithAPIKey(apiKey))
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ChatModelGPT5Mini,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(`Parse this Markdown document from an academic paper and extract it into the specified JSON structure.

1. Extract document metadata (title, authors, publication date, publication, doi, abstract) if present. Look for YAML frontmatter or metadata at the beginning.

2. Extract the main textual content, preserving the markdown formatting.
   - Keep all markdown syntax (headings, lists, emphasis, etc.).
   - Preserve footnote/endnote references.

3. If there are bibliographic references (full bibliographic entries), extract those into the "references" array.

4. If there are images (markdown image syntax), extract them into the "images" array. Use the image URL and alt text.

5. If there are markdown tables, extract their content into the "tables" array.

6. If there are footnotes (typically marked with [^1] syntax), extract them into the "footnotes" array. Use empty strings for page_number and in_text_page fields since Markdown doesn't have page numbers.

7. If there are endnotes at the end of the document, extract them into the "endnotes" array. Use empty string for page_number field.

8. For page_number_info, use empty string for page_number, 0.0 for confidence, "none" for location, and empty string for page_range_info since Markdown documents don't have page numbers.

Markdown Content:
` + string(mdData.Data)),
					},
					"user",
				),
			},
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("parsed_markdown", parsedDocumentSchema),
		},
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Metadata   models.ItemMetadata `json:"metadata"`
		Content    string              `json:"content"`
		References []models.Reference  `json:"references"`
		Images     []models.Image      `json:"images"`
		Tables     []models.Table      `json:"tables"`
		Footnotes  []models.Footnote   `json:"footnotes"`
		Endnotes   []models.Endnote    `json:"endnotes"`
	}

	outputText := response.OutputText()
	err = json.Unmarshal([]byte(outputText), &result)
	if err != nil {
		return nil, err
	}

	return &models.ParsedItem{
		Metadata:    result.Metadata,
		Pages:       []string{result.Content},
		PageNumbers: []string{"1"},
		References:  result.References,
		Images:      result.Images,
		Tables:      result.Tables,
		Footnotes:   result.Footnotes,
		Endnotes:    result.Endnotes,
	}, nil
}

// parseText parses a plain text document and returns a ParsedItem
func parseText(ctx context.Context, apiKey string, textData models.DocumentData, log logger.Logger) (*models.ParsedItem, error) {
	log.Info("Parsing plain text document")
	log.Debug("Calling OpenAI API for text parsing")
	client := openai.NewClient(option.WithAPIKey(apiKey))
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ChatModelGPT5Mini,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(`Parse this plain text document from an academic paper and extract it into the specified JSON structure.

1. Extract document metadata (title, authors, publication date, publication, doi, abstract) if present at the beginning of the text.

2. Extract the main textual content, converting it to markdown format:
   - Identify section headings and mark them with appropriate markdown heading levels.
   - Preserve paragraph structure.
   - Preserve footnote/endnote references using square brackets eg. [1].

3. If there are bibliographic references (full bibliographic entries, typically at the end), extract those into the "references" array.

4. If there are footnotes (notes with markers), extract them into the "footnotes" array. Use empty strings for page_number and in_text_page fields since plain text doesn't have reliable page numbers.

5. If there are endnotes at the end of the document, extract them into the "endnotes" array. Use empty string for page_number field.

6. For page_number_info, use empty string for page_number, 0.0 for confidence, "none" for location, and empty string for page_range_info since plain text documents don't have page numbers.

Note: Plain text documents won't have images or tables, so those arrays will be empty.

Text Content:
` + string(textData.Data)),
					},
					"user",
				),
			},
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("parsed_text", parsedDocumentSchema),
		},
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Metadata   models.ItemMetadata `json:"metadata"`
		Content    string              `json:"content"`
		References []models.Reference  `json:"references"`
		Footnotes  []models.Footnote   `json:"footnotes"`
		Endnotes   []models.Endnote    `json:"endnotes"`
	}

	outputText := response.OutputText()
	err = json.Unmarshal([]byte(outputText), &result)
	if err != nil {
		return nil, err
	}

	return &models.ParsedItem{
		Metadata:    result.Metadata,
		Pages:       []string{result.Content},
		PageNumbers: []string{"1"},
		References:  result.References,
		Images:      []models.Image{},
		Tables:      []models.Table{},
		Footnotes:   result.Footnotes,
		Endnotes:    result.Endnotes,
	}, nil
}

func SummarizeItem(ctx context.Context, apiKey string, pdfData *models.ParsedItem, log logger.Logger) (string, error) {
	log.Info("Generating summary for document: %s", pdfData.Metadata.Title)
	fullContent := strings.Join(pdfData.Pages, "\n")
	log.Debug("Calling OpenAI API for summarization (content length: %d chars)", len(fullContent))
	client := openai.NewClient(option.WithAPIKey(apiKey))
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ChatModelGPT5Mini,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(`Summarize this academic text into 1-3 paragraphs. It should be coherent, concise, accurately reflect the original content, and use a detached academic tone. This should be in expository prose, not point form. No lists, just coherent sentences and paragraphs.

` + fullContent),
					},
					"user",
				),
			},
		},
	})
	if err != nil {
		log.Error("Failed to generate summary: %v", err)
		return "", err
	}
	log.Info("Successfully generated summary")
	return response.OutputText(), nil
}
