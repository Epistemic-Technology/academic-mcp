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

type ZoteroCollectionsQuery struct {
	TopLevelOnly     bool   `json:"top_level_only,omitempty"`    // List only top-level collections (no parent)
	ParentCollection string `json:"parent_collection,omitempty"` // Filter by parent collection key (for subcollections)
	Limit            int    `json:"limit,omitempty"`             // Max results (default 100)
	Sort             string `json:"sort,omitempty"`              // Sort field (default "title")
}

type ZoteroCollectionsResponse struct {
	Collections []CollectionResult `json:"collections"`
	Count       int                `json:"count"`
}

type CollectionResult struct {
	Key              string `json:"key"`                         // Collection key (unique identifier)
	Name             string `json:"name"`                        // Collection name
	ParentCollection string `json:"parent_collection,omitempty"` // Parent collection key (empty if top-level)
}

func ZoteroCollectionsTool() *mcp.Tool {
	inputschema, err := jsonschema.For[ZoteroCollectionsQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "zotero-collections",
		Description: "List and search collections in a Zotero library. Returns collection names, keys, and hierarchy information. Use this to browse your library structure and find collection keys for filtering items.",
		InputSchema: inputschema,
	}
}

func ZoteroCollectionsToolHandler(ctx context.Context, req *mcp.CallToolRequest, query ZoteroCollectionsQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *ZoteroCollectionsResponse, error) {
	log.Info("zotero-collections tool called")

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
	listParams := operations.ListCollectionsParams{
		TopLevelOnly:     query.TopLevelOnly,
		ParentCollection: query.ParentCollection,
		Limit:            query.Limit,
		Sort:             query.Sort,
	}

	// Execute collection listing using internal operation
	collections, err := operations.ListZoteroCollections(ctx, zoteroAPIKey, libraryID, listParams, log)
	if err != nil {
		return nil, nil, err
	}

	// Convert internal results to tool response format
	results := make([]CollectionResult, len(collections))
	for i, collection := range collections {
		results[i] = CollectionResult{
			Key:              collection.Key,
			Name:             collection.Name,
			ParentCollection: collection.ParentCollection,
		}
	}

	response := &ZoteroCollectionsResponse{
		Collections: results,
		Count:       len(results),
	}

	return nil, response, nil
}
