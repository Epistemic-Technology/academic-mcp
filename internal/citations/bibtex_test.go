package citations

import (
	"strings"
	"testing"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

func TestGenerateBibTeXEntry(t *testing.T) {
	tests := []struct {
		name     string
		docID    string
		metadata *models.ItemMetadata
		citekey  string
		want     []string // Strings that should appear in the output
	}{
		{
			name:  "article with full metadata",
			docID: "test-doc-1",
			metadata: &models.ItemMetadata{
				Title:           "Machine Learning in Climate Science",
				Authors:         []string{"Smith, John", "Doe, Jane"},
				PublicationDate: "2020-05-15",
				Publication:     "Nature Climate Change",
				DOI:             "10.1038/s41558-020-0000-0",
				ItemType:        "article",
				Volume:          "10",
				Issue:           "5",
				Pages:           "123-130",
				Abstract:        "This paper explores machine learning applications.",
			},
			citekey: "smithDoe2020",
			want: []string{
				"@article{smithDoe2020,",
				"title = {Machine Learning in Climate Science}",
				"author = {Smith, John and Doe, Jane}",
				"journal = {Nature Climate Change}",
				"year = {2020}",
				"volume = {10}",
				"number = {5}",
				"pages = {123--130}",
				"doi = {10.1038/s41558-020-0000-0}",
			},
		},
		{
			name:  "book with minimal metadata",
			docID: "test-doc-2",
			metadata: &models.ItemMetadata{
				Title:           "Introduction to Algorithms",
				Authors:         []string{"Cormen, Thomas", "Leiserson, Charles", "Rivest, Ronald"},
				PublicationDate: "2009",
				Publisher:       "MIT Press",
				ItemType:        "book",
				ISBN:            "978-0262033848",
			},
			citekey: "cormenEtAl2009",
			want: []string{
				"@book{cormenEtAl2009,",
				"title = {Introduction to Algorithms}",
				"author = {Cormen, Thomas and Leiserson, Charles and Rivest, Ronald}",
				"year = {2009}",
				"publisher = {MIT Press}",
				"isbn = {978-0262033848}",
			},
		},
		{
			name:  "conference paper",
			docID: "test-doc-3",
			metadata: &models.ItemMetadata{
				Title:           "Neural Networks for Image Recognition",
				Authors:         []string{"Wang, Li"},
				PublicationDate: "2021",
				Publication:     "Proceedings of CVPR 2021",
				ItemType:        "conferencePaper",
				Pages:           "1234-1243",
			},
			citekey: "wang2021",
			want: []string{
				"@inproceedings{wang2021,",
				"title = {Neural Networks for Image Recognition}",
				"author = {Wang, Li}",
				"booktitle = {Proceedings of CVPR 2021}",
				"year = {2021}",
				"pages = {1234--1243}",
			},
		},
		{
			name:  "entry with special characters",
			docID: "test-doc-4",
			metadata: &models.ItemMetadata{
				Title:           "Analysis of CO2 & Temperature Effects: 50% Increase",
				Authors:         []string{"Müller, Hans"},
				PublicationDate: "2022",
				Publication:     "Environmental Science & Technology",
				ItemType:        "article",
			},
			citekey: "muller2022",
			want: []string{
				"@article{muller2022,",
				"title = {Analysis of CO2 \\& Temperature Effects: 50\\% Increase}",
				"author = {Müller, Hans}",
				"journal = {Environmental Science \\& Technology}",
				"year = {2022}",
			},
		},
		{
			name:  "unknown item type defaults to misc",
			docID: "test-doc-5",
			metadata: &models.ItemMetadata{
				Title:           "Miscellaneous Document",
				Authors:         []string{"Brown, Alice"},
				PublicationDate: "2023",
				ItemType:        "unknownType",
			},
			citekey: "brown2023",
			want: []string{
				"@misc{brown2023,",
				"title = {Miscellaneous Document}",
				"author = {Brown, Alice}",
				"year = {2023}",
			},
		},
		{
			name:  "entry with URL",
			docID: "test-doc-6",
			metadata: &models.ItemMetadata{
				Title:           "Online Resource",
				Authors:         []string{"Green, Bob"},
				PublicationDate: "2024",
				URL:             "https://example.com/paper",
				ItemType:        "misc",
			},
			citekey: "green2024",
			want: []string{
				"@misc{green2024,",
				"url = {https://example.com/paper}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBibTeXEntry(tt.docID, tt.metadata, tt.citekey)

			// Check that all expected strings appear in output
			for _, wantStr := range tt.want {
				if !strings.Contains(got, wantStr) {
					t.Errorf("GenerateBibTeXEntry() missing expected string:\nwant: %s\ngot:  %s", wantStr, got)
				}
			}

			// Check that entry is properly closed
			if !strings.HasSuffix(strings.TrimSpace(got), "}") {
				t.Errorf("GenerateBibTeXEntry() not properly closed: %s", got)
			}
		})
	}
}

func TestMapItemTypeToBibTeX(t *testing.T) {
	tests := []struct {
		itemType string
		want     string
	}{
		{"article", "article"},
		{"journalArticle", "article"},
		{"book", "book"},
		{"conferencePaper", "inproceedings"},
		{"thesis", "mastersthesis"},
		{"phdthesis", "phdthesis"},
		{"techreport", "techreport"},
		{"unknownType", "misc"},
		{"", "misc"},
	}

	for _, tt := range tests {
		t.Run(tt.itemType, func(t *testing.T) {
			got := mapItemTypeToBibTeX(tt.itemType)
			if got != tt.want {
				t.Errorf("mapItemTypeToBibTeX(%q) = %q, want %q", tt.itemType, got, tt.want)
			}
		})
	}
}

func TestFormatBibTeXAuthors(t *testing.T) {
	tests := []struct {
		name    string
		authors []string
		want    string
	}{
		{
			name:    "single author comma format",
			authors: []string{"Smith, John"},
			want:    "Smith, John",
		},
		{
			name:    "single author space format",
			authors: []string{"John Smith"},
			want:    "Smith, John",
		},
		{
			name:    "multiple authors mixed format",
			authors: []string{"Smith, John", "Jane Doe"},
			want:    "Smith, John and Doe, Jane",
		},
		{
			name:    "three authors",
			authors: []string{"Smith, John", "Doe, Jane", "Brown, Alice"},
			want:    "Smith, John and Doe, Jane and Brown, Alice",
		},
		{
			name:    "author with middle name",
			authors: []string{"John Michael Smith"},
			want:    "Smith, John Michael",
		},
		{
			name:    "single name only",
			authors: []string{"Smith"},
			want:    "Smith",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBibTeXAuthors(tt.authors)
			if got != tt.want {
				t.Errorf("formatBibTeXAuthors(%v) = %q, want %q", tt.authors, got, tt.want)
			}
		})
	}
}

