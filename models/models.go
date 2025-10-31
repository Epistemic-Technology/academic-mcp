package models

type ParsedItem struct {
	Metadata    ItemMetadata `json:"metadata,omitempty"`
	Pages       []string     `json:"pages,omitempty"`
	PageNumbers []string     `json:"page_numbers,omitempty"` // Source page numbers corresponding to Pages
	References  []Reference  `json:"references,omitempty"`
	Images      []Image      `json:"images,omitempty"`
	Tables      []Table      `json:"tables,omitempty"`
	Footnotes   []Footnote   `json:"footnotes,omitempty"`
	Endnotes    []Endnote    `json:"endnotes,omitempty"`
}

type ParsedPage struct {
	Metadata       ItemMetadata   `json:"metadata,omitempty"`
	Content        string         `json:"content,omitempty"`
	Images         []Image        `json:"images,omitempty"`
	Tables         []Table        `json:"tables,omitempty"`
	References     []Reference    `json:"references,omitempty"`
	Footnotes      []Footnote     `json:"footnotes,omitempty"`
	Endnotes       []Endnote      `json:"endnotes,omitempty"`
	PageNumberInfo PageNumberInfo `json:"page_number_info,omitempty"`
}

// PageNumberInfo contains information about the printed page number on a page
type PageNumberInfo struct {
	// PageNumber is the printed page number detected on the page (e.g., "125", "iv", "A-3")
	PageNumber string `json:"page_number,omitempty"`
	// Confidence indicates how confident the LLM is about the page number (0.0-1.0)
	Confidence float64 `json:"confidence,omitempty"`
	// Location describes where the page number was found (e.g., "bottom center", "top right")
	Location string `json:"location,omitempty"`
	// PageRangeInfo is any detected page range information from headers/titles (e.g., "Pages 125-150")
	PageRangeInfo string `json:"page_range_info,omitempty"`
}

type ItemMetadata struct {
	Title           string   `json:"title,omitempty"`
	Authors         []string `json:"authors,omitempty"`
	PublicationDate string   `json:"publication_date,omitempty"`
	Publication     string   `json:"publication,omitempty"`
	DOI             string   `json:"doi,omitempty"`
	Abstract        string   `json:"abstract,omitempty"`
}

type Reference struct {
	ReferenceText string `json:"reference_text,omitempty"`
	DOI           string `json:"doi,omitempty"`
}

type Image struct {
	ImageURL         string `json:"image_url,omitempty"`
	ImageDescription string `json:"image_description,omitempty"`
	Caption          string `json:"caption,omitempty"`
}

type Table struct {
	TableID    string `json:"table_id,omitempty"`
	TableTitle string `json:"table_title,omitempty"`
	TableData  string `json:"table_data,omitempty"`
}

// Footnote represents a footnote appearing at the bottom of a specific page
type Footnote struct {
	Marker     string `json:"marker,omitempty"`       // The footnote marker (e.g., "1", "*", "a")
	Text       string `json:"text,omitempty"`         // The full text of the footnote
	PageNumber string `json:"page_number,omitempty"`  // The page where this footnote appears
	InTextPage string `json:"in_text_page,omitempty"` // The page where the marker appears in text (if different)
}

// Endnote represents an endnote appearing at the end of a document/chapter
type Endnote struct {
	Marker     string `json:"marker,omitempty"`      // The endnote marker (e.g., "1", "i", "a")
	Text       string `json:"text,omitempty"`        // The full text of the endnote
	PageNumber string `json:"page_number,omitempty"` // The page where this endnote definition appears
}

type PdfData []byte
type PdfPageData []byte
type PdfPages []PdfPageData

// SourceInfo contains information about where the PDF came from
type SourceInfo struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
}

// DocumentInfo contains basic information about a stored document
type DocumentInfo struct {
	DocumentID string     `json:"document_id"`
	Title      string     `json:"title,omitempty"`
	Authors    []string   `json:"authors,omitempty"`
	DOI        string     `json:"doi,omitempty"`
	SourceInfo SourceInfo `json:"source_info,omitempty"`
}
