package citations

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

// GenerateCitekey creates a pandoc-style citekey from metadata.
// Format: author(s)Year (e.g., "smith2020", "smithJones2021", "smithEtAl2020")
// If a collision is detected, appends a letter suffix (a, b, c, etc.)
func GenerateCitekey(metadata *models.ItemMetadata, existingCitekeys map[string]bool) string {
	// Extract year from publication date
	year := extractYear(metadata.PublicationDate)

	// Extract author part
	authorPart := extractAuthorPart(metadata.Authors)

	// Create base citekey
	baseCitekey := authorPart + year

	// Handle empty base citekey
	if baseCitekey == "" {
		baseCitekey = "unknown"
	}

	// Ensure pandoc compatibility (alphanumerics, underscores, internal punctuation)
	baseCitekey = sanitizeCitekey(baseCitekey)

	// Handle collisions by adding suffix
	citekey := baseCitekey
	suffix := 'a'
	for existingCitekeys[citekey] {
		citekey = baseCitekey + string(suffix)
		suffix++
		// Safety check to prevent infinite loop
		if suffix > 'z' {
			// If we run out of letters, start using numbers
			numSuffix := 1
			for existingCitekeys[baseCitekey+string('z')+string(rune('0'+numSuffix))] {
				numSuffix++
			}
			citekey = baseCitekey + string('z') + string(rune('0'+numSuffix))
			break
		}
	}

	return citekey
}

// extractYear extracts a 4-digit year from a publication date string
// Handles formats like "2020", "2020-01-15", "January 2020", etc.
func extractYear(pubDate string) string {
	if pubDate == "" {
		return ""
	}

	// Look for 4 consecutive digits
	re := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	matches := re.FindString(pubDate)
	if matches != "" {
		return matches
	}

	return ""
}

// extractAuthorPart creates the author portion of the citekey
// Rules:
// - No authors: return empty string
// - 1 author: use last name
// - 2 authors: use both last names (e.g., "smithJones")
// - 3+ authors: use first author's last name + "EtAl"
func extractAuthorPart(authors []string) string {
	if len(authors) == 0 {
		return ""
	}

	if len(authors) == 1 {
		return formatAuthorName(authors[0])
	}

	if len(authors) == 2 {
		first := formatAuthorName(authors[0])
		second := formatAuthorName(authors[1])
		// Capitalize first letter of second author
		if len(second) > 0 {
			second = strings.ToUpper(string(second[0])) + second[1:]
		}
		return first + second
	}

	// 3 or more authors
	first := formatAuthorName(authors[0])
	return first + "EtAl"
}

// formatAuthorName extracts and formats the last name from an author string
// Handles formats like:
// - "Smith, John" -> "smith"
// - "John Smith" -> "smith"
// - "Smith" -> "smith"
// - "von Neumann, John" -> "vonNeumann"
func formatAuthorName(author string) string {
	if author == "" {
		return ""
	}

	var lastName string

	// Check if comma-separated (Last, First)
	if strings.Contains(author, ",") {
		parts := strings.Split(author, ",")
		lastName = strings.TrimSpace(parts[0])
	} else {
		// Assume "First Last" or just "Last"
		parts := strings.Fields(author)
		if len(parts) > 0 {
			lastName = parts[len(parts)-1]
		}
	}

	// Handle multi-part last names (e.g., "von Neumann" -> "vonNeumann")
	if strings.Contains(lastName, " ") {
		parts := strings.Fields(lastName)
		// Lowercase first part, capitalize subsequent parts
		result := strings.ToLower(parts[0])
		for i := 1; i < len(parts); i++ {
			if len(parts[i]) > 0 {
				result += strings.ToUpper(string(parts[i][0])) + strings.ToLower(parts[i][1:])
			}
		}
		return result
	}

	// Simple case: just lowercase the last name
	return strings.ToLower(lastName)
}

// sanitizeCitekey ensures the citekey is pandoc-compatible
// Pandoc allows: alphanumerics, underscores, and internal punctuation
// We'll be conservative and only allow: letters, digits, underscores
func sanitizeCitekey(citekey string) string {
	var result strings.Builder

	for _, r := range citekey {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			result.WriteRune(r)
		}
	}

	sanitized := result.String()

	// Ensure it's not empty and doesn't start with a digit
	if sanitized == "" {
		return "unknown"
	}

	// If it starts with a digit, prepend "ref"
	if unicode.IsDigit(rune(sanitized[0])) {
		sanitized = "ref" + sanitized
	}

	return sanitized
}
