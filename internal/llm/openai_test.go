package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Epistemic-Technology/academic-mcp/internal/documents"
	"github.com/Epistemic-Technology/academic-mcp/models"
)

func getAPIKey(t *testing.T) string {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}
	return apiKey
}

func loadSamplePDFs(t *testing.T) []string {
	samplesDir := filepath.Join("..", "samples")
	files, err := filepath.Glob(filepath.Join(samplesDir, "*.pdf"))
	if err != nil {
		t.Fatalf("Failed to list sample PDFs: %v", err)
	}

	if len(files) == 0 {
		t.Skip("No sample PDFs found in samples directory")
	}

	return files
}

func TestParsePDFPage_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey := getAPIKey(t)
	ctx := context.Background()
	sampleFiles := loadSamplePDFs(t)

	for _, filePath := range sampleFiles {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			// Read the sample PDF
			pdfBytes, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read PDF file %s: %v", filePath, err)
			}

			// Split the PDF into pages
			pages, err := documents.SplitPdf(models.DocumentData{
				Data: pdfBytes,
				Type: "pdf",
			})
			if err != nil {
				t.Fatalf("Failed to split PDF: %v", err)
			}

			if len(pages) == 0 {
				t.Skip("PDF has no pages")
			}

			// Test parsing the first page
			firstPage := pages[0]
			parsedPage, err := ParsePDFPage(ctx, apiKey, &firstPage)
			if err != nil {
				t.Fatalf("ParsePDFPage failed: %v", err)
			}

			// Validate the structure
			if parsedPage == nil {
				t.Fatal("ParsePDFPage returned nil result")
			}

			// Content should be present (even if empty string is valid)
			t.Logf("Content length: %d characters", len(parsedPage.Content))

			// References, Images, and Tables should be initialized (can be empty arrays)
			if parsedPage.References == nil {
				t.Error("References should be initialized (not nil)")
			}
			if parsedPage.Images == nil {
				t.Error("Images should be initialized (not nil)")
			}
			if parsedPage.Tables == nil {
				t.Error("Tables should be initialized (not nil)")
			}

			// Log what we found
			t.Logf("Found %d references, %d images, %d tables",
				len(parsedPage.References), len(parsedPage.Images), len(parsedPage.Tables))

			// Validate references structure
			for i, ref := range parsedPage.References {
				if ref.ReferenceText == "" {
					t.Errorf("Reference %d has empty ReferenceText", i)
				}
				t.Logf("Reference %d: %s (DOI: %s)", i, ref.ReferenceText, ref.DOI)
			}

			// Validate images structure
			for i, img := range parsedPage.Images {
				if img.ImageURL == "" {
					t.Errorf("Image %d has empty ImageURL", i)
				}
				if img.Caption == "" {
					t.Errorf("Image %d has empty Caption", i)
				}
				t.Logf("Image %d: %s", i, img.Caption)
			}

			// Validate tables structure
			for i, tbl := range parsedPage.Tables {
				if tbl.TableID == "" {
					t.Errorf("Table %d has empty TableID", i)
				}
				if tbl.TableTitle == "" {
					t.Errorf("Table %d has empty TableTitle", i)
				}
				if tbl.TableData == "" {
					t.Errorf("Table %d has empty TableData", i)
				}
				t.Logf("Table %d: %s", i, tbl.TableTitle)
			}
		})
	}
}

