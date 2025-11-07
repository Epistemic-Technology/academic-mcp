package storage

import (
	"fmt"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

// CalculateResourcePaths generates all available resource URIs for a parsed document.
// Returns a slice of resource paths based on the content available in the parsed item.
func CalculateResourcePaths(docID string, parsedItem *models.ParsedItem) []string {
	resourcePaths := []string{
		fmt.Sprintf("pdf://%s", docID),
		fmt.Sprintf("pdf://%s/metadata", docID),
		fmt.Sprintf("pdf://%s/pages", docID),
	}

	// Add sample page paths if source page numbers are available
	if len(parsedItem.PageNumbers) > 0 {
		firstPage := parsedItem.PageNumbers[0]
		lastPage := parsedItem.PageNumbers[len(parsedItem.PageNumbers)-1]
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/pages/%s", docID, firstPage),
			fmt.Sprintf("pdf://%s/pages/%s", docID, lastPage),
		)
	}

	// Add template for accessing any page
	resourcePaths = append(resourcePaths, fmt.Sprintf("pdf://%s/pages/{sourcePageNumber}", docID))

	// Add reference paths if references exist
	if len(parsedItem.References) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/references", docID),
			fmt.Sprintf("pdf://%s/references/{refIndex}", docID),
		)
	}

	// Add image paths if images exist
	if len(parsedItem.Images) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/images", docID),
			fmt.Sprintf("pdf://%s/images/{imageIndex}", docID),
		)
	}

	// Add table paths if tables exist
	if len(parsedItem.Tables) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/tables", docID),
			fmt.Sprintf("pdf://%s/tables/{tableIndex}", docID),
		)
	}

	// Add footnote paths if footnotes exist
	if len(parsedItem.Footnotes) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/footnotes", docID),
			fmt.Sprintf("pdf://%s/footnotes/{footnoteIndex}", docID),
		)
	}

	// Add endnote paths if endnotes exist
	if len(parsedItem.Endnotes) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/endnotes", docID),
			fmt.Sprintf("pdf://%s/endnotes/{endnoteIndex}", docID),
		)
	}

	// Add quotation paths if quotations exist
	if len(parsedItem.Quotations) > 0 {
		resourcePaths = append(resourcePaths,
			fmt.Sprintf("pdf://%s/quotations", docID),
			fmt.Sprintf("pdf://%s/quotations/{quotationIndex}", docID),
		)
	}

	return resourcePaths
}
