package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

// SQLiteStore implements the Store interface using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database tables if they don't exist
func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		title TEXT,
		authors TEXT,
		publication_date TEXT,
		publication TEXT,
		doi TEXT,
		abstract TEXT,
		zotero_id TEXT,
		url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS pages (
		document_id TEXT NOT NULL,
		page_number INTEGER NOT NULL,
		content TEXT,
		PRIMARY KEY (document_id, page_number),
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS document_references (
		document_id TEXT NOT NULL,
		ref_index INTEGER NOT NULL,
		reference_text TEXT,
		doi TEXT,
		PRIMARY KEY (document_id, ref_index),
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS images (
		document_id TEXT NOT NULL,
		image_index INTEGER NOT NULL,
		image_url TEXT,
		image_description TEXT,
		caption TEXT,
		PRIMARY KEY (document_id, image_index),
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS document_tables (
		document_id TEXT NOT NULL,
		table_index INTEGER NOT NULL,
		table_id TEXT,
		table_title TEXT,
		table_data TEXT,
		PRIMARY KEY (document_id, table_index),
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_documents_doi ON documents(doi);
	CREATE INDEX IF NOT EXISTS idx_documents_zotero_id ON documents(zotero_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// StoreParsedItem stores a parsed PDF and returns a unique document ID
func (s *SQLiteStore) StoreParsedItem(ctx context.Context, item *models.ParsedItem, sourceInfo *models.SourceInfo) (string, error) {
	// Generate document ID based on source info
	docID := generateDocumentID(item, sourceInfo)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Store metadata
	authorsJSON, err := json.Marshal(item.Metadata.Authors)
	if err != nil {
		return "", fmt.Errorf("failed to marshal authors: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO documents (id, title, authors, publication_date, publication, doi, abstract, zotero_id, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, docID, item.Metadata.Title, string(authorsJSON), item.Metadata.PublicationDate,
		item.Metadata.Publication, item.Metadata.DOI, item.Metadata.Abstract,
		sourceInfo.ZoteroID, sourceInfo.URL)
	if err != nil {
		return "", fmt.Errorf("failed to insert document: %w", err)
	}

	// Store pages
	for i, pageContent := range item.Pages {
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO pages (document_id, page_number, content)
			VALUES (?, ?, ?)
		`, docID, i+1, pageContent)
		if err != nil {
			return "", fmt.Errorf("failed to insert page %d: %w", i+1, err)
		}
	}

	// Store references
	for i, ref := range item.References {
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO document_references (document_id, ref_index, reference_text, doi)
			VALUES (?, ?, ?, ?)
		`, docID, i, ref.ReferenceText, ref.DOI)
		if err != nil {
			return "", fmt.Errorf("failed to insert reference %d: %w", i, err)
		}
	}

	// Store images
	for i, img := range item.Images {
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO images (document_id, image_index, image_url, image_description, caption)
			VALUES (?, ?, ?, ?, ?)
		`, docID, i, img.ImageURL, img.ImageDescription, img.Caption)
		if err != nil {
			return "", fmt.Errorf("failed to insert image %d: %w", i, err)
		}
	}

	// Store tables
	for i, tbl := range item.Tables {
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO document_tables (document_id, table_index, table_id, table_title, table_data)
			VALUES (?, ?, ?, ?, ?)
		`, docID, i, tbl.TableID, tbl.TableTitle, tbl.TableData)
		if err != nil {
			return "", fmt.Errorf("failed to insert table %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return docID, nil
}

// GetMetadata retrieves metadata for a document by ID
func (s *SQLiteStore) GetMetadata(ctx context.Context, docID string) (*models.ItemMetadata, error) {
	var metadata models.ItemMetadata
	var authorsJSON string

	err := s.db.QueryRowContext(ctx, `
		SELECT title, authors, publication_date, publication, doi, abstract
		FROM documents
		WHERE id = ?
	`, docID).Scan(&metadata.Title, &authorsJSON, &metadata.PublicationDate,
		&metadata.Publication, &metadata.DOI, &metadata.Abstract)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("document not found: %s", docID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query metadata: %w", err)
	}

	if err := json.Unmarshal([]byte(authorsJSON), &metadata.Authors); err != nil {
		return nil, fmt.Errorf("failed to unmarshal authors: %w", err)
	}

	return &metadata, nil
}

// GetPage retrieves a specific page by document ID and page number (1-indexed)
func (s *SQLiteStore) GetPage(ctx context.Context, docID string, pageNum int) (string, error) {
	var content string
	err := s.db.QueryRowContext(ctx, `
		SELECT content FROM pages
		WHERE document_id = ? AND page_number = ?
	`, docID, pageNum).Scan(&content)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("page not found: %s page %d", docID, pageNum)
	}
	if err != nil {
		return "", fmt.Errorf("failed to query page: %w", err)
	}

	return content, nil
}

// GetPages retrieves all pages for a document
func (s *SQLiteStore) GetPages(ctx context.Context, docID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT content FROM pages
		WHERE document_id = ?
		ORDER BY page_number
	`, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to query pages: %w", err)
	}
	defer rows.Close()

	var pages []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("failed to scan page: %w", err)
		}
		pages = append(pages, content)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pages: %w", err)
	}

	return pages, nil
}

// GetReferences retrieves all references for a document
func (s *SQLiteStore) GetReferences(ctx context.Context, docID string) ([]models.Reference, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT reference_text, doi FROM document_references
		WHERE document_id = ?
		ORDER BY ref_index
	`, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to query references: %w", err)
	}
	defer rows.Close()

	var references []models.Reference
	for rows.Next() {
		var ref models.Reference
		if err := rows.Scan(&ref.ReferenceText, &ref.DOI); err != nil {
			return nil, fmt.Errorf("failed to scan reference: %w", err)
		}
		references = append(references, ref)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating references: %w", err)
	}

	return references, nil
}

// GetReference retrieves a specific reference by index (0-indexed)
func (s *SQLiteStore) GetReference(ctx context.Context, docID string, refIndex int) (*models.Reference, error) {
	var ref models.Reference
	err := s.db.QueryRowContext(ctx, `
		SELECT reference_text, doi FROM document_references
		WHERE document_id = ? AND ref_index = ?
	`, docID, refIndex).Scan(&ref.ReferenceText, &ref.DOI)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("reference not found: %s index %d", docID, refIndex)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query reference: %w", err)
	}

	return &ref, nil
}

// GetImages retrieves all images for a document
func (s *SQLiteStore) GetImages(ctx context.Context, docID string) ([]models.Image, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT image_url, image_description, caption FROM images
		WHERE document_id = ?
		ORDER BY image_index
	`, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to query images: %w", err)
	}
	defer rows.Close()

	var images []models.Image
	for rows.Next() {
		var img models.Image
		if err := rows.Scan(&img.ImageURL, &img.ImageDescription, &img.Caption); err != nil {
			return nil, fmt.Errorf("failed to scan image: %w", err)
		}
		images = append(images, img)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating images: %w", err)
	}

	return images, nil
}

// GetImage retrieves a specific image by index (0-indexed)
func (s *SQLiteStore) GetImage(ctx context.Context, docID string, imageIndex int) (*models.Image, error) {
	var img models.Image
	err := s.db.QueryRowContext(ctx, `
		SELECT image_url, image_description, caption FROM images
		WHERE document_id = ? AND image_index = ?
	`, docID, imageIndex).Scan(&img.ImageURL, &img.ImageDescription, &img.Caption)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("image not found: %s index %d", docID, imageIndex)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query image: %w", err)
	}

	return &img, nil
}

// GetTables retrieves all tables for a document
func (s *SQLiteStore) GetTables(ctx context.Context, docID string) ([]models.Table, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT table_id, table_title, table_data FROM document_tables
		WHERE document_id = ?
		ORDER BY table_index
	`, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []models.Table
	for rows.Next() {
		var tbl models.Table
		if err := rows.Scan(&tbl.TableID, &tbl.TableTitle, &tbl.TableData); err != nil {
			return nil, fmt.Errorf("failed to scan table: %w", err)
		}
		tables = append(tables, tbl)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	return tables, nil
}

// GetTable retrieves a specific table by index (0-indexed)
func (s *SQLiteStore) GetTable(ctx context.Context, docID string, tableIndex int) (*models.Table, error) {
	var tbl models.Table
	err := s.db.QueryRowContext(ctx, `
		SELECT table_id, table_title, table_data FROM document_tables
		WHERE document_id = ? AND table_index = ?
	`, docID, tableIndex).Scan(&tbl.TableID, &tbl.TableTitle, &tbl.TableData)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("table not found: %s index %d", docID, tableIndex)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query table: %w", err)
	}

	return &tbl, nil
}

// ListDocuments returns a list of all stored document IDs with their metadata
func (s *SQLiteStore) ListDocuments(ctx context.Context) ([]models.DocumentInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, authors, doi, zotero_id, url
		FROM documents
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %w", err)
	}
	defer rows.Close()

	var documents []models.DocumentInfo
	for rows.Next() {
		var doc models.DocumentInfo
		var authorsJSON string
		if err := rows.Scan(&doc.DocumentID, &doc.Title, &authorsJSON, &doc.DOI,
			&doc.SourceInfo.ZoteroID, &doc.SourceInfo.URL); err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}

		if err := json.Unmarshal([]byte(authorsJSON), &doc.Authors); err != nil {
			return nil, fmt.Errorf("failed to unmarshal authors: %w", err)
		}

		documents = append(documents, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating documents: %w", err)
	}

	return documents, nil
}

// DeleteDocument removes a document and all associated data
func (s *SQLiteStore) DeleteDocument(ctx context.Context, docID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, docID)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("document not found: %s", docID)
	}

	return nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// generateDocumentID creates a unique document ID based on available information
func generateDocumentID(item *models.ParsedItem, sourceInfo *models.SourceInfo) string {
	if sourceInfo.ZoteroID != "" {
		return "zotero_" + sourceInfo.ZoteroID
	}
	if item.Metadata.DOI != "" {
		return "doi_" + item.Metadata.DOI
	}
	if sourceInfo.URL != "" {
		// Use a hash of the URL for the ID
		return fmt.Sprintf("url_%x", hashString(sourceInfo.URL))
	}
	// Fallback to a hash of the title
	return fmt.Sprintf("title_%x", hashString(item.Metadata.Title))
}

// hashString creates a simple hash of a string
func hashString(s string) uint32 {
	var hash uint32
	for i := 0; i < len(s); i++ {
		hash = hash*31 + uint32(s[i])
	}
	return hash
}

// Ensure SQLiteStore implements Store interface
var _ Store = (*SQLiteStore)(nil)
