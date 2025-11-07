package operations

import (
	"context"
	"os"
	"testing"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
)

// getZoteroCredentials retrieves Zotero credentials from environment.
// Skips the test if credentials are not available.
func getZoteroCredentials(t *testing.T) (apiKey, libraryID string) {
	apiKey = os.Getenv("ZOTERO_API_KEY")
	libraryID = os.Getenv("ZOTERO_LIBRARY_ID")

	if apiKey == "" || libraryID == "" {
		t.Skip("ZOTERO_API_KEY and ZOTERO_LIBRARY_ID not set, skipping integration test")
	}

	return apiKey, libraryID
}

func TestSearchZotero_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey, libraryID := getZoteroCredentials(t)
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	tests := []struct {
		name   string
		params ZoteroSearchParams
	}{
		{
			name: "Basic search with limit",
			params: ZoteroSearchParams{
				Limit: 5,
			},
		},
		{
			name: "Search with query",
			params: ZoteroSearchParams{
				Query: "climate",
				Limit: 3,
			},
		},
		{
			name: "Search with sort",
			params: ZoteroSearchParams{
				Limit: 5,
				Sort:  "title",
			},
		},
		{
			name: "Search with item type filter",
			params: ZoteroSearchParams{
				ItemTypes: []string{"book"},
				Limit:     5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := SearchZotero(ctx, apiKey, libraryID, tt.params, log)
			if err != nil {
				t.Fatalf("SearchZotero failed: %v", err)
			}

			// Results can be empty if library doesn't match criteria
			t.Logf("Found %d items", len(results))

			// Validate structure of returned items
			for i, item := range results {
				if item.Key == "" {
					t.Errorf("Item %d has empty Key", i)
				}
				// Title can be empty for some item types, so we just check it's initialized
				t.Logf("Item %d: %s (type: %s)", i, item.Title, item.ItemType)

				// Check attachments structure
				for j, att := range item.Attachments {
					if att.Key == "" {
						t.Errorf("Item %d attachment %d has empty Key", i, j)
					}
					t.Logf("  Attachment: %s (%s)", att.Filename, att.ContentType)
				}
			}
		})
	}
}

func TestSearchZotero_MissingCredentials(t *testing.T) {
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
			params := ZoteroSearchParams{
				Limit: 5,
			}

			_, err := SearchZotero(ctx, tt.apiKey, tt.libraryID, params, log)
			if err == nil {
				t.Fatal("Expected error but got none")
			}

			if err.Error() != tt.wantError {
				t.Errorf("Expected error %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}

func TestSearchZotero_DefaultParameters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey, libraryID := getZoteroCredentials(t)
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// Test with empty parameters - should use defaults
	params := ZoteroSearchParams{}

	results, err := SearchZotero(ctx, apiKey, libraryID, params, log)
	if err != nil {
		t.Fatalf("SearchZotero failed: %v", err)
	}

	t.Logf("Found %d items with default parameters", len(results))

	// With defaults, we should get up to 25 items
	if len(results) > 25 {
		t.Errorf("Expected max 25 items with default limit, got %d", len(results))
	}
}

func TestSearchZotero_ItemsWithAttachments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey, libraryID := getZoteroCredentials(t)
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// Search for items that are likely to have attachments
	params := ZoteroSearchParams{
		ItemTypes: []string{"journalArticle", "book"},
		Limit:     10,
	}

	results, err := SearchZotero(ctx, apiKey, libraryID, params, log)
	if err != nil {
		t.Fatalf("SearchZotero failed: %v", err)
	}

	t.Logf("Found %d items", len(results))

	// Count items with attachments
	itemsWithAttachments := 0
	totalAttachments := 0

	for _, item := range results {
		if len(item.Attachments) > 0 {
			itemsWithAttachments++
			totalAttachments += len(item.Attachments)

			// Verify attachment structure
			for _, att := range item.Attachments {
				if att.Key == "" {
					t.Errorf("Attachment has empty Key")
				}
				if att.Filename == "" {
					t.Errorf("Attachment has empty Filename")
				}
				// ContentType and LinkMode can be empty in some cases
			}
		}
	}

	t.Logf("Items with attachments: %d/%d", itemsWithAttachments, len(results))
	t.Logf("Total attachments: %d", totalAttachments)
}
