package storage

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

// GenerateDocumentID creates a unique document ID from source info and document data.
// This function can be called before parsing to check if a document already exists.
// Priority: Zotero ID > URL hash > document data hash
func GenerateDocumentID(sourceInfo *models.SourceInfo, documentData models.DocumentData) string {
	if sourceInfo.ZoteroID != "" {
		return "zotero_" + sourceInfo.ZoteroID
	}
	if sourceInfo.URL != "" {
		// Use SHA-256 hash of the URL
		hash := sha256.Sum256([]byte(sourceInfo.URL))
		return fmt.Sprintf("url_%x", hash[:8]) // Use first 8 bytes for shorter IDs
	}
	// Fallback to hash of document data
	hash := sha256.Sum256(documentData.Data)
	return fmt.Sprintf("data_%x", hash[:8])
}

// Store defines the interface for storing and retrieving parsed PDF data
type Store interface {
	// StoreParsedItem stores a parsed PDF with the provided document ID
	StoreParsedItem(ctx context.Context, docID string, item *models.ParsedItem, sourceInfo *models.SourceInfo) error

	// GetMetadata retrieves metadata for a document by ID
	GetMetadata(ctx context.Context, docID string) (*models.ItemMetadata, error)

	// GetPage retrieves a specific page by document ID and page number (1-indexed sequential)
	GetPage(ctx context.Context, docID string, pageNum int) (string, error)

	// GetPageBySourceNumber retrieves a page by its source page number (e.g., "125", "iv")
	GetPageBySourceNumber(ctx context.Context, docID string, sourcePageNum string) (string, error)

	// GetPages retrieves all pages for a document
	GetPages(ctx context.Context, docID string) ([]string, error)

	// GetPageMapping returns a map of source page numbers to sequential page numbers
	GetPageMapping(ctx context.Context, docID string) (map[string]int, error)

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

	// GetFootnotes retrieves all footnotes for a document
	GetFootnotes(ctx context.Context, docID string) ([]models.Footnote, error)

	// GetFootnote retrieves a specific footnote by index (0-indexed)
	GetFootnote(ctx context.Context, docID string, footnoteIndex int) (*models.Footnote, error)

	// GetEndnotes retrieves all endnotes for a document
	GetEndnotes(ctx context.Context, docID string) ([]models.Endnote, error)

	// GetEndnote retrieves a specific endnote by index (0-indexed)
	GetEndnote(ctx context.Context, docID string, endnoteIndex int) (*models.Endnote, error)

	// ListDocuments returns a list of all stored document IDs with their metadata
	ListDocuments(ctx context.Context) ([]models.DocumentInfo, error)

	// DeleteDocument removes a document and all associated data
	DeleteDocument(ctx context.Context, docID string) error

	// DocumentExists checks if a document with the given ID already exists
	DocumentExists(ctx context.Context, docID string) (bool, error)

	// GetParsedItem retrieves a complete ParsedItem for a document by ID
	GetParsedItem(ctx context.Context, docID string) (*models.ParsedItem, error)

	// Close closes the database connection
	Close() error
}
