package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

// estimateTokens provides a rough estimate of token count for text
// Uses approximation of ~4 characters per token for English text
func estimateTokens(text string) int {
	return len(text) / 4
}

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
	case "md", "txt":
		return parseTextDocument(ctx, apiKey, docData, log)
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

	// Estimate token count before conversion
	originalTokens := estimateTokens(string(htmlData.Data))
	log.Info("Original HTML size: %d bytes (~%d tokens)", len(htmlData.Data), originalTokens)

	// Convert HTML to markdown to reduce context window usage
	log.Debug("Converting HTML to markdown")
	markdown, err := documents.PreprocessHTML(htmlData.Data)
	if err != nil {
		log.Error("Failed to convert HTML to markdown: %v", err)
		return nil, err
	}

	// Estimate token count after conversion
	markdownTokens := estimateTokens(markdown)
	reductionPercent := 100.0 * (1.0 - float64(len(markdown))/float64(len(htmlData.Data)))
	tokenReductionPercent := 100.0 * (1.0 - float64(markdownTokens)/float64(originalTokens))

	log.Info("Converted HTML to markdown:")
	log.Info("  Size: %d bytes → %d bytes (%.1f%% reduction)",
		len(htmlData.Data), len(markdown), reductionPercent)
	log.Info("  Tokens: ~%d → ~%d (%.1f%% reduction)",
		originalTokens, markdownTokens, tokenReductionPercent)

	markdownPreview := markdown
	if len(markdownPreview) > 200 {
		markdownPreview = markdownPreview[:200] + "..."
	}
	log.Debug("Calling OpenAI API with markdown preview: %s", markdownPreview)

	// Now that HTML is converted to markdown, use the text document parser
	mdData := models.DocumentData{
		Data: []byte(markdown),
		Type: "md",
	}
	return parseTextDocument(ctx, apiKey, mdData, log)
}

// parseTextDocument parses a text document (markdown or plain text) and returns a ParsedItem
func parseTextDocument(ctx context.Context, apiKey string, textData models.DocumentData, log logger.Logger) (*models.ParsedItem, error) {
	log.Info("Parsing text document (type: %s)", textData.Type)

	// Estimate token count for diagnostics
	contentTokens := estimateTokens(string(textData.Data))
	const promptTokens = 500 // Approximate prompt size
	totalTokens := contentTokens + promptTokens
	const tokenLimit = 400000

	log.Info("Document size: %d bytes (~%d tokens)", len(textData.Data), contentTokens)
	log.Info("Estimated total tokens: %d (content) + %d (prompt) = %d (limit: %d)",
		contentTokens, promptTokens, totalTokens, tokenLimit)

	if totalTokens > tokenLimit {
		log.Warn("Document may exceed context window! Estimated: %d tokens, Limit: %d tokens",
			totalTokens, tokenLimit)
	} else if totalTokens > tokenLimit*0.9 {
		log.Warn("Document is close to context window limit (%.1f%% used)",
			float64(totalTokens)/float64(tokenLimit)*100)
	}

	log.Debug("Calling OpenAI API for text parsing")
	client := openai.NewClient(option.WithAPIKey(apiKey))
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ChatModelGPT5Mini,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(`Parse this text document from an academic paper and extract it into the specified JSON structure.

1. Extract document metadata (title, authors, publication date, publication, doi, abstract) if present at the beginning.

2. Extract the main textual content:
   - If the document is already in markdown format, preserve the existing markdown syntax (headings, lists, emphasis, etc.).
   - If the document is plain text, convert it to markdown format by identifying section headings and marking them with appropriate heading levels.
   - Preserve paragraph structure.
   - Preserve footnote/endnote references.

3. If there are bibliographic references (full bibliographic entries, not in-text citations), extract those into the "references" array.

4. If there are images (markdown image syntax or image descriptions in text), extract them into the "images" array. For markdown images, use the image URL and alt text. For plain text, this array will typically be empty.

5. If there are tables (markdown tables or structured tabular data), extract their content into the "tables" array. For plain text, this array will typically be empty.

6. If there are footnotes (notes with markers at the bottom of pages), extract them into the "footnotes" array. Use empty strings for page_number and in_text_page fields since text documents don't have reliable page numbers.

7. If there are endnotes at the end of the document, extract them into the "endnotes" array. Use empty string for page_number field.

8. For page_number_info, use empty string for page_number, 0.0 for confidence, "none" for location, and empty string for page_range_info since text documents don't have page numbers.

Text Content:
` + string(textData.Data)),
					},
					"user",
				),
			},
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("parsed_text_document", parsedDocumentSchema),
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

