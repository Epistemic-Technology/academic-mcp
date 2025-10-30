package storage

import (
	"context"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

// Store defines the interface for storing and retrieving parsed PDF data
type Store interface {
	// StoreParsedItem stores a parsed PDF and returns a unique document ID
	StoreParsedItem(ctx context.Context, item *models.ParsedItem, sourceInfo *models.SourceInfo) (string, error)

	// GetMetadata retrieves metadata for a document by ID
	GetMetadata(ctx context.Context, docID string) (*models.ItemMetadata, error)

	// GetPage retrieves a specific page by document ID and page number (1-indexed)
	GetPage(ctx context.Context, docID string, pageNum int) (string, error)

	// GetPages retrieves all pages for a document
	GetPages(ctx context.Context, docID string) ([]string, error)

	// GetReferences retrieves all references for a document
	GetReferences(ctx context.Context, docID string) ([]models.Reference, error)

	// GetReference retrieves a specific reference by index (0-indexed)
	GetReference(ctx context.Context, docID string, refIndex int) (*models.Reference, error)

	// GetImages retrieves all images for a document
	GetImages(ctx context.Context, docID string) ([]models.Image, error)

	// GetImage retrieves a specific image by index (0-indexed)
	GetImage(ctx context.Context, docID string, imageIndex int) (*models.Image, error)

	// GetTables retrieves all tables for a document
	GetTables(ctx context.Context, docID string) ([]models.Table, error)

	// GetTable retrieves a specific table by index (0-indexed)
	GetTable(ctx context.Context, docID string, tableIndex int) (*models.Table, error)

	// ListDocuments returns a list of all stored document IDs with their metadata
	ListDocuments(ctx context.Context) ([]models.DocumentInfo, error)

	// DeleteDocument removes a document and all associated data
	DeleteDocument(ctx context.Context, docID string) error

	// Close closes the database connection
	Close() error
}
