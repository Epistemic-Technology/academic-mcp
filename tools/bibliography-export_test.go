package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/Epistemic-Technology/academic-mcp/models"
)

func TestBibliographyExportToolHandler(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary in-memory SQLite store
	log := logger.NewNoOpLogger()
	store, err := storage.NewSQLiteStore(":memory:", log)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store some test documents
	testDocs := []struct {
		docID string
		item  *models.ParsedItem
	}{
		{
			docID: "test-doc-1",
			item: &models.ParsedItem{
				Metadata: models.ItemMetadata{
					Title:           "Machine Learning in Climate Science",
					Authors:         []string{"Smith, John", "Doe, Jane"},
					PublicationDate: "2020-05-15",
					Publication:     "Nature Climate Change",
					DOI:             "10.1038/s41558-020-0000-0",
					ItemType:        "article",
					Volume:          "10",
					Issue:           "5",
					Pages:           "123-130",
					Citekey:         "smithDoe2020",
				},
				Pages: []string{"Page 1 content"},
			},
		},
		{
			docID: "test-doc-2",
			item: &models.ParsedItem{
				Metadata: models.ItemMetadata{
					Title:           "Introduction to Algorithms",
					Authors:         []string{"Cormen, Thomas", "Leiserson, Charles", "Rivest, Ronald"},
					PublicationDate: "2009",
					Publisher:       "MIT Press",
					ItemType:        "book",
					ISBN:            "978-0262033848",
					Citekey:         "cormenEtAl2009",
				},
				Pages: []string{"Page 1 content"},
			},
		},
		{
			docID: "test-doc-3",
			item: &models.ParsedItem{
				Metadata: models.ItemMetadata{
					Title:           "Document Without Citekey",
					Authors:         []string{"Unknown, Author"},
					PublicationDate: "2023",
					ItemType:        "misc",
					// Note: No citekey
				},
				Pages: []string{"Page 1 content"},
			},
		},
	}

	for _, td := range testDocs {
		err := store.StoreParsedItem(ctx, td.docID, td.item, &models.SourceInfo{})
		if err != nil {
			t.Fatalf("Failed to store test document %s: %v", td.docID, err)
		}
	}

	t.Run("export specific documents", func(t *testing.T) {
		query := BibliographyExportQuery{
			DocumentIDs: []string{"test-doc-1", "test-doc-2"},
			Format:      "bibtex",
		}

		_, response, err := BibliographyExportToolHandler(ctx, nil, query, store, log)
		if err != nil {
			t.Fatalf("BibliographyExportToolHandler failed: %v", err)
		}

		// Verify response
		if response.Format != "bibtex" {
			t.Errorf("Expected format 'bibtex', got '%s'", response.Format)
		}

		if response.DocumentCount != 2 {
			t.Errorf("Expected 2 documents, got %d", response.DocumentCount)
		}

		if len(response.MissingCitekey) != 0 {
			t.Errorf("Expected no missing citekeys, got %v", response.MissingCitekey)
		}

		// Verify content contains expected entries
		if !strings.Contains(response.Content, "@article{smithDoe2020,") {
			t.Error("Expected content to contain article entry")
		}

		if !strings.Contains(response.Content, "@book{cormenEtAl2009,") {
			t.Error("Expected content to contain book entry")
		}

		if !strings.Contains(response.Content, "Machine Learning in Climate Science") {
			t.Error("Expected content to contain first document title")
		}

		if !strings.Contains(response.Content, "Introduction to Algorithms") {
			t.Error("Expected content to contain second document title")
		}
	})

	t.Run("export entire library", func(t *testing.T) {
		query := BibliographyExportQuery{
			Format: "bibtex",
		}

		_, response, err := BibliographyExportToolHandler(ctx, nil, query, store, log)
		if err != nil {
			t.Fatalf("BibliographyExportToolHandler failed: %v", err)
		}

		// Should export 2 documents (the 3rd has no citekey)
		if response.DocumentCount != 2 {
			t.Errorf("Expected 2 documents with citekeys, got %d", response.DocumentCount)
		}

		// Should report 1 missing citekey
		if len(response.MissingCitekey) != 1 {
			t.Errorf("Expected 1 missing citekey, got %d", len(response.MissingCitekey))
		}

		if len(response.MissingCitekey) > 0 && response.MissingCitekey[0] != "test-doc-3" {
			t.Errorf("Expected missing citekey for test-doc-3, got %s", response.MissingCitekey[0])
		}
	})

	t.Run("export with unsupported format", func(t *testing.T) {
		query := BibliographyExportQuery{
			DocumentIDs: []string{"test-doc-1"},
			Format:      "endnote",
		}

		_, _, err := BibliographyExportToolHandler(ctx, nil, query, store, log)
		if err == nil {
			t.Error("Expected error for unsupported format, got nil")
		}

		if !strings.Contains(err.Error(), "unsupported format") {
			t.Errorf("Expected 'unsupported format' error, got: %v", err)
		}
	})

	t.Run("export nonexistent document", func(t *testing.T) {
		query := BibliographyExportQuery{
			DocumentIDs: []string{"nonexistent-doc"},
			Format:      "bibtex",
		}

		_, _, err := BibliographyExportToolHandler(ctx, nil, query, store, log)
		if err == nil {
			t.Error("Expected error for nonexistent document, got nil")
		}
	})

	t.Run("default format is bibtex", func(t *testing.T) {
		query := BibliographyExportQuery{
			DocumentIDs: []string{"test-doc-1"},
			// Format not specified
		}

		_, response, err := BibliographyExportToolHandler(ctx, nil, query, store, log)
		if err != nil {
			t.Fatalf("BibliographyExportToolHandler failed: %v", err)
		}

		if response.Format != "bibtex" {
			t.Errorf("Expected default format 'bibtex', got '%s'", response.Format)
		}
	})
}
