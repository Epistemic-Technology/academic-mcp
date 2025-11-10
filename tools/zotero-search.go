package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/operations"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
)

type ZoteroSearchQuery struct {
	Query      string   `json:"query,omitempty"`      // Quick search text (searches title, creator, year)
	Tags       []string `json:"tags,omitempty"`       // Filter by tags
	ItemTypes  []string `json:"item_types,omitempty"` // Filter by type (e.g., "book", "article", "-attachment")
	Collection string   `json:"collection,omitempty"` // Filter by collection key (optional)
	Limit      int      `json:"limit,omitempty"`      // Max results (default 25)
	Sort       string   `json:"sort,omitempty"`       // Sort field (default "dateModified")
}

type ZoteroSearchResponse struct {
	Items []ZoteroItemResult `json:"items"`
	Count int                `json:"count"`
}

type ZoteroItemResult struct {
	Key         string           `json:"key"`
	Title       string           `json:"title"`
	Creators    []string         `json:"creators,omitempty"`
	ItemType    string           `json:"item_type"`
	Date        string           `json:"date,omitempty"`
	Attachments []AttachmentInfo `json:"attachments,omitempty"`
	Citekey     string           `json:"citekey,omitempty"` // Citekey if document has been parsed
}

type AttachmentInfo struct {
	Key         string `json:"key"` // Use this as zotero_id in document-parse
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"` // MIME type (e.g., "application/pdf")
	LinkMode    string `json:"link_mode"`    // imported_file, imported_url, linked_file, linked_url
}

func ZoteroSearchTool() *mcp.Tool {
	inputschema, err := jsonschema.For[ZoteroSearchQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "zotero-search",
		Description: "Search for items in a Zotero library and retrieve their metadata and attachment information. Returns bibliographic items with their associated file attachments (PDFs, etc.). Use the attachment keys with document-parse to analyze specific files.",
		InputSchema: inputschema,
	}
}

func ZoteroSearchToolHandler(ctx context.Context, req *mcp.CallToolRequest, query ZoteroSearchQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *ZoteroSearchResponse, error) {
	log.Info("zotero-search tool called")

	// Get Zotero credentials from environment
	zoteroAPIKey := os.Getenv("ZOTERO_API_KEY")
	if zoteroAPIKey == "" {
		return nil, nil, fmt.Errorf("ZOTERO_API_KEY environment variable not set")
	}

	libraryID := os.Getenv("ZOTERO_LIBRARY_ID")
	if libraryID == "" {
		return nil, nil, fmt.Errorf("ZOTERO_LIBRARY_ID environment variable not set")
	}

	// Convert tool query parameters to operations parameters
	searchParams := operations.ZoteroSearchParams{
		Query:      query.Query,
		Tags:       query.Tags,
		ItemTypes:  query.ItemTypes,
		Collection: query.Collection,
		Limit:      query.Limit,
		Sort:       query.Sort,
	}

	// Execute search using internal operation
	items, err := operations.SearchZotero(ctx, zoteroAPIKey, libraryID, searchParams, log)
	if err != nil {
		return nil, nil, err
	}

	// Get existing citekeys for all documents
	citekeyMap, err := store.GetCitekeyMap(ctx)
	if err != nil {
		log.Error("Failed to retrieve citekey map: %v", err)
		// Don't fail the whole request, just skip citekey enrichment
		citekeyMap = make(map[string]string)
	}

	// Build a reverse map from zotero_id to citekey
	zoteroToCitekey := make(map[string]string)
	for docID, citekey := range citekeyMap {
		// Extract zotero ID from document ID (format: "zotero_XXXXX")
		if len(docID) > 7 && docID[:7] == "zotero_" {
			zoteroID := docID[7:]
			zoteroToCitekey[zoteroID] = citekey
		}
	}

	// Convert internal results to tool response format
	results := make([]ZoteroItemResult, len(items))
	for i, item := range items {
		results[i] = ZoteroItemResult{
			Key:      item.Key,
			Title:    item.Title,
			Creators: item.Creators,
			ItemType: item.ItemType,
			Date:     item.Date,
		}
		// Convert attachments and check for citekeys
		for _, att := range item.Attachments {
			results[i].Attachments = append(results[i].Attachments, AttachmentInfo{
				Key:         att.Key,
				Filename:    att.Filename,
				ContentType: att.ContentType,
				LinkMode:    att.LinkMode,
			})
			// If this attachment has been parsed, add citekey to the result
			if citekey, found := zoteroToCitekey[att.Key]; found {
				results[i].Citekey = citekey
			}
		}
	}

	response := &ZoteroSearchResponse{
		Items: results,
		Count: len(results),
	}

	return nil, response, nil
}
