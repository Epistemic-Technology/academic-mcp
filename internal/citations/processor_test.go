package citations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/models"
)

func TestExtractCitekeys(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "single citation",
			content: "This is a fact [@smith2020].",
			want:    []string{"smith2020"},
		},
		{
			name:    "multiple citations in one bracket",
			content: "These are facts [@smith2020; @jones2021].",
			want:    []string{"smith2020", "jones2021"},
		},
		{
			name:    "in-text citation",
			content: "According to @smith2020, this is true.",
			want:    []string{"smith2020"},
		},
		{
			name:    "citation with prefix",
			content: "As shown in previous work [see @smith2020], this is known.",
			want:    []string{"smith2020"},
		},
		{
			name:    "citation with page number",
			content: "This is documented [@smith2020, p. 42].",
			want:    []string{"smith2020"},
		},
		{
			name:    "suppress author citation",
			content: "This was shown in 2020 [-@smith2020].",
			want:    []string{"smith2020"},
		},
		{
			name:    "no citations",
			content: "This is just plain text with no references.",
			want:    []string{},
		},
		{
			name:    "multiple separate citations",
			content: "First [@smith2020]. Second [@jones2021]. Third [@brown2019].",
			want:    []string{"smith2020", "jones2021", "brown2019"},
		},
		{
			name:    "citation with special characters in key",
			content: "Reference [@smith-jones_2020:v1].",
			want:    []string{"smith-jones_2020:v1"},
		},
		{
			name:    "duplicate citations",
			content: "First [@smith2020]. Later again [@smith2020].",
			want:    []string{"smith2020"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractCitekeys(tt.content)
			if err != nil {
				t.Errorf("ExtractCitekeys() error = %v", err)
				return
			}

			// Convert slices to maps for comparison (order doesn't matter)
			gotMap := make(map[string]bool)
			for _, k := range got {
				gotMap[k] = true
			}
			wantMap := make(map[string]bool)
			for _, k := range tt.want {
				wantMap[k] = true
			}

			if len(gotMap) != len(wantMap) {
				t.Errorf("ExtractCitekeys() = %v, want %v", got, tt.want)
				return
			}

			for k := range wantMap {
				if !gotMap[k] {
					t.Errorf("ExtractCitekeys() missing citekey %v, got %v", k, got)
				}
			}
		})
	}
}

