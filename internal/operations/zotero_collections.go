package operations

import (
	"context"
	"fmt"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/zotero/zotero"
)

// ListCollectionsParams contains parameters for listing Zotero collections.
type ListCollectionsParams struct {
	TopLevelOnly     bool   // List only top-level collections (no parent)
	ParentCollection string // Filter by parent collection key (for subcollections)
	Limit            int    // Max results (default 100)
	Sort             string // Sort field (default "title")
}

// CollectionResult represents a Zotero collection with basic information.
type CollectionResult struct {
	Key              string // Collection key (unique identifier)
	Name             string // Collection name
	ParentCollection string // Parent collection key (empty if top-level)
}

// ListZoteroCollections retrieves collections from a Zotero library with the given parameters.
// This function encapsulates the common logic for listing collections and can be reused
// across multiple tools.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - apiKey: Zotero API key for authentication
//   - libraryID: Zotero library ID (user or group)
//   - params: Collection listing parameters
//   - log: Logger for recording operations
//
// Returns:
//   - results: Array of collections with metadata
//   - error: Any error encountered during the operation
func ListZoteroCollections(ctx context.Context, apiKey, libraryID string, params ListCollectionsParams, log logger.Logger) ([]CollectionResult, error) {
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
		Limit: params.Limit,
		Sort:  params.Sort,
	}

	// Set defaults
	if queryParams.Limit == 0 {
		queryParams.Limit = 100
	}
	if queryParams.Sort == "" {
		queryParams.Sort = "title"
	}

	// Retrieve collections based on parameters
	var collections []zotero.Collection
	var err error

	if params.ParentCollection != "" {
		// Get subcollections of a specific collection
		log.Info("Retrieving subcollections for collection: %s", params.ParentCollection)
		collections, err = client.CollectionsSub(ctx, params.ParentCollection, queryParams)
	} else if params.TopLevelOnly {
		// Get only top-level collections
		log.Info("Retrieving top-level collections")
		collections, err = client.CollectionsTop(ctx, queryParams)
	} else {
		// Get all collections
		log.Info("Retrieving all collections")
		collections, err = client.Collections(ctx, queryParams)
	}

	if err != nil {
		log.Error("Failed to retrieve Zotero collections: %v", err)
		return nil, fmt.Errorf("failed to retrieve Zotero collections: %w", err)
	}

	log.Info("Found %d collections in Zotero library", len(collections))

	// Process collections into result format
	results := make([]CollectionResult, 0, len(collections))
	for _, collection := range collections {
		result := CollectionResult{
			Key:              collection.Data.Key,
			Name:             collection.Data.Name,
			ParentCollection: collection.Data.ParentCollection.String(),
		}
		results = append(results, result)
	}

	log.Info("Returning %d processed collections", len(results))

	return results, nil
}
