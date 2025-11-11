package operations

import (
	"context"
	"fmt"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/zotero/zotero"
)

// ZoteroSearchParams contains parameters for searching a Zotero library.
type ZoteroSearchParams struct {
	Query      string   // Quick search text (searches title, creator, year)
	Tags       []string // Filter by tags
	ItemTypes  []string // Filter by type (e.g., "book", "article", "-attachment")
	Collection string   // Filter by collection key (optional)
	Limit      int      // Max results (default 25)
	Sort       string   // Sort field (default "dateModified")
}

// ZoteroItemResult represents a Zotero item with its attachments.
type ZoteroItemResult struct {
	Key         string
	Title       string
	Creators    []string
	ItemType    string
	Date        string
	Attachments []AttachmentInfo
}

// AttachmentInfo contains information about a file attached to a Zotero item.
type AttachmentInfo struct {
	Key         string // Use this as zotero_id in document-parse
	Filename    string
	ContentType string // MIME type (e.g., "application/pdf")
	LinkMode    string // imported_file, imported_url, linked_file, linked_url
}

// SearchZotero searches a Zotero library with the given parameters and returns
// processed results with attachments. This function encapsulates the common
// logic for searching Zotero and can be reused across multiple tools.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - apiKey: Zotero API key for authentication
//   - libraryID: Zotero library ID (user or group)
//   - params: Search parameters (query, tags, item types, limit, sort)
//   - log: Logger for recording operations
//
// Returns:
//   - results: Array of processed items with metadata and attachments
//   - error: Any error encountered during the search
func SearchZotero(ctx context.Context, apiKey, libraryID string, params ZoteroSearchParams, log logger.Logger) ([]ZoteroItemResult, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Zotero API key is required")
	}
	if libraryID == "" {
		return nil, fmt.Errorf("Zotero library ID is required")
	}

	// Create Zotero client
	client := zotero.NewClient(libraryID, zotero.LibraryTypeUser, zotero.WithAPIKey(apiKey))

	// Set up query parameters
	queryParams := &zotero.QueryParams{
		Q:        params.Query,
		QMode:    "titleCreatorYear", // Search in title, creator, and year fields
		Tag:      params.Tags,
		ItemType: params.ItemTypes,
		Limit:    params.Limit,
		Sort:     params.Sort,
	}

	// Set defaults
	if queryParams.Limit == 0 {
		queryParams.Limit = 25
	}
	if queryParams.Sort == "" {
		queryParams.Sort = "dateModified"
	}
	if len(queryParams.ItemType) == 0 {
		queryParams.ItemType = []string{"-attachment"}
	}

	// Search for items (either in a specific collection or the entire library)
	var items []zotero.Item
	var err error
	if params.Collection != "" {
		// If we're retriving items in a collection, we want to retrieve the max number of items (100)
		if queryParams.Limit == 0 {
			queryParams.Limit = 100
		}
		items, err = client.CollectionItems(ctx, params.Collection, queryParams)
		if err != nil {
			log.Error("Failed to search collection %s: %v", params.Collection, err)
			return nil, fmt.Errorf("failed to search collection %s: %w", params.Collection, err)
		}
	} else {
		// Search the entire library
		items, err = client.Items(ctx, queryParams)
		if err != nil {
			log.Error("Failed to search Zotero library: %v", err)
			return nil, fmt.Errorf("failed to search Zotero library: %w", err)
		}
	}

	log.Info("Found %d items in Zotero library", len(items))

	// Process each item and retrieve attachments
	results := make([]ZoteroItemResult, 0, len(items))
	for _, item := range items {
		// Skip attachment items themselves (we want parent items with attachments)
		if item.Data.ItemType == "attachment" {
			continue
		}

		result := ZoteroItemResult{
			Key:      item.Key,
			Title:    item.Data.Title,
			ItemType: item.Data.ItemType,
			Date:     item.Data.DateAdded,
		}

		// Extract creator names
		for _, creator := range item.Data.Creators {
			if creator.Name != "" {
				result.Creators = append(result.Creators, creator.Name)
			} else if creator.FirstName != "" || creator.LastName != "" {
				name := creator.FirstName
				if name != "" && creator.LastName != "" {
					name += " "
				}
				name += creator.LastName
				result.Creators = append(result.Creators, name)
			}
		}

		// Retrieve attachments for this item
		children, err := client.Children(ctx, item.Key, nil)
		if err != nil {
			log.Error("Failed to retrieve children for item %s: %v", item.Key, err)
			// Continue processing other items
			continue
		}

		// Filter for attachment-type children
		for _, child := range children {
			if child.Data.ItemType == "attachment" {
				attachment := AttachmentInfo{
					Key:         child.Key,
					Filename:    child.Data.Filename,
					ContentType: child.Data.ContentType,
					LinkMode:    child.Data.LinkMode,
				}
				result.Attachments = append(result.Attachments, attachment)
			}
		}

		results = append(results, result)
	}

	log.Info("Returning %d processed items", len(results))

	return results, nil
}