func TestGetOutputExtension(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{"markdown", "markdown", "md"},
		{"md", "md", "md"},
		{"html", "html", "html"},
		{"docx", "docx", "docx"},
		{"pdf", "pdf", "pdf"},
		{"uppercase", "HTML", "html"},
		{"unknown defaults to md", "latex", "md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getOutputExtension(tt.format)
			if got != tt.want {
				t.Errorf("getOutputExtension(%q) = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}

// TestProcessCitations_Integration tests the full citation processing pipeline
// This test is marked with -short flag to skip during regular development testing
// because it requires pandoc to be installed and makes actual file system calls
func TestProcessCitations_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test that requires pandoc")
	}

	// Create mock store with test documents
	store := &mockStore{
		documents: map[string]*models.ItemMetadata{
			"doc1": {
				Title:           "Test Paper",
				Authors:         []string{"Smith, John"},
				PublicationDate: "2020",
				Publication:     "Test Journal",
				ItemType:        "article",
				Citekey:         "smith2020",
			},
		},
		citekeyToDocID: map[string]string{
			"smith2020": "doc1",
		},
	}

	log, err := logger.NewLogger(logger.LogConfig{
		Output: "stderr",
		Level:  "info",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	ctx := context.Background()

	content := `# Test Document

This is a test citation [@smith2020].

## References`

	// Test markdown output
	processed, warnings, err := ProcessCitations(ctx, content, store, "", "markdown", log)
	if err != nil {
		t.Errorf("ProcessCitations() error = %v", err)
		return
	}

	if len(warnings) > 0 {
		t.Errorf("ProcessCitations() unexpected warnings: %v", warnings)
	}

	// Check that the output contains the formatted citation
	if !strings.Contains(processed, "Smith") {
		t.Errorf("ProcessCitations() output doesn't contain expected author name, got: %v", processed)
	}

	// Check that bibliography section was added
	if !strings.Contains(strings.ToLower(processed), "reference") {
		t.Errorf("ProcessCitations() output doesn't contain bibliography section, got: %v", processed)
	}
}

func TestProcessCitations_MissingCitekeys(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test that requires pandoc")
	}

	// Create mock store with only one document
	store := &mockStore{
		documents: map[string]*models.ItemMetadata{
			"doc1": {
				Title:           "Test Paper",
				Authors:         []string{"Smith, John"},
				PublicationDate: "2020",
				Citekey:         "smith2020",
			},
		},
		citekeyToDocID: map[string]string{
			"smith2020": "doc1",
		},
	}

	log, err := logger.NewLogger(logger.LogConfig{
		Output: "stderr",
		Level:  "info",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	ctx := context.Background()

	content := `# Test Document

This cites an existing document [@smith2020] and a missing one [@jones2021].`

	processed, warnings, err := ProcessCitations(ctx, content, store, "", "markdown", log)
	if err != nil {
		t.Errorf("ProcessCitations() error = %v", err)
		return
	}

	// Should have a warning about the missing citekey
	if len(warnings) != 1 {
		t.Errorf("ProcessCitations() expected 1 warning, got %d: %v", len(warnings), warnings)
	}

	if !strings.Contains(warnings[0], "jones2021") {
		t.Errorf("ProcessCitations() warning doesn't mention missing citekey jones2021: %v", warnings[0])
	}

	// Should still process successfully with the available citation
	if processed == "" {
		t.Errorf("ProcessCitations() returned empty output")
	}
}

func TestProcessCitations_NoCitations(t *testing.T) {
	store := &mockStore{
		documents:      map[string]*models.ItemMetadata{},
		citekeyToDocID: map[string]string{},
	}

	log, err := logger.NewLogger(logger.LogConfig{
		Output: "stderr",
		Level:  "info",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	ctx := context.Background()

	content := `# Test Document

This has no citations at all.`

	processed, warnings, err := ProcessCitations(ctx, content, store, "", "markdown", log)
	if err != nil {
		t.Errorf("ProcessCitations() error = %v", err)
		return
	}

	// Should return original content unchanged
	if processed != content {
		t.Errorf("ProcessCitations() with no citations should return original content")
	}

	if len(warnings) > 0 {
		t.Errorf("ProcessCitations() unexpected warnings: %v", warnings)
	}
}

// mockStore implements storage.Store interface for testing
type mockStore struct {
	documents      map[string]*models.ItemMetadata
	citekeyToDocID map[string]string
}

func (m *mockStore) StoreParsedItem(ctx context.Context, docID string, item *models.ParsedItem, sourceInfo *models.SourceInfo) error {
	return nil
}

func (m *mockStore) GetMetadata(ctx context.Context, docID string) (*models.ItemMetadata, error) {
	if meta, ok := m.documents[docID]; ok {
		return meta, nil
	}
	return nil, fmt.Errorf("document not found: %s", docID)
}

func (m *mockStore) GetPage(ctx context.Context, docID string, pageNum int) (string, error) {
	return "", nil
}

func (m *mockStore) GetPageBySourceNumber(ctx context.Context, docID string, sourcePageNum string) (string, error) {
	return "", nil
}

func (m *mockStore) GetPages(ctx context.Context, docID string) ([]string, error) {
	return nil, nil
}

func (m *mockStore) GetPageMapping(ctx context.Context, docID string) (map[string]int, error) {
	return nil, nil
}

func (m *mockStore) GetReferences(ctx context.Context, docID string) ([]models.Reference, error) {
	return nil, nil
}

func (m *mockStore) GetReference(ctx context.Context, docID string, refIndex int) (*models.Reference, error) {
	return nil, nil
}

func (m *mockStore) GetImages(ctx context.Context, docID string) ([]models.Image, error) {
	return nil, nil
}

func (m *mockStore) GetImage(ctx context.Context, docID string, imageIndex int) (*models.Image, error) {
	return nil, nil
}

func (m *mockStore) GetTables(ctx context.Context, docID string) ([]models.Table, error) {
	return nil, nil
}

func (m *mockStore) GetTable(ctx context.Context, docID string, tableIndex int) (*models.Table, error) {
	return nil, nil
}

func (m *mockStore) GetFootnotes(ctx context.Context, docID string) ([]models.Footnote, error) {
	return nil, nil
}

func (m *mockStore) GetFootnote(ctx context.Context, docID string, footnoteIndex int) (*models.Footnote, error) {
	return nil, nil
}

func (m *mockStore) GetEndnotes(ctx context.Context, docID string) ([]models.Endnote, error) {
	return nil, nil
}

func (m *mockStore) GetEndnote(ctx context.Context, docID string, endnoteIndex int) (*models.Endnote, error) {
	return nil, nil
}

func (m *mockStore) GetQuotations(ctx context.Context, docID string) ([]models.Quotation, error) {
	return nil, nil
}

func (m *mockStore) GetQuotation(ctx context.Context, docID string, quotationIndex int) (*models.Quotation, error) {
	return nil, nil
}

func (m *mockStore) ListDocuments(ctx context.Context) ([]models.DocumentInfo, error) {
	return nil, nil
}

func (m *mockStore) DeleteDocument(ctx context.Context, docID string) error {
	return nil
}

func (m *mockStore) DocumentExists(ctx context.Context, docID string) (bool, error) {
	_, ok := m.documents[docID]
	return ok, nil
}

func (m *mockStore) GetParsedItem(ctx context.Context, docID string) (*models.ParsedItem, error) {
	return nil, nil
}

func (m *mockStore) GetCitekeyMap(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)
	for docID, meta := range m.documents {
		if meta.Citekey != "" {
			result[docID] = meta.Citekey
		}
	}
	return result, nil
}

func (m *mockStore) GetDocumentByCitekey(ctx context.Context, citekey string) (string, error) {
	if docID, ok := m.citekeyToDocID[citekey]; ok {
		return docID, nil
	}
	return "", fmt.Errorf("citekey not found: %s", citekey)
}

func (m *mockStore) Close() error {
	return nil
}
