package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
)

// PDFResourceHandler handles resource requests for parsed PDF documents
type PDFResourceHandler struct {
	store storage.Store
}

// NewPDFResourceHandler creates a new PDF resource handler
func NewPDFResourceHandler(store storage.Store) *PDFResourceHandler {
	return &PDFResourceHandler{store: store}
}

// ListResources returns a list of available resources
func (h *PDFResourceHandler) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	docs, err := h.store.ListDocuments(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	var resources []mcp.Resource
	for _, doc := range docs {
		// Add main document resource
		resources = append(resources, mcp.Resource{
			URI:         fmt.Sprintf("pdf://%s", doc.DocumentID),
			Name:        fmt.Sprintf("%s (Document)", doc.Title),
			Description: fmt.Sprintf("Parsed PDF document: %s", doc.Title),
			MIMEType:    "application/json",
		})

		// Add metadata resource
		resources = append(resources, mcp.Resource{
			URI:         fmt.Sprintf("pdf://%s/metadata", doc.DocumentID),
			Name:        fmt.Sprintf("%s (Metadata)", doc.Title),
			Description: "Document metadata including title, authors, DOI, and abstract",
			MIMEType:    "application/json",
		})

		// Add pages resource
		resources = append(resources, mcp.Resource{
			URI:         fmt.Sprintf("pdf://%s/pages", doc.DocumentID),
			Name:        fmt.Sprintf("%s (All Pages)", doc.Title),
			Description: "All pages of the document",
			MIMEType:    "application/json",
		})

		// Add references resource
		resources = append(resources, mcp.Resource{
			URI:         fmt.Sprintf("pdf://%s/references", doc.DocumentID),
			Name:        fmt.Sprintf("%s (References)", doc.Title),
			Description: "All references cited in the document",
			MIMEType:    "application/json",
		})

		// Add images resource
		resources = append(resources, mcp.Resource{
			URI:         fmt.Sprintf("pdf://%s/images", doc.DocumentID),
			Name:        fmt.Sprintf("%s (Images)", doc.Title),
			Description: "All images from the document",
			MIMEType:    "application/json",
		})

		// Add tables resource
		resources = append(resources, mcp.Resource{
			URI:         fmt.Sprintf("pdf://%s/tables", doc.DocumentID),
			Name:        fmt.Sprintf("%s (Tables)", doc.Title),
			Description: "All tables from the document",
			MIMEType:    "application/json",
		})

		// Add footnotes resource
		resources = append(resources, mcp.Resource{
			URI:         fmt.Sprintf("pdf://%s/footnotes", doc.DocumentID),
			Name:        fmt.Sprintf("%s (Footnotes)", doc.Title),
			Description: "All footnotes from the document",
			MIMEType:    "application/json",
		})

		// Add endnotes resource
		resources = append(resources, mcp.Resource{
			URI:         fmt.Sprintf("pdf://%s/endnotes", doc.DocumentID),
			Name:        fmt.Sprintf("%s (Endnotes)", doc.Title),
			Description: "All endnotes from the document",
			MIMEType:    "application/json",
		})
	}

	return resources, nil
}

