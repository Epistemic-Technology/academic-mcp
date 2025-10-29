package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Epistemic-Technology/academic-mcp/models"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestSplitPdf(t *testing.T) {
	// Get all PDF files in the samples directory
	samplesDir := filepath.Join("..", "samples")
	files, err := filepath.Glob(filepath.Join(samplesDir, "*.pdf"))
	if err != nil {
		t.Fatalf("Failed to list sample PDFs: %v", err)
	}

	if len(files) == 0 {
		t.Skip("No sample PDFs found in samples directory")
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			// Read the sample PDF
			pdfBytes, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read PDF file %s: %v", filePath, err)
			}

			// Get the expected page count
			reader := bytes.NewReader(pdfBytes)
			expectedPageCount, err := api.PageCount(reader, nil)
			if err != nil {
				t.Fatalf("Failed to get page count: %v", err)
			}

			t.Logf("PDF %s has %d pages", filepath.Base(filePath), expectedPageCount)

			// Split the PDF
			pages, err := SplitPdf(models.PdfData(pdfBytes))
			if err != nil {
				t.Fatalf("SplitPdf failed: %v", err)
			}

			// Validate we got the expected number of pages
			if len(pages) != expectedPageCount {
				t.Errorf("Expected %d pages, got %d", expectedPageCount, len(pages))
			}

			// Validate each page is a valid single-page PDF
			for i, pageData := range pages {
				if len(pageData) == 0 {
					t.Errorf("Page %d is empty", i+1)
					continue
				}

				// Try to read the page as a PDF
				pageReader := bytes.NewReader(pageData)
				_, err := api.ReadContext(pageReader, nil)
				if err != nil {
					t.Errorf("Page %d is not a valid PDF: %v", i+1, err)
					continue
				}

				// Verify it's a single-page PDF
				pageReader = bytes.NewReader(pageData)
				pageCount, err := api.PageCount(pageReader, nil)
				if err != nil {
					t.Errorf("Failed to get page count for page %d: %v", i+1, err)
					continue
				}

				if pageCount != 1 {
					t.Errorf("Page %d should have 1 page, but has %d", i+1, pageCount)
				}
			}
		})
	}
}

func TestSplitPdf_EmptyInput(t *testing.T) {
	_, err := SplitPdf(models.PdfData([]byte{}))
	if err == nil {
		t.Error("Expected error for empty PDF data, got nil")
	}
}

func TestSplitPdf_InvalidInput(t *testing.T) {
	invalidData := []byte("This is not a PDF")
	_, err := SplitPdf(models.PdfData(invalidData))
	if err == nil {
		t.Error("Expected error for invalid PDF data, got nil")
	}
}
