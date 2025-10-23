package tools

type PDFSummarizeQuery struct {
	ZoteroID string `json:"zotero_id,omitempty"`
	URL      string `json:"url,omitempty"`
	RawData  []byte `json:"raw_data,omitempty"`
}

type PDFSummarizeResponse struct {
	Summary string `json:"summary,omitempty"`
}