// ExtractQuotations extracts representative quotations from a parsed document.
// For paginated documents (PDFs), it processes pages individually to maintain accurate page numbers.
// For non-paginated documents, it processes the entire content at once.
func ExtractQuotations(ctx context.Context, apiKey string, parsedItem *models.ParsedItem, summary string, maxQuotations int, log logger.Logger) ([]models.Quotation, error) {
	log.Info("Extracting quotations from document: %s (max: %d)", parsedItem.Metadata.Title, maxQuotations)

	// JSON schema for quotation extraction
	quotationSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"quotations": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"quotation_text": map[string]any{"type": "string"},
						"page_number":    map[string]any{"type": "string"},
						"context":        map[string]any{"type": "string"},
						"relevance":      map[string]any{"type": "string"},
					},
					"required":             []string{"quotation_text", "page_number", "context", "relevance"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"quotations"},
		"additionalProperties": false,
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	// Check if this is a paginated document (PDF with source page numbers)
	isPaginated := len(parsedItem.PageNumbers) > 0 && parsedItem.PageNumbers[0] != ""

	var quotations []models.Quotation
	var err error

	if isPaginated {
		// Process pages individually for PDFs
		log.Info("Processing %d pages individually for quotation extraction", len(parsedItem.Pages))
		quotations, err = extractQuotationsFromPages(ctx, &client, parsedItem, summary, quotationSchema, log)
	} else {
		// Process entire content at once for non-paginated documents
		log.Info("Processing entire document at once for quotation extraction")
		quotations, err = extractQuotationsFromFullText(ctx, &client, parsedItem, summary, quotationSchema, log)
	}

	if err != nil {
		return nil, err
	}

	// Apply max quotations limit if necessary
	if maxQuotations > 0 && len(quotations) > maxQuotations {
		log.Info("Found %d quotations, prioritizing to top %d", len(quotations), maxQuotations)
		quotations, err = prioritizeQuotations(ctx, &client, quotations, parsedItem, summary, maxQuotations, log)
		if err != nil {
			log.Error("Failed to prioritize quotations, returning all: %v", err)
			// Don't fail completely, just return all quotations if prioritization fails
			return quotations, nil
		}
		log.Info("Prioritization complete, returning %d quotations", len(quotations))
	}

	return quotations, nil
}

// extractQuotationsFromPages processes each page individually to extract quotations with accurate page numbers
func extractQuotationsFromPages(ctx context.Context, client *openai.Client, parsedItem *models.ParsedItem, summary string, schema map[string]any, log logger.Logger) ([]models.Quotation, error) {
	allQuotations := make([]models.Quotation, 0)

	// Process pages in parallel
	type pageResult struct {
		pageNum    int
		quotations []models.Quotation
		err        error
	}
	results := make(chan pageResult, len(parsedItem.Pages))

	for i, pageContent := range parsedItem.Pages {
		go func(pageIndex int, content string, sourcePageNum string) {
			log.Debug("Extracting quotations from page %d (source: %s)", pageIndex+1, sourcePageNum)

			prompt := fmt.Sprintf(`You are analyzing page %s of an academic document.

Document Summary:
%s

Document Title: %s
Page Content:
%s

Extract 0-3 representative quotations from this page. A good quotation should be:
- A direct quote from the text (exact wording)
- Significant in presenting key arguments, findings, or theoretical contributions
- Self-contained enough to be meaningful on its own
- Memorable or well-articulated
- NOT a citation or reference to other works

For each quotation, provide:
- quotation_text: The exact quoted text (use quotes around it)
- page_number: "%s" (the source page number)
- context: Brief explanation of where this appears (e.g., "in the introduction", "from the methodology section")
- relevance: Why this quotation is significant (key argument, important finding, etc.)

If there are no suitable quotations on this page, return an empty array.`,
				sourcePageNum, summary, parsedItem.Metadata.Title, content, sourcePageNum)

			response, err := client.Responses.New(ctx, responses.ResponseNewParams{
				Model: shared.ChatModelGPT5Mini,
				Input: responses.ResponseNewParamsInputUnion{
					OfInputItemList: responses.ResponseInputParam{
						responses.ResponseInputItemParamOfMessage(
							responses.ResponseInputMessageContentListParam{
								responses.ResponseInputContentParamOfInputText(prompt),
							},
							"user",
						),
					},
				},
				Text: responses.ResponseTextConfigParam{
					Format: responses.ResponseFormatTextConfigParamOfJSONSchema("quotations", schema),
				},
			})

			if err != nil {
				log.Error("Failed to extract quotations from page %d: %v", pageIndex+1, err)
				results <- pageResult{pageNum: pageIndex, quotations: nil, err: err}
				return
			}

			var result struct {
				Quotations []models.Quotation `json:"quotations"`
			}
			outputText := response.OutputText()
			err = json.Unmarshal([]byte(outputText), &result)
			if err != nil {
				log.Error("Failed to parse quotations from page %d: %v", pageIndex+1, err)
				results <- pageResult{pageNum: pageIndex, quotations: nil, err: err}
				return
			}

			log.Debug("Found %d quotations on page %d", len(result.Quotations), pageIndex+1)
			results <- pageResult{pageNum: pageIndex, quotations: result.Quotations, err: nil}
		}(i, pageContent, parsedItem.PageNumbers[i])
	}

	// Collect results
	pageQuotations := make(map[int][]models.Quotation)
	for range len(parsedItem.Pages) {
		result := <-results
		if result.err != nil {
			close(results)
			return nil, result.err
		}
		pageQuotations[result.pageNum] = result.quotations
	}
	close(results)

	// Aggregate in page order
	for i := 0; i < len(parsedItem.Pages); i++ {
		if quotes, ok := pageQuotations[i]; ok {
			allQuotations = append(allQuotations, quotes...)
		}
	}

	log.Info("Successfully extracted %d quotations from %d pages", len(allQuotations), len(parsedItem.Pages))
	return allQuotations, nil
}

