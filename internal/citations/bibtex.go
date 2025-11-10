package citations

import (
	"fmt"
	"strings"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

// GenerateBibTeXEntry creates a BibTeX entry from document metadata.
// Returns a formatted BibTeX entry string ready for inclusion in a .bib file.
func GenerateBibTeXEntry(docID string, metadata *models.ItemMetadata, citekey string) string {
	if citekey == "" {
		citekey = "unknown"
	}

	// Map item type to BibTeX entry type
	entryType := mapItemTypeToBibTeX(metadata.ItemType)

	var builder strings.Builder

	// Write entry header
	builder.WriteString(fmt.Sprintf("@%s{%s,\n", entryType, citekey))

	// Add fields in standard BibTeX order
	// Title (required for most entry types)
	if metadata.Title != "" {
		builder.WriteString(fmt.Sprintf("  title = {%s},\n", escapeBibTeX(metadata.Title)))
	}

	// Authors
	if len(metadata.Authors) > 0 {
		authorsStr := formatBibTeXAuthors(metadata.Authors)
		builder.WriteString(fmt.Sprintf("  author = {%s},\n", authorsStr))
	}

	// Publication/Journal/Book title
	if metadata.Publication != "" {
		fieldName := getPublicationFieldName(entryType)
		builder.WriteString(fmt.Sprintf("  %s = {%s},\n", fieldName, escapeBibTeX(metadata.Publication)))
	}

	// Year
	if metadata.PublicationDate != "" {
		year := extractYear(metadata.PublicationDate)
		if year != "" {
			builder.WriteString(fmt.Sprintf("  year = {%s},\n", year))
		}
	}

	// Volume
	if metadata.Volume != "" {
		builder.WriteString(fmt.Sprintf("  volume = {%s},\n", metadata.Volume))
	}

	// Number/Issue
	if metadata.Issue != "" {
		builder.WriteString(fmt.Sprintf("  number = {%s},\n", metadata.Issue))
	}

	// Pages
	if metadata.Pages != "" {
		builder.WriteString(fmt.Sprintf("  pages = {%s},\n", formatBibTeXPages(metadata.Pages)))
	}

	// Publisher
	if metadata.Publisher != "" {
		builder.WriteString(fmt.Sprintf("  publisher = {%s},\n", escapeBibTeX(metadata.Publisher)))
	}

	// DOI
	if metadata.DOI != "" {
		builder.WriteString(fmt.Sprintf("  doi = {%s},\n", metadata.DOI))
	}

	// ISSN
	if metadata.ISSN != "" {
		builder.WriteString(fmt.Sprintf("  issn = {%s},\n", metadata.ISSN))
	}

	// ISBN
	if metadata.ISBN != "" {
		builder.WriteString(fmt.Sprintf("  isbn = {%s},\n", metadata.ISBN))
	}

	// URL
	if metadata.URL != "" {
		builder.WriteString(fmt.Sprintf("  url = {%s},\n", metadata.URL))
	}

	// Abstract (optional, but useful)
	if metadata.Abstract != "" {
		builder.WriteString(fmt.Sprintf("  abstract = {%s},\n", escapeBibTeX(metadata.Abstract)))
	}

	// Close the entry
	result := builder.String()
	// Remove trailing comma from last field
	result = strings.TrimSuffix(result, ",\n")
	result += "\n}\n"

	return result
}

// mapItemTypeToBibTeX maps our ItemType field to BibTeX entry types
func mapItemTypeToBibTeX(itemType string) string {
	switch strings.ToLower(itemType) {
	case "article", "journalarticle":
		return "article"
	case "book":
		return "book"
	case "inbook", "bookchapter", "booksection":
		return "inbook"
	case "incollection":
		return "incollection"
	case "inproceedings", "conferencepaper":
		return "inproceedings"
	case "mastersthesis", "thesis":
		return "mastersthesis"
	case "phdthesis", "dissertation":
		return "phdthesis"
	case "techreport", "report":
		return "techreport"
	case "unpublished":
		return "unpublished"
	case "proceedings":
		return "proceedings"
	case "manual":
		return "manual"
	case "misc":
		return "misc"
	default:
		// Default to misc for unknown types
		return "misc"
	}
}

// getPublicationFieldName returns the appropriate BibTeX field name for the publication
// based on the entry type
func getPublicationFieldName(entryType string) string {
	switch entryType {
	case "article":
		return "journal"
	case "inproceedings":
		return "booktitle"
	case "inbook", "incollection":
		return "booktitle"
	default:
		return "journal" // Default to journal for most cases
	}
}

// formatBibTeXAuthors formats an author list for BibTeX
// BibTeX format: "Last1, First1 and Last2, First2 and Last3, First3"
// We receive authors in various formats, so we need to normalize
func formatBibTeXAuthors(authors []string) string {
	var formattedAuthors []string

	for _, author := range authors {
		// Try to parse and reformat if needed
		// If already in "Last, First" format, keep it
		// If in "First Last" format, convert it
		if strings.Contains(author, ",") {
			// Already in "Last, First" format
			formattedAuthors = append(formattedAuthors, strings.TrimSpace(author))
		} else {
			// Convert "First Last" to "Last, First"
			parts := strings.Fields(author)
			if len(parts) >= 2 {
				lastName := parts[len(parts)-1]
				firstName := strings.Join(parts[:len(parts)-1], " ")
				formattedAuthors = append(formattedAuthors, fmt.Sprintf("%s, %s", lastName, firstName))
			} else if len(parts) == 1 {
				// Just a single name (could be last name only)
				formattedAuthors = append(formattedAuthors, parts[0])
			}
		}
	}

	return strings.Join(formattedAuthors, " and ")
}

// formatBibTeXPages ensures page ranges use BibTeX format (double dash)
// Converts "123-456" to "123--456"
func formatBibTeXPages(pages string) string {
	// Replace single dash with double dash for page ranges
	// But be careful not to replace double dashes that are already there
	pages = strings.ReplaceAll(pages, "--", "DOUBLE_DASH_PLACEHOLDER")
	pages = strings.ReplaceAll(pages, "-", "--")
	pages = strings.ReplaceAll(pages, "DOUBLE_DASH_PLACEHOLDER", "--")
	return pages
}

// escapeBibTeX escapes special characters for BibTeX
// Key characters to handle:
// - Braces: protect capitalization
// - Percent signs: escape with backslash
// - Ampersands: use \&
// - Underscores: use \_
// - Dollar signs: escape with backslash
func escapeBibTeX(text string) string {
	// Basic escaping strategy:
	// 1. Escape backslashes first
	text = strings.ReplaceAll(text, "\\", "\\textbackslash{}")

	// 2. Escape special LaTeX characters
	text = strings.ReplaceAll(text, "%", "\\%")
	text = strings.ReplaceAll(text, "&", "\\&")
	text = strings.ReplaceAll(text, "_", "\\_")
	text = strings.ReplaceAll(text, "$", "\\$")
	text = strings.ReplaceAll(text, "#", "\\#")

	// 3. Protect braces in titles (common in academic writing)
	// We don't need to escape { and } as BibTeX handles them,
	// but we should be aware they exist for capitalization protection

	return text
}

// GenerateBibTeXFile generates a complete BibTeX file from multiple entries
func GenerateBibTeXFile(entries []string) string {
	var builder strings.Builder

	builder.WriteString("% BibTeX bibliography file\n")
	builder.WriteString("% Generated by academic-mcp\n\n")

	for i, entry := range entries {
		builder.WriteString(entry)
		// Add spacing between entries
		if i < len(entries)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}