func TestParsePDFPage_InvalidAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	sampleFiles := loadSamplePDFs(t)

	if len(sampleFiles) == 0 {
		t.Skip("No sample PDFs available")
	}

	// Read first sample PDF
	pdfBytes, err := os.ReadFile(sampleFiles[0])
	if err != nil {
		t.Fatalf("Failed to read PDF file: %v", err)
	}

	// Split the PDF into pages
	pages, err := documents.SplitPdf(models.DocumentData{
		Data: pdfBytes,
		Type: "pdf",
	})
	if err != nil {
		t.Fatalf("Failed to split PDF: %v", err)
	}

	if len(pages) == 0 {
		t.Skip("PDF has no pages")
	}

	// Test with invalid API key
	invalidAPIKey := "sk-invalid-key-12345"
	firstPage := pages[0]
	_, err = ParsePDFPage(ctx, invalidAPIKey, &firstPage)
	if err == nil {
		t.Error("Expected error with invalid API key, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestParsePDFPage_EmptyPage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey := getAPIKey(t)
	ctx := context.Background()

	emptyPage := models.DocumentPageData([]byte{})
	_, err := ParsePDFPage(ctx, apiKey, &emptyPage)
	if err == nil {
		t.Error("Expected error with empty page data, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestParsePDF_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey := getAPIKey(t)
	ctx := context.Background()
	sampleFiles := loadSamplePDFs(t)

	for _, filePath := range sampleFiles {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			// Read the sample PDF
			pdfBytes, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read PDF file %s: %v", filePath, err)
			}

			// Parse the entire PDF
			parsedItem, err := ParsePDF(ctx, apiKey, models.DocumentData{
				Data: pdfBytes,
				Type: "pdf",
			})
			if err != nil {
				t.Fatalf("ParsePDF failed: %v", err)
			}

			// Validate the structure
			if parsedItem == nil {
				t.Fatal("ParsePDF returned nil result")
			}

			// Pages should be present
			if len(parsedItem.Pages) == 0 {
				t.Error("Expected at least one page")
			}

			t.Logf("Parsed %d pages", len(parsedItem.Pages))

			// All arrays should be initialized
			if parsedItem.References == nil {
				t.Error("References should be initialized (not nil)")
			}
			if parsedItem.Images == nil {
				t.Error("Images should be initialized (not nil)")
			}
			if parsedItem.Tables == nil {
				t.Error("Tables should be initialized (not nil)")
			}

			// Log aggregate statistics
			t.Logf("Total content: %d pages", len(parsedItem.Pages))
			t.Logf("Total references: %d", len(parsedItem.References))
			t.Logf("Total images: %d", len(parsedItem.Images))
			t.Logf("Total tables: %d", len(parsedItem.Tables))

			// Validate that pages have content
			for i, pageContent := range parsedItem.Pages {
				if len(pageContent) == 0 {
					t.Logf("Warning: Page %d has no content", i+1)
				}
			}

			// Validate references
			for i, ref := range parsedItem.References {
				if ref.ReferenceText == "" {
					t.Errorf("Reference %d has empty ReferenceText", i)
				}
			}

			// Validate images
			for i, img := range parsedItem.Images {
				if img.ImageURL == "" && img.ImageDescription == "" && img.Caption == "" {
					t.Errorf("Image %d has empty ImageURL, ImageDescription, and Caption", i)
				}
			}

			// Validate tables
			for i, tbl := range parsedItem.Tables {
				if tbl.TableID == "" && tbl.TableTitle == "" && tbl.TableData == "" {
					t.Errorf("Table %d has empty TableID, TableTitle, and TableData", i)
				}
			}
		})
	}
}

