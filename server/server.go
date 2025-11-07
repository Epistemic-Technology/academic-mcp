package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
	"github.com/Epistemic-Technology/academic-mcp/resources"
	"github.com/Epistemic-Technology/academic-mcp/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func CreateServer(log logger.Logger) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "academic-mcp", Version: "v0.0.1"}, nil)

	store, err := initializeStorage(log)
	if err != nil {
		log.Fatal("Failed to initialize storage: %v", err)
	}

	pdfResourceHandler := resources.NewPDFResourceHandler(store)

	// Register tools with storage and logger dependencies
	mcp.AddTool(server, tools.DocumentParseTool(), func(ctx context.Context, req *mcp.CallToolRequest, query tools.DocumentParseQuery) (*mcp.CallToolResult, *tools.DocumentParseResponse, error) {
		return tools.DocumentParseToolHandler(ctx, req, query, store, log)
	})

	mcp.AddTool(server, tools.DocumentSummarizeTool(), func(ctx context.Context, req *mcp.CallToolRequest, query tools.DocumentSummarizeQuery) (*mcp.CallToolResult, *tools.DocumentSummarizeResponse, error) {
		return tools.DocumentSummarizeToolHandler(ctx, req, query, store, log)
	})

	// Template for document summary
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}",
		Name:        "pdf-document",
		Description: "Parsed PDF document with metadata and content summary",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for metadata
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/metadata",
		Name:        "pdf-metadata",
		Description: "Document metadata including title, authors, DOI, and abstract",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for pages
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/pages",
		Name:        "pdf-pages",
		Description: "All pages of the document",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for individual page
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/pages/{pageIndex}",
		Name:        "pdf-page",
		Description: "A specific page from the document (0-indexed)",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for references
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/references",
		Name:        "pdf-references",
		Description: "All references cited in the document",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for individual reference
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/references/{referenceIndex}",
		Name:        "pdf-reference",
		Description: "A specific reference from the document (0-indexed)",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for images
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/images",
		Name:        "pdf-images",
		Description: "All images from the document",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for individual image
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/images/{imageIndex}",
		Name:        "pdf-image",
		Description: "A specific image from the document (0-indexed)",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for tables
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/tables",
		Name:        "pdf-tables",
		Description: "All tables from the document",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for individual table
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/tables/{tableIndex}",
		Name:        "pdf-table",
		Description: "A specific table from the document (0-indexed)",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for footnotes
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/footnotes",
		Name:        "pdf-footnotes",
		Description: "All footnotes from the document",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for individual footnote
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/footnotes/{footnoteIndex}",
		Name:        "pdf-footnote",
		Description: "A specific footnote from the document (0-indexed)",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for endnotes
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/endnotes",
		Name:        "pdf-endnotes",
		Description: "All endnotes from the document",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	// Template for individual endnote
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "pdf://{documentId}/endnotes/{endnoteIndex}",
		Name:        "pdf-endnote",
		Description: "A specific endnote from the document (0-indexed)",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return pdfResourceHandler.ReadResource(ctx, req.Params.URI)
	})

	return server
}

// initializeStorage creates and initializes the storage backend
func initializeStorage(log logger.Logger) (storage.Store, error) {
	// Determine database path
	dbPath := os.Getenv("ACADEMIC_MCP_DB_PATH")
	if dbPath == "" {
		// Default to ~/.academic-mcp/academic.db
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dbDir := filepath.Join(homeDir, ".academic-mcp")
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
		dbPath = filepath.Join(dbDir, "academic.db")
	}

	log.Info("Initializing SQLite database at: %s", dbPath)

	store, err := storage.NewSQLiteStore(dbPath, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite store: %w", err)
	}

	return store, nil
}
