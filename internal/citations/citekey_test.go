package citations

import (
	"testing"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

func TestGenerateCitekey_SingleAuthor(t *testing.T) {
	tests := []struct {
		name     string
		metadata *models.ItemMetadata
		want     string
	}{
		{
			name: "single author with year",
			metadata: &models.ItemMetadata{
				Authors:         []string{"Smith, John"},
				PublicationDate: "2020",
			},
			want: "smith2020",
		},
		{
			name: "single author first-last format",
			metadata: &models.ItemMetadata{
				Authors:         []string{"John Smith"},
				PublicationDate: "2021",
			},
			want: "smith2021",
		},
		{
			name: "single author no year",
			metadata: &models.ItemMetadata{
				Authors: []string{"Smith, John"},
			},
			want: "smith",
		},
		{
			name: "single author with date format",
			metadata: &models.ItemMetadata{
				Authors:         []string{"Smith, John"},
				PublicationDate: "2020-05-15",
			},
			want: "smith2020",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := make(map[string]bool)
			got := GenerateCitekey(tt.metadata, existing)
			if got != tt.want {
				t.Errorf("GenerateCitekey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateCitekey_TwoAuthors(t *testing.T) {
	tests := []struct {
		name     string
		metadata *models.ItemMetadata
		want     string
	}{
		{
			name: "two authors",
			metadata: &models.ItemMetadata{
				Authors:         []string{"Smith, John", "Jones, Mary"},
				PublicationDate: "2020",
			},
			want: "smithJones2020",
		},
		{
			name: "two authors first-last format",
			metadata: &models.ItemMetadata{
				Authors:         []string{"John Smith", "Mary Jones"},
				PublicationDate: "2021",
			},
			want: "smithJones2021",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := make(map[string]bool)
			got := GenerateCitekey(tt.metadata, existing)
			if got != tt.want {
				t.Errorf("GenerateCitekey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateCitekey_MultipleAuthors(t *testing.T) {
	tests := []struct {
		name     string
		metadata *models.ItemMetadata
		want     string
	}{
		{
			name: "three authors",
			metadata: &models.ItemMetadata{
				Authors:         []string{"Smith, John", "Jones, Mary", "Brown, Bob"},
				PublicationDate: "2020",
			},
			want: "smithEtAl2020",
		},
		{
			name: "five authors",
			metadata: &models.ItemMetadata{
				Authors:         []string{"Smith, John", "Jones, Mary", "Brown, Bob", "White, Alice", "Black, Charlie"},
				PublicationDate: "2021",
			},
			want: "smithEtAl2021",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := make(map[string]bool)
			got := GenerateCitekey(tt.metadata, existing)
			if got != tt.want {
				t.Errorf("GenerateCitekey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateCitekey_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		metadata *models.ItemMetadata
		want     string
	}{
		{
			name: "author with hyphen",
			metadata: &models.ItemMetadata{
				Authors:         []string{"Smith-Jones, John"},
				PublicationDate: "2020",
			},
			want: "smithjones2020",
		},
		{
			name: "author with apostrophe",
			metadata: &models.ItemMetadata{
				Authors:         []string{"O'Brien, Patrick"},
				PublicationDate: "2020",
			},
			want: "obrien2020",
		},
		{
			name: "author with accented characters",
			metadata: &models.ItemMetadata{
				Authors:         []string{"García, José"},
				PublicationDate: "2020",
			},
			want: "garcía2020", // Unicode letters are preserved
		},
		{
			name: "author with multi-part last name",
			metadata: &models.ItemMetadata{
				Authors:         []string{"von Neumann, John"},
				PublicationDate: "1945",
			},
			want: "vonNeumann1945",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := make(map[string]bool)
			got := GenerateCitekey(tt.metadata, existing)
			if got != tt.want {
				t.Errorf("GenerateCitekey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateCitekey_Collisions(t *testing.T) {
	tests := []struct {
		name     string
		metadata *models.ItemMetadata
		existing map[string]bool
		want     string
	}{
		{
			name: "first collision adds 'a'",
			metadata: &models.ItemMetadata{
				Authors:         []string{"Smith, John"},
				PublicationDate: "2020",
			},
			existing: map[string]bool{
				"smith2020": true,
			},
			want: "smith2020a",
		},
		{
			name: "second collision adds 'b'",
			metadata: &models.ItemMetadata{
				Authors:         []string{"Smith, John"},
				PublicationDate: "2020",
			},
			existing: map[string]bool{
				"smith2020":  true,
				"smith2020a": true,
			},
			want: "smith2020b",
		},
		{
			name: "third collision adds 'c'",
			metadata: &models.ItemMetadata{
				Authors:         []string{"Smith, John"},
				PublicationDate: "2020",
			},
			existing: map[string]bool{
				"smith2020":  true,
				"smith2020a": true,
				"smith2020b": true,
			},
			want: "smith2020c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateCitekey(tt.metadata, tt.existing)
			if got != tt.want {
				t.Errorf("GenerateCitekey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateCitekey_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		metadata *models.ItemMetadata
		want     string
	}{
		{
			name: "no authors no date",
			metadata: &models.ItemMetadata{
				Title: "Some Document",
			},
			want: "unknown",
		},
		{
			name: "empty author list",
			metadata: &models.ItemMetadata{
				Authors:         []string{},
				PublicationDate: "2020",
			},
			want: "ref2020", // Starts with digit, gets "ref" prefix
		},
		{
			name: "author but no date",
			metadata: &models.ItemMetadata{
				Authors: []string{"Smith, John"},
			},
			want: "smith",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := make(map[string]bool)
			got := GenerateCitekey(tt.metadata, existing)
			if got != tt.want {
				t.Errorf("GenerateCitekey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		name    string
		pubDate string
		want    string
	}{
		{"simple year", "2020", "2020"},
		{"ISO date", "2020-05-15", "2020"},
		{"verbose date", "May 15, 2020", "2020"},
		{"year range", "2019-2020", "2019"},
		{"month year", "January 2020", "2020"},
		{"empty string", "", ""},
		{"no year", "TBD", ""},
		{"old year", "1999", "1999"},
		{"future year", "2099", "2099"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractYear(tt.pubDate)
			if got != tt.want {
				t.Errorf("extractYear(%q) = %v, want %v", tt.pubDate, got, tt.want)
			}
		})
	}
}

func TestExtractAuthorPart(t *testing.T) {
	tests := []struct {
		name    string
		authors []string
		want    string
	}{
		{"no authors", []string{}, ""},
		{"one author", []string{"Smith, John"}, "smith"},
		{"two authors", []string{"Smith, John", "Jones, Mary"}, "smithJones"},
		{"three authors", []string{"Smith, John", "Jones, Mary", "Brown, Bob"}, "smithEtAl"},
		{"five authors", []string{"A", "B", "C", "D", "E"}, "aEtAl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAuthorPart(tt.authors)
			if got != tt.want {
				t.Errorf("extractAuthorPart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatAuthorName(t *testing.T) {
	tests := []struct {
		name   string
		author string
		want   string
	}{
		{"last, first", "Smith, John", "smith"},
		{"first last", "John Smith", "smith"},
		{"single name", "Smith", "smith"},
		{"multi-part last name", "von Neumann", "neumann"}, // Takes last part when space-separated
		{"three part name", "John von Neumann", "neumann"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAuthorName(tt.author)
			if got != tt.want {
				t.Errorf("formatAuthorName(%q) = %v, want %v", tt.author, got, tt.want)
			}
		})
	}
}

func TestSanitizeCitekey(t *testing.T) {
	tests := []struct {
		name    string
		citekey string
		want    string
	}{
		{"alphanumeric", "smith2020", "smith2020"},
		{"with spaces", "smith 2020", "smith2020"},
		{"with hyphens", "smith-jones2020", "smithjones2020"},
		{"with special chars", "smith@#$2020", "smith2020"},
		{"empty string", "", "unknown"},
		{"starts with digit", "2020smith", "ref2020smith"},
		{"only digits", "123", "ref123"},
		{"with underscores", "smith_2020", "smith_2020"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeCitekey(tt.citekey)
			if got != tt.want {
				t.Errorf("sanitizeCitekey(%q) = %v, want %v", tt.citekey, got, tt.want)
			}
		})
	}
}

func TestGenerateCitekey_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name     string
		metadata *models.ItemMetadata
		want     string
	}{
		{
			name: "typical journal article",
			metadata: &models.ItemMetadata{
				Title:           "The Structure of DNA",
				Authors:         []string{"Watson, James", "Crick, Francis"},
				PublicationDate: "1953",
				Publication:     "Nature",
			},
			want: "watsonCrick1953",
		},
		{
			name: "book with many authors",
			metadata: &models.ItemMetadata{
				Title:           "Design Patterns",
				Authors:         []string{"Gamma, Erich", "Helm, Richard", "Johnson, Ralph", "Vlissides, John"},
				PublicationDate: "1994",
				ItemType:        "book",
			},
			want: "gammaEtAl1994",
		},
		{
			name: "preprint without date",
			metadata: &models.ItemMetadata{
				Title:   "Emerging Research in AI",
				Authors: []string{"Smith, John"},
			},
			want: "smith",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := make(map[string]bool)
			got := GenerateCitekey(tt.metadata, existing)
			if got != tt.want {
				t.Errorf("GenerateCitekey() = %v, want %v", got, tt.want)
			}
		})
	}
}
