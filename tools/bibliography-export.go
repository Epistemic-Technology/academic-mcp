package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Epistemic-Technology/academic-mcp/internal/citations"
	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type BibliographyExportQuery struct {
	DocumentIDs []string `json:"document_ids,omitempty"`
	Format      string   `json:"format,omitempty"` // Currently only "bibtex" is supported
}

type BibliographyExportResponse struct {
	Format         string   `json:"format"`
	Content        string   `json:"content"`
	DocumentCount  int      `json:"document_count"`
	MissingCitekey []string `json:"missing_citekey,omitempty"`
}

func BibliographyExportTool() *mcp.Tool {
	inputschema, err := jsonschema.For[BibliographyExportQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "bibliography-export",
		Description: "Export bibliography in BibTeX format. If document_ids are specified, exports only those documents. If not specified, exports the entire library. All documents must have been previously parsed.",
		InputSchema: inputschema,
	}
}

func BibliographyExportToolHandler(ctx context.Context, req *mcp.CallToolRequest, query BibliographyExportQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *BibliographyExportResponse, error) {
	log.Info("bibliography-export tool called")

	// Default to BibTeX format
	format := query.Format
	if format == "" {
		format = "bibtex"
	}

	// Currently only BibTeX is supported
	if strings.ToLower(format) != "bibtex" {
		log.Error("Unsupported format: %s", format)
		return nil, nil, fmt.Errorf("unsupported format: %s (only 'bibtex' is supported)", format)
	}

	// Determine which documents to export
	var documentIDs []string
	if len(query.DocumentIDs) > 0 {
		// Export specific documents
		documentIDs = query.DocumentIDs
		log.Info("Exporting %d specific documents", len(documentIDs))
	} else {
		// Export entire library
		log.Info("Exporting entire library")
		docInfos, err := store.ListDocuments(ctx)
		if err != nil {
			log.Error("Failed to list documents: %v", err)
			return nil, nil, fmt.Errorf("failed to list documents: %w", err)
		}
		for _, docInfo := range docInfos {
			documentIDs = append(documentIDs, docInfo.DocumentID)
		}
		log.Info("Found %d documents in library", len(documentIDs))
	}

	// Generate BibTeX entries for each document
	var entries []string
	var missingCitekey []string

	for _, docID := range documentIDs {
		// Get metadata for the document
		metadata, err := store.GetMetadata(ctx, docID)
		if err != nil {
			log.Error("Failed to get metadata for document %s: %v", docID, err)
			return nil, nil, fmt.Errorf("failed to get metadata for document %s: %w", docID, err)
		}

		// Check if citekey exists
		if metadata.Citekey == "" {
			log.Warn("Document %s does not have a citekey", docID)
			missingCitekey = append(missingCitekey, docID)
			continue
		}

		// Generate BibTeX entry
		entry := citations.GenerateBibTeXEntry(docID, metadata, metadata.Citekey)
		entries = append(entries, entry)
		log.Info("Generated BibTeX entry for %s (citekey: %s)", docID, metadata.Citekey)
	}

	// Generate complete BibTeX file
	bibContent := citations.GenerateBibTeXFile(entries)

	log.Info("Successfully generated BibTeX file with %d entries", len(entries))

	responseData := &BibliographyExportResponse{
		Format:         format,
		Content:        bibContent,
		DocumentCount:  len(entries),
		MissingCitekey: missingCitekey,
	}

	return nil, responseData, nil
}
