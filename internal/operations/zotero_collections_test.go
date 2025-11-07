package operations

import (
	"context"
	"testing"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
)

func TestListZoteroCollections_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey, libraryID := getZoteroCredentials(t)
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	tests := []struct {
		name   string
		params ListCollectionsParams
	}{
		{
			name: "List all collections",
			params: ListCollectionsParams{
				Limit: 100,
			},
		},
		{
			name: "List top-level collections only",
			params: ListCollectionsParams{
				TopLevelOnly: true,
				Limit:        50,
			},
		},
		{
			name: "List with custom sort",
			params: ListCollectionsParams{
				Sort:  "title",
				Limit: 50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := ListZoteroCollections(ctx, apiKey, libraryID, tt.params, log)
			if err != nil {
				t.Fatalf("ListZoteroCollections failed: %v", err)
			}

			// Results can be empty if library has no collections
			t.Logf("Found %d collections", len(results))

			// Validate structure of returned collections
			for i, collection := range results {
				if collection.Key == "" {
					t.Errorf("Collection %d has empty Key", i)
				}
				if collection.Name == "" {
					t.Errorf("Collection %d has empty Name", i)
				}

				parentInfo := "top-level"
				if collection.ParentCollection != "" {
					parentInfo = "parent: " + collection.ParentCollection
				}
				t.Logf("Collection %d: %s (%s)", i, collection.Name, parentInfo)
			}

			// If top-level only, verify no parent collections
			if tt.params.TopLevelOnly {
				for i, collection := range results {
					if collection.ParentCollection != "" {
						t.Errorf("Collection %d has parent %s but TopLevelOnly was set", i, collection.ParentCollection)
					}
				}
			}
		})
	}
}

func TestListZoteroCollections_Subcollections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey, libraryID := getZoteroCredentials(t)
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// First, get all collections to find one with subcollections
	allParams := ListCollectionsParams{
		Limit: 100,
	}

	allCollections, err := ListZoteroCollections(ctx, apiKey, libraryID, allParams, log)
	if err != nil {
		t.Fatalf("ListZoteroCollections failed: %v", err)
	}

	if len(allCollections) == 0 {
		t.Skip("No collections in library, skipping subcollection test")
	}

	// Find a collection that has subcollections (appears as parent)
	var parentKey string
	parentCounts := make(map[string]int)

	for _, collection := range allCollections {
		if collection.ParentCollection != "" {
			parentCounts[collection.ParentCollection]++
		}
	}

	// Find a parent with at least one subcollection
	for key, count := range parentCounts {
		if count > 0 {
			parentKey = key
			t.Logf("Found parent collection %s with %d subcollections", key, count)
			break
		}
	}

	if parentKey == "" {
		t.Skip("No parent collections with subcollections found, skipping test")
	}

	// Test retrieving subcollections
	subParams := ListCollectionsParams{
		ParentCollection: parentKey,
		Limit:            50,
	}

	subCollections, err := ListZoteroCollections(ctx, apiKey, libraryID, subParams, log)
	if err != nil {
		t.Fatalf("ListZoteroCollections for subcollections failed: %v", err)
	}

	t.Logf("Found %d subcollections for parent %s", len(subCollections), parentKey)

	// Verify all returned collections have the correct parent
	for i, collection := range subCollections {
		if collection.ParentCollection != parentKey {
			t.Errorf("Subcollection %d has parent %s, expected %s", i, collection.ParentCollection, parentKey)
		}
	}
}

func TestListZoteroCollections_MissingCredentials(t *testing.T) {
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	tests := []struct {
		name      string
		apiKey    string
		libraryID string
		wantError string
	}{
		{
			name:      "Missing API key",
			apiKey:    "",
			libraryID: "12345",
			wantError: "Zotero API key is required",
		},
		{
			name:      "Missing library ID",
			apiKey:    "test-key",
			libraryID: "",
			wantError: "Zotero library ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := ListCollectionsParams{
				Limit: 50,
			}

			_, err := ListZoteroCollections(ctx, tt.apiKey, tt.libraryID, params, log)
			if err == nil {
				t.Fatal("Expected error but got none")
			}

			if err.Error() != tt.wantError {
				t.Errorf("Expected error %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}

func TestListZoteroCollections_DefaultParameters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey, libraryID := getZoteroCredentials(t)
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// Test with empty parameters - should use defaults
	params := ListCollectionsParams{}

	results, err := ListZoteroCollections(ctx, apiKey, libraryID, params, log)
	if err != nil {
		t.Fatalf("ListZoteroCollections failed: %v", err)
	}

	t.Logf("Found %d collections with default parameters", len(results))

	// With defaults, we should get up to 100 collections
	if len(results) > 100 {
		t.Errorf("Expected max 100 collections with default limit, got %d", len(results))
	}
}

func TestListZoteroCollections_HierarchyStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey, libraryID := getZoteroCredentials(t)
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// Get all collections to analyze hierarchy
	params := ListCollectionsParams{
		Limit: 100,
	}

	results, err := ListZoteroCollections(ctx, apiKey, libraryID, params, log)
	if err != nil {
		t.Fatalf("ListZoteroCollections failed: %v", err)
	}

	if len(results) == 0 {
		t.Skip("No collections in library, skipping hierarchy test")
	}

	// Count top-level and nested collections
	topLevel := 0
	nested := 0
	parentKeys := make(map[string]bool)

	for _, collection := range results {
		if collection.ParentCollection == "" {
			topLevel++
		} else {
			nested++
			parentKeys[collection.ParentCollection] = true
		}
	}

	t.Logf("Collection hierarchy:")
	t.Logf("  Total collections: %d", len(results))
	t.Logf("  Top-level collections: %d", topLevel)
	t.Logf("  Nested collections: %d", nested)
	t.Logf("  Unique parent collections: %d", len(parentKeys))

	// Verify that all parent keys reference actual collections
	collectionKeys := make(map[string]bool)
	for _, collection := range results {
		collectionKeys[collection.Key] = true
	}

	for parentKey := range parentKeys {
		if !collectionKeys[parentKey] {
			t.Logf("Warning: Parent key %s not found in collection list (may be due to limit)", parentKey)
		}
	}
}
