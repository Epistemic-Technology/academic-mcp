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
		if index >= 0 {
			content, err = h.getPage(ctx, docID, index)
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

	summary := map[string]interface{}{
		"document_id": docID,
		"metadata":    metadata,
		"page_count":  len(pages),
		"ref_count":   len(refs),
		"image_count": len(images),
		"table_count": len(tables),
		"available_resources": []string{
			fmt.Sprintf("pdf://%s/metadata", docID),
			fmt.Sprintf("pdf://%s/pages", docID),
			fmt.Sprintf("pdf://%s/references", docID),
			fmt.Sprintf("pdf://%s/images", docID),
			fmt.Sprintf("pdf://%s/tables", docID),
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

func (h *PDFResourceHandler) getAllPages(ctx context.Context, docID string) (string, error) {
	pages, err := h.store.GetPages(ctx, docID)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"page_count": len(pages),
		"pages":      pages,
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
