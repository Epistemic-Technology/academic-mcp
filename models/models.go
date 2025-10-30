package models

type ParsedItem struct {
	Metadata   ItemMetadata `json:"metadata,omitempty"`
	Pages      []string     `json:"pages,omitempty"`
	References []Reference  `json:"references,omitempty"`
	Images     []Image      `json:"images,omitempty"`
	Tables     []Table      `json:"tables,omitempty"`
}

type ParsedPage struct {
	Metadata   ItemMetadata `json:"metadata,omitempty"`
	Content    string       `json:"content,omitempty"`
	Images     []Image      `json:"images,omitempty"`
	Tables     []Table      `json:"tables,omitempty"`
	References []Reference  `json:"references,omitempty"`
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
