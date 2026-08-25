package tools

import (
	"context"
	"fmt"

	"github.com/Epistemic-Technology/academic-mcp/internal/citations"
	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DocumentProcessCitationsQuery struct {
	Content      string `json:"content"`                 // Markdown content with pandoc citations
	CSLStyle     string `json:"csl_style,omitempty"`     // CSL style file path/name (optional, defaults to APA)
	OutputFormat string `json:"output_format,omitempty"` // Output format: "markdown", "html", "docx", "pdf" (default: markdown)
}

type DocumentProcessCitationsResponse struct {
	ProcessedContent string   `json:"processed_content"`  // Document with formatted citations and bibliography
	OutputFormat     string   `json:"output_format"`      // Format of the processed content
	CitationCount    int      `json:"citation_count"`     // Number of unique citations found
	Warnings         []string `json:"warnings,omitempty"` // Warnings for missing citekeys or other issues
}

func DocumentProcessCitationsTool() *mcp.Tool {
	inputschema, err := jsonschema.For[DocumentProcessCitationsQuery](nil)
	if err != nil {
		panic(err)
	}
	return &mcp.Tool{
		Name:        "document-process-citations",
		Description: "Process a markdown document containing pandoc-style citations (e.g., [@citekey]) and generate formatted output with a bibliography. Requires that cited documents have been previously parsed and stored in the library. Uses pandoc with citeproc for citation formatting.",
		InputSchema: inputschema,
	}
}

func DocumentProcessCitationsToolHandler(ctx context.Context, req *mcp.CallToolRequest, query DocumentProcessCitationsQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *DocumentProcessCitationsResponse, error) {
	log.Info("document-process-citations tool called")

	// Validate required fields
	if query.Content == "" {
		log.Error("Content is required")
		return nil, nil, fmt.Errorf("content is required")
	}

	// Set defaults
	outputFormat := query.OutputFormat
	if outputFormat == "" {
		outputFormat = "markdown"
		log.Info("Using default output format: markdown")
	}

	cslStyle := query.CSLStyle
	if cslStyle == "" {
		log.Info("No CSL style specified, using pandoc default (typically APA-like)")
	} else {
		log.Info("Using CSL style: %s", cslStyle)
	}

	// Extract citekeys to count them before processing
	citekeys, err := citations.ExtractCitekeys(query.Content)
	if err != nil {
		log.Error("Failed to extract citekeys: %v", err)
		return nil, nil, fmt.Errorf("failed to extract citekeys: %w", err)
	}

	log.Info("Found %d unique citations in document", len(citekeys))

	// Process the document with citations
	processedContent, warnings, err := citations.ProcessCitations(
		ctx,
		query.Content,
		store,
		cslStyle,
		outputFormat,
		log,
	)
	if err != nil {
		log.Error("Failed to process citations: %v", err)
		return nil, nil, fmt.Errorf("failed to process citations: %w", err)
	}

	log.Info("Successfully processed document with %d citations", len(citekeys))

	responseData := &DocumentProcessCitationsResponse{
		ProcessedContent: processedContent,
		OutputFormat:     outputFormat,
		CitationCount:    len(citekeys),
		Warnings:         warnings,
	}

	return nil, responseData, nil
}