// extractQuotationsFromFullText processes the entire document at once for non-paginated documents
func extractQuotationsFromFullText(ctx context.Context, client *openai.Client, parsedItem *models.ParsedItem, summary string, schema map[string]any, log logger.Logger) ([]models.Quotation, error) {
	fullContent := strings.Join(parsedItem.Pages, "\n")

	prompt := fmt.Sprintf(`You are analyzing an academic document.

Document Summary:
%s

Document Title: %s
Full Content:
%s

Extract 5-15 representative quotations from this document. A good quotation should be:
- A direct quote from the text (exact wording)
- Significant in presenting key arguments, findings, or theoretical contributions
- Self-contained enough to be meaningful on its own
- Memorable or well-articulated
- NOT a citation or reference to other works
- Distributed throughout the document (introduction, body, conclusion)

For each quotation, provide:
- quotation_text: The exact quoted text (use quotes around it)
- page_number: "" (empty string since this document doesn't have page numbers)
- context: Brief explanation of where this appears (e.g., "in the introduction", "from the methodology section")
- relevance: Why this quotation is significant (key argument, important finding, etc.)`,
		summary, parsedItem.Metadata.Title, fullContent)

	log.Debug("Calling OpenAI API for full-text quotation extraction")
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ChatModelGPT5Mini,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(prompt),
					},
					"user",
				),
			},
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("quotations", schema),
		},
	})

	if err != nil {
		log.Error("Failed to extract quotations: %v", err)
		return nil, err
	}

	var result struct {
		Quotations []models.Quotation `json:"quotations"`
	}
	outputText := response.OutputText()
	err = json.Unmarshal([]byte(outputText), &result)
	if err != nil {
		log.Error("Failed to parse quotations: %v", err)
		return nil, err
	}

	log.Info("Successfully extracted %d quotations from document", len(result.Quotations))
	return result.Quotations, nil
}

// prioritizeQuotations takes a list of quotations and asks the LLM to select the most significant ones
func prioritizeQuotations(ctx context.Context, client *openai.Client, quotations []models.Quotation, parsedItem *models.ParsedItem, summary string, maxQuotations int, log logger.Logger) ([]models.Quotation, error) {
	log.Info("Prioritizing %d quotations down to %d", len(quotations), maxQuotations)

	// Build a JSON representation of the quotations for the LLM
	quotationsJSON, err := json.MarshalIndent(quotations, "", "  ")
	if err != nil {
		log.Error("Failed to marshal quotations for prioritization: %v", err)
		return nil, err
	}

	prompt := fmt.Sprintf(`You are reviewing quotations extracted from an academic document and need to select the %d most significant ones.

Document Title: %s
Document Summary:
%s

All Extracted Quotations:
%s

Your task is to select the %d MOST significant quotations from the list above. Prioritize quotations that:
1. Present key arguments or theoretical contributions
2. Contain important findings or conclusions
3. Are memorable or particularly well-articulated
4. Represent different sections of the document (diversity)
5. Are self-contained and meaningful

Return ONLY the selected quotations in the exact same format (with quotation_text, page_number, context, and relevance preserved exactly as provided). Do not modify the quotation text or metadata.

Select exactly %d quotations (or fewer if there aren't enough high-quality ones).`,
		maxQuotations, parsedItem.Metadata.Title, summary, string(quotationsJSON), maxQuotations, maxQuotations)

	// JSON schema for the response
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"quotations": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"quotation_text": map[string]any{"type": "string"},
						"page_number":    map[string]any{"type": "string"},
						"context":        map[string]any{"type": "string"},
						"relevance":      map[string]any{"type": "string"},
					},
					"required":             []string{"quotation_text", "page_number", "context", "relevance"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"quotations"},
		"additionalProperties": false,
	}

	log.Debug("Calling OpenAI API for quotation prioritization")
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ChatModelGPT5Mini,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(prompt),
					},
					"user",
				),
			},
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("prioritized_quotations", schema),
		},
	})

	if err != nil {
		log.Error("Failed to prioritize quotations: %v", err)
		return nil, err
	}

	var result struct {
		Quotations []models.Quotation `json:"quotations"`
	}
	outputText := response.OutputText()
	err = json.Unmarshal([]byte(outputText), &result)
	if err != nil {
		log.Error("Failed to parse prioritized quotations: %v", err)
		return nil, err
	}

	log.Info("Successfully prioritized to %d quotations", len(result.Quotations))
	return result.Quotations, nil
}