// ReadResource reads a specific resource by URI
func (h *PDFResourceHandler) ReadResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	// Parse URI: pdf://doc_id/resource_type/optional_index
	if !strings.HasPrefix(uri, "pdf://") {
		return nil, fmt.Errorf("invalid URI scheme, expected pdf://")
	}

	path := strings.TrimPrefix(uri, "pdf://")
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid URI, missing document ID")
	}

	docID := parts[0]
	resourceType := ""
	index := -1

	if len(parts) > 1 {
		resourceType = parts[1]
	}
	if len(parts) > 2 {
		var err error
		index, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid index: %s", parts[2])
		}
	}

	var content string
	var err error

	switch resourceType {
	case "":
		// Return document summary
		content, err = h.getDocumentSummary(ctx, docID)
	case "metadata":
		content, err = h.getMetadata(ctx, docID)
	case "pages":
		if len(parts) > 2 {
			// Try to get page by source page number (e.g., "125" or "iv")
			pageIdentifier := parts[2]
			content, err = h.getPageByIdentifier(ctx, docID, pageIdentifier)
		} else {
			content, err = h.getAllPages(ctx, docID)
		}
	case "references":
		if index >= 0 {
			content, err = h.getReference(ctx, docID, index)
		} else {
			content, err = h.getAllReferences(ctx, docID)
		}
	case "images":
		if index >= 0 {
			content, err = h.getImage(ctx, docID, index)
		} else {
			content, err = h.getAllImages(ctx, docID)
		}
	case "tables":
		if index >= 0 {
			content, err = h.getTable(ctx, docID, index)
		} else {
			content, err = h.getAllTables(ctx, docID)
		}
	case "footnotes":
		if index >= 0 {
			content, err = h.getFootnote(ctx, docID, index)
		} else {
			content, err = h.getAllFootnotes(ctx, docID)
		}
	case "endnotes":
		if index >= 0 {
			content, err = h.getEndnote(ctx, docID, index)
		} else {
			content, err = h.getAllEndnotes(ctx, docID)
		}
	case "quotations":
		if index >= 0 {
			content, err = h.getQuotation(ctx, docID, index)
		} else {
			content, err = h.getAllQuotations(ctx, docID)
		}
	default:
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}

	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      uri,
				MIMEType: "application/json",
				Text:     content,
			},
		},
	}, nil
}

// Helper functions to retrieve specific content

