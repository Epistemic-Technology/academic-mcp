package documents

import (
	"context"
	"fmt"
	"strings"

	"github.com/Epistemic-Technology/academic-mcp/models"
	"github.com/Epistemic-Technology/zotero/zotero"
)

// FetchZoteroMetadata retrieves metadata for a Zotero item (attachment or parent item).
// If the zoteroID is an attachment, it fetches the parent item's metadata.
// Returns nil if the item is not found or has no useful metadata.
func FetchZoteroMetadata(ctx context.Context, zoteroID string, apiKey string, libraryID string) (*models.ItemMetadata, error) {
	if zoteroID == "" || apiKey == "" || libraryID == "" {
		return nil, fmt.Errorf("zoteroID, apiKey, and libraryID are required")
	}

	client := zotero.NewClient(libraryID, zotero.LibraryTypeUser, zotero.WithAPIKey(apiKey))

	// Fetch the item
	item, err := client.Item(ctx, zoteroID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Zotero item %s: %w", zoteroID, err)
	}

	// If this is an attachment, fetch the parent item instead
	if item.Data.ItemType == "attachment" && item.Data.ParentItem != "" {
		parentItem, err := client.Item(ctx, item.Data.ParentItem, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch parent item %s: %w", item.Data.ParentItem, err)
		}
		item = parentItem
	}

	// Skip if still an attachment (orphaned attachment with no parent)
	if item.Data.ItemType == "attachment" {
		return nil, nil
	}

	// Convert Zotero item to our metadata format
	metadata := zoteroItemToMetadata(item)
	metadata.MetadataSource = "zotero"

	return metadata, nil
}

// zoteroItemToMetadata converts a Zotero Item to our ItemMetadata structure
func zoteroItemToMetadata(item *zotero.Item) *models.ItemMetadata {
	metadata := &models.ItemMetadata{
		Title:    item.Data.Title,
		ItemType: item.Data.ItemType,
		Abstract: item.Data.AbstractNote,
	}

	// Extract creator names (authors, editors, etc.)
	for _, creator := range item.Data.Creators {
		var name string
		if creator.Name != "" {
			name = creator.Name
		} else if creator.FirstName != "" || creator.LastName != "" {
			name = strings.TrimSpace(creator.FirstName + " " + creator.LastName)
		}
		if name != "" {
			metadata.Authors = append(metadata.Authors, name)
		}
	}

	// Extract type-specific fields from Extra map
	// The zotero library uses reflection to populate Extra with all additional fields
	if item.Data.Extra != nil {
		// Common bibliographic fields
		if val, ok := item.Data.Extra["date"].(string); ok {
			metadata.PublicationDate = val
		}
		if val, ok := item.Data.Extra["publicationTitle"].(string); ok {
			metadata.Publication = val
		}
		if val, ok := item.Data.Extra["DOI"].(string); ok {
			metadata.DOI = val
		}
		if val, ok := item.Data.Extra["publisher"].(string); ok {
			metadata.Publisher = val
		}
		if val, ok := item.Data.Extra["volume"].(string); ok {
			metadata.Volume = val
		}
		if val, ok := item.Data.Extra["issue"].(string); ok {
			metadata.Issue = val
		}
		if val, ok := item.Data.Extra["pages"].(string); ok {
			metadata.Pages = val
		}
		if val, ok := item.Data.Extra["ISSN"].(string); ok {
			metadata.ISSN = val
		}
		if val, ok := item.Data.Extra["ISBN"].(string); ok {
			metadata.ISBN = val
		}
		if val, ok := item.Data.Extra["url"].(string); ok {
			metadata.URL = val
		}
	}

	return metadata
}

// MergeMetadata merges external metadata with extracted metadata.
// External metadata takes priority for all fields.
// Falls back to extracted metadata when external field is empty.
func MergeMetadata(external *models.ItemMetadata, extracted *models.ItemMetadata) *models.ItemMetadata {
	if external == nil && extracted == nil {
		return &models.ItemMetadata{MetadataSource: "none"}
	}
	if external == nil {
		result := *extracted
		result.MetadataSource = "extracted"
		return &result
	}
	if extracted == nil {
		result := *external
		result.MetadataSource = "external"
		return &result
	}

	// Merge with external taking priority
	merged := &models.ItemMetadata{
		MetadataSource: "merged",
	}

	// Title: prefer external
	if external.Title != "" {
		merged.Title = external.Title
	} else {
		merged.Title = extracted.Title
	}

	// Authors: prefer external (LLM extraction can be unreliable)
	if len(external.Authors) > 0 {
		merged.Authors = external.Authors
	} else {
		merged.Authors = extracted.Authors
	}

	// Publication date: prefer external
	if external.PublicationDate != "" {
		merged.PublicationDate = external.PublicationDate
	} else {
		merged.PublicationDate = extracted.PublicationDate
	}

	// Publication/journal: prefer external
	if external.Publication != "" {
		merged.Publication = external.Publication
	} else {
		merged.Publication = extracted.Publication
	}

	// DOI: prefer external
	if external.DOI != "" {
		merged.DOI = external.DOI
	} else {
		merged.DOI = extracted.DOI
	}

	// Abstract: prefer external
	if external.Abstract != "" {
		merged.Abstract = external.Abstract
	} else {
		merged.Abstract = extracted.Abstract
	}

	// Additional fields (typically only from external sources)
	merged.ItemType = external.ItemType
	merged.Publisher = external.Publisher
	merged.Volume = external.Volume
	merged.Issue = external.Issue
	merged.Pages = external.Pages
	merged.ISSN = external.ISSN
	merged.ISBN = external.ISBN
	merged.URL = external.URL

	return merged
}