func TestParsePDF_InvalidPDF(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey := getAPIKey(t)
	ctx := context.Background()

	invalidPDF := models.DocumentData{
		Data: []byte("This is not a PDF"),
		Type: "pdf",
	}
	_, err := ParsePDF(ctx, apiKey, invalidPDF)
	if err == nil {
		t.Error("Expected error with invalid PDF data, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestParsePDF_EmptyPDF(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey := getAPIKey(t)
	ctx := context.Background()

	emptyPDF := models.DocumentData{
		Data: []byte{},
		Type: "pdf",
	}
	_, err := ParsePDF(ctx, apiKey, emptyPDF)
	if err == nil {
		t.Error("Expected error with empty PDF data, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestParsedPage_JSONSerialization(t *testing.T) {
	// Test that ParsedPage can be properly serialized/deserialized
	original := &models.ParsedPage{
		Content: "This is test content",
		References: []models.Reference{
			{ReferenceText: "Smith et al. 2023", DOI: "10.1234/test"},
		},
		Images: []models.Image{
			{ImageURL: "data:image/png;base64,test", Caption: "Test figure"},
		},
		Tables: []models.Table{
			{TableID: "table1", TableTitle: "Test Table", TableData: "col1,col2\n1,2"},
		},
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal ParsedPage: %v", err)
	}

	// Deserialize from JSON
	var deserialized models.ParsedPage
	err = json.Unmarshal(jsonData, &deserialized)
	if err != nil {
		t.Fatalf("Failed to unmarshal ParsedPage: %v", err)
	}

	// Validate
	if deserialized.Content != original.Content {
		t.Errorf("Content mismatch: got %q, want %q", deserialized.Content, original.Content)
	}
	if len(deserialized.References) != len(original.References) {
		t.Errorf("References count mismatch: got %d, want %d", len(deserialized.References), len(original.References))
	}
	if len(deserialized.Images) != len(original.Images) {
		t.Errorf("Images count mismatch: got %d, want %d", len(deserialized.Images), len(original.Images))
	}
	if len(deserialized.Tables) != len(original.Tables) {
		t.Errorf("Tables count mismatch: got %d, want %d", len(deserialized.Tables), len(original.Tables))
	}
}

func TestParsedItem_JSONSerialization(t *testing.T) {
	// Test that ParsedItem can be properly serialized/deserialized
	original := &models.ParsedItem{
		Pages: []string{"Page 1 content", "Page 2 content"},
		References: []models.Reference{
			{ReferenceText: "Smith et al. 2023", DOI: "10.1234/test"},
		},
		Images: []models.Image{
			{ImageURL: "data:image/png;base64,test", Caption: "Test figure"},
		},
		Tables: []models.Table{
			{TableID: "table1", TableTitle: "Test Table", TableData: "col1,col2\n1,2"},
		},
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal ParsedItem: %v", err)
	}

	// Deserialize from JSON
	var deserialized models.ParsedItem
	err = json.Unmarshal(jsonData, &deserialized)
	if err != nil {
		t.Fatalf("Failed to unmarshal ParsedItem: %v", err)
	}

	// Validate
	if len(deserialized.Pages) != len(original.Pages) {
		t.Errorf("Pages count mismatch: got %d, want %d", len(deserialized.Pages), len(original.Pages))
	}
	if len(deserialized.References) != len(original.References) {
		t.Errorf("References count mismatch: got %d, want %d", len(deserialized.References), len(original.References))
	}
	if len(deserialized.Images) != len(original.Images) {
		t.Errorf("Images count mismatch: got %d, want %d", len(deserialized.Images), len(original.Images))
	}
	if len(deserialized.Tables) != len(original.Tables) {
		t.Errorf("Tables count mismatch: got %d, want %d", len(deserialized.Tables), len(original.Tables))
	}
}

func TestParsePDF_ConcurrentPageProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey := getAPIKey(t)
	ctx := context.Background()
	sampleFiles := loadSamplePDFs(t)

	for _, filePath := range sampleFiles {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			// Read the sample PDF
			pdfBytes, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read PDF file %s: %v", filePath, err)
			}

			// Get expected page count
			pages, err := documents.SplitPdf(models.DocumentData{
				Data: pdfBytes,
				Type: "pdf",
			})
			if err != nil {
				t.Fatalf("Failed to split PDF: %v", err)
			}
			expectedPageCount := len(pages)

			// Parse the entire PDF (which processes pages concurrently)
			parsedItem, err := ParsePDF(ctx, apiKey, models.DocumentData{
				Data: pdfBytes,
				Type: "pdf",
			})
			if err != nil {
				t.Fatalf("ParsePDF failed: %v", err)
			}

			// Verify all pages were processed
			if len(parsedItem.Pages) != expectedPageCount {
				t.Errorf("Expected %d pages, got %d", expectedPageCount, len(parsedItem.Pages))
			}

			// Verify no pages are nil or missing
			for i, page := range parsedItem.Pages {
				if page == "" {
					t.Logf("Warning: Page %d has empty content", i+1)
				}
			}
		})
	}
}