func (h *PDFResourceHandler) getDocumentSummary(ctx context.Context, docID string) (string, error) {
	metadata, err := h.store.GetMetadata(ctx, docID)
	if err != nil {
		return "", err
	}

	pages, err := h.store.GetPages(ctx, docID)
	if err != nil {
		return "", err
	}

	refs, err := h.store.GetReferences(ctx, docID)
	if err != nil {
		return "", err
	}

	images, err := h.store.GetImages(ctx, docID)
	if err != nil {
		return "", err
	}

	tables, err := h.store.GetTables(ctx, docID)
	if err != nil {
		return "", err
	}

	footnotes, err := h.store.GetFootnotes(ctx, docID)
	if err != nil {
		return "", err
	}

	endnotes, err := h.store.GetEndnotes(ctx, docID)
	if err != nil {
		return "", err
	}

	quotations, err := h.store.GetQuotations(ctx, docID)
	if err != nil {
		return "", err
	}

	summary := map[string]interface{}{
		"document_id":     docID,
		"metadata":        metadata,
		"page_count":      len(pages),
		"ref_count":       len(refs),
		"image_count":     len(images),
		"table_count":     len(tables),
		"footnote_count":  len(footnotes),
		"endnote_count":   len(endnotes),
		"quotation_count": len(quotations),
		"available_resources": []string{
			fmt.Sprintf("pdf://%s/metadata", docID),
			fmt.Sprintf("pdf://%s/pages", docID),
			fmt.Sprintf("pdf://%s/references", docID),
			fmt.Sprintf("pdf://%s/images", docID),
			fmt.Sprintf("pdf://%s/tables", docID),
			fmt.Sprintf("pdf://%s/footnotes", docID),
			fmt.Sprintf("pdf://%s/endnotes", docID),
			fmt.Sprintf("pdf://%s/quotations", docID),
		},
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal summary: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getMetadata(ctx context.Context, docID string) (string, error) {
	metadata, err := h.store.GetMetadata(ctx, docID)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getPage(ctx context.Context, docID string, pageNum int) (string, error) {
	page, err := h.store.GetPage(ctx, docID, pageNum+1) // Convert 0-indexed to 1-indexed
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"page_number": pageNum,
		"content":     page,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal page: %w", err)
	}

	return string(data), nil
}

// getPageByIdentifier retrieves a page by source page number (e.g., "125", "iv")
func (h *PDFResourceHandler) getPageByIdentifier(ctx context.Context, docID string, pageIdentifier string) (string, error) {
	// Try to get page by source page number
	content, err := h.store.GetPageBySourceNumber(ctx, docID, pageIdentifier)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"source_page_number": pageIdentifier,
		"content":            content,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal page: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getAllPages(ctx context.Context, docID string) (string, error) {
	pages, err := h.store.GetPages(ctx, docID)
	if err != nil {
		return "", err
	}

	// Get page mapping to include source page numbers
	mapping, err := h.store.GetPageMapping(ctx, docID)
	if err != nil {
		return "", err
	}

	// Build reverse mapping (sequential -> source)
	reverseMapping := make(map[int]string)
	for source, seq := range mapping {
		reverseMapping[seq] = source
	}

	// Build page list with both sequential and source numbers
	type pageInfo struct {
		SequentialNumber int    `json:"sequential_number"`
		SourcePageNumber string `json:"source_page_number"`
		Content          string `json:"content"`
	}

	pageList := make([]pageInfo, len(pages))
	for i, content := range pages {
		sourceNum := reverseMapping[i+1] // i+1 because pages are 1-indexed in DB
		if sourceNum == "" {
			sourceNum = fmt.Sprintf("%d", i+1)
		}
		pageList[i] = pageInfo{
			SequentialNumber: i + 1,
			SourcePageNumber: sourceNum,
			Content:          content,
		}
	}

	result := map[string]interface{}{
		"page_count": len(pages),
		"pages":      pageList,
		"note":       "Access individual pages using source page numbers, e.g., pdf://" + docID + "/pages/125",
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal pages: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getReference(ctx context.Context, docID string, refIndex int) (string, error) {
	ref, err := h.store.GetReference(ctx, docID, refIndex)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal reference: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getAllReferences(ctx context.Context, docID string) (string, error) {
	refs, err := h.store.GetReferences(ctx, docID)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"reference_count": len(refs),
		"references":      refs,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal references: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getImage(ctx context.Context, docID string, imageIndex int) (string, error) {
	img, err := h.store.GetImage(ctx, docID, imageIndex)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(img, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal image: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getAllImages(ctx context.Context, docID string) (string, error) {
	images, err := h.store.GetImages(ctx, docID)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"image_count": len(images),
		"images":      images,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal images: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getTable(ctx context.Context, docID string, tableIndex int) (string, error) {
	tbl, err := h.store.GetTable(ctx, docID, tableIndex)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(tbl, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal table: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getAllTables(ctx context.Context, docID string) (string, error) {
	tables, err := h.store.GetTables(ctx, docID)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"table_count": len(tables),
		"tables":      tables,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal tables: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getFootnote(ctx context.Context, docID string, footnoteIndex int) (string, error) {
	footnote, err := h.store.GetFootnote(ctx, docID, footnoteIndex)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(footnote, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal footnote: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getAllFootnotes(ctx context.Context, docID string) (string, error) {
	footnotes, err := h.store.GetFootnotes(ctx, docID)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"footnote_count": len(footnotes),
		"footnotes":      footnotes,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal footnotes: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getEndnote(ctx context.Context, docID string, endnoteIndex int) (string, error) {
	endnote, err := h.store.GetEndnote(ctx, docID, endnoteIndex)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(endnote, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal endnote: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getAllEndnotes(ctx context.Context, docID string) (string, error) {
	endnotes, err := h.store.GetEndnotes(ctx, docID)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"endnote_count": len(endnotes),
		"endnotes":      endnotes,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal endnotes: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getQuotation(ctx context.Context, docID string, quotationIndex int) (string, error) {
	quotation, err := h.store.GetQuotation(ctx, docID, quotationIndex)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(quotation, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal quotation: %w", err)
	}

	return string(data), nil
}

func (h *PDFResourceHandler) getAllQuotations(ctx context.Context, docID string) (string, error) {
	quotations, err := h.store.GetQuotations(ctx, docID)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"quotation_count": len(quotations),
		"quotations":      quotations,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal quotations: %w", err)
	}

	return string(data), nil
}