func TestFormatBibTeXPages(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123-456", "123--456"},
		{"123--456", "123--456"}, // Already formatted
		{"1-10", "1--10"},
		{"100", "100"}, // Single page
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatBibTeXPages(tt.input)
			if got != tt.want {
				t.Errorf("formatBibTeXPages(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapeBibTeX(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Simple text", "Simple text"},
		{"Text & more", "Text \\& more"},
		{"50% increase", "50\\% increase"},
		{"Variable_name", "Variable\\_name"},
		{"Cost $100", "Cost \\$100"},
		{"Section #1", "Section \\#1"},
		{"Multiple & special % chars", "Multiple \\& special \\% chars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeBibTeX(tt.input)
			if got != tt.want {
				t.Errorf("escapeBibTeX(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateBibTeXFile(t *testing.T) {
	entries := []string{
		"@article{smith2020,\n  title = {First Paper}\n}",
		"@book{doe2021,\n  title = {Second Book}\n}",
	}

	result := GenerateBibTeXFile(entries)

	// Check header
	if !strings.Contains(result, "% BibTeX bibliography file") {
		t.Error("GenerateBibTeXFile() missing header")
	}

	// Check all entries are included
	for _, entry := range entries {
		if !strings.Contains(result, entry) {
			t.Errorf("GenerateBibTeXFile() missing entry: %s", entry)
		}
	}
}
