package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/models"
)

// SQLiteStore implements the Store interface using SQLite
type SQLiteStore struct {
	db     *sql.DB
	logger logger.Logger
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(dbPath string, log logger.Logger) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{db: db, logger: log}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Debug("SQLite store initialized successfully")

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
		source_page_number TEXT NOT NULL,
		content TEXT,
		PRIMARY KEY (document_id, page_number),
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_pages_source_number ON pages(document_id, source_page_number);

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

	CREATE TABLE IF NOT EXISTS footnotes (
		document_id TEXT NOT NULL,
		footnote_index INTEGER NOT NULL,
		marker TEXT,
		text TEXT,
		page_number TEXT,
		in_text_page TEXT,
		PRIMARY KEY (document_id, footnote_index),
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS endnotes (
		document_id TEXT NOT NULL,
		endnote_index INTEGER NOT NULL,
		marker TEXT,
		text TEXT,
		page_number TEXT,
		PRIMARY KEY (document_id, endnote_index),
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_documents_doi ON documents(doi);
	CREATE INDEX IF NOT EXISTS idx_documents_zotero_id ON documents(zotero_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// StoreParsedItem stores a parsed PDF with the provided document ID
func (s *SQLiteStore) StoreParsedItem(ctx context.Context, docID string, item *models.ParsedItem, sourceInfo *models.SourceInfo) error {
	s.logger.Info("Storing parsed document: %s (title: %s, pages: %d, refs: %d)",
		docID, item.Metadata.Title, len(item.Pages), len(item.References))

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("Failed to begin transaction for document %s: %v", docID, err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Store metadata
	authorsJSON, err := json.Marshal(item.Metadata.Authors)
	if err != nil {
		return fmt.Errorf("failed to marshal authors: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO documents (id, title, authors, publication_date, publication, doi, abstract, zotero_id, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, docID, item.Metadata.Title, string(authorsJSON), item.Metadata.PublicationDate,
		item.Metadata.Publication, item.Metadata.DOI, item.Metadata.Abstract,
		sourceInfo.ZoteroID, sourceInfo.URL)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	// Store pages
	for i, pageContent := range item.Pages {
		sourcePageNum := fmt.Sprintf("%d", i+1) // Default to sequential numbering
		if i < len(item.PageNumbers) && item.PageNumbers[i] != "" {
			sourcePageNum = item.PageNumbers[i]
		}

		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO pages (document_id, page_number, source_page_number, content)
			VALUES (?, ?, ?, ?)
		`, docID, i+1, sourcePageNum, pageContent)
		if err != nil {
			return fmt.Errorf("failed to insert page %d: %w", i+1, err)
		}
	}

	// Store references
	for i, ref := range item.References {
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO document_references (document_id, ref_index, reference_text, doi)
			VALUES (?, ?, ?, ?)
		`, docID, i, ref.ReferenceText, ref.DOI)
		if err != nil {
			return fmt.Errorf("failed to insert reference %d: %w", i, err)
		}
	}

	// Store images
	for i, img := range item.Images {
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO images (document_id, image_index, image_url, image_description, caption)
			VALUES (?, ?, ?, ?, ?)
		`, docID, i, img.ImageURL, img.ImageDescription, img.Caption)
		if err != nil {
			return fmt.Errorf("failed to insert image %d: %w", i, err)
		}
	}

	// Store tables
	for i, tbl := range item.Tables {
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO document_tables (document_id, table_index, table_id, table_title, table_data)
			VALUES (?, ?, ?, ?, ?)
		`, docID, i, tbl.TableID, tbl.TableTitle, tbl.TableData)
		if err != nil {
			return fmt.Errorf("failed to insert table %d: %w", i, err)
		}
	}

	// Store footnotes
	for i, footnote := range item.Footnotes {
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO footnotes (document_id, footnote_index, marker, text, page_number, in_text_page)
			VALUES (?, ?, ?, ?, ?, ?)
		`, docID, i, footnote.Marker, footnote.Text, footnote.PageNumber, footnote.InTextPage)
		if err != nil {
			return fmt.Errorf("failed to insert footnote %d: %w", i, err)
		}
	}

	// Store endnotes
	for i, endnote := range item.Endnotes {
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO endnotes (document_id, endnote_index, marker, text, page_number)
			VALUES (?, ?, ?, ?, ?)
		`, docID, i, endnote.Marker, endnote.Text, endnote.PageNumber)
		if err != nil {
			return fmt.Errorf("failed to insert endnote %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("Failed to commit transaction for document %s: %v", docID, err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Debug("Successfully stored document %s", docID)
	return nil
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

// GetPage retrieves a specific page by document ID and page number (1-indexed sequential)
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

// GetPageBySourceNumber retrieves a page by its source page number (e.g., "125", "iv")
func (s *SQLiteStore) GetPageBySourceNumber(ctx context.Context, docID string, sourcePageNum string) (string, error) {
	var content string
	err := s.db.QueryRowContext(ctx, `
		SELECT content FROM pages
		WHERE document_id = ? AND source_page_number = ?
	`, docID, sourcePageNum).Scan(&content)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("page not found: %s source page %s", docID, sourcePageNum)
	}
	if err != nil {
		return "", fmt.Errorf("failed to query page by source number: %w", err)
	}

	return content, nil
}

// GetPageMapping returns a map of source page numbers to sequential page numbers
func (s *SQLiteStore) GetPageMapping(ctx context.Context, docID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_page_number, page_number FROM pages
		WHERE document_id = ?
		ORDER BY page_number
	`, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to query page mapping: %w", err)
	}
	defer rows.Close()

	mapping := make(map[string]int)
	for rows.Next() {
		var sourcePageNum string
		var pageNum int
		if err := rows.Scan(&sourcePageNum, &pageNum); err != nil {
			return nil, fmt.Errorf("failed to scan page mapping: %w", err)
		}
		mapping[sourcePageNum] = pageNum
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating page mapping: %w", err)
	}

	return mapping, nil
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

// GetFootnotes retrieves all footnotes for a document
func (s *SQLiteStore) GetFootnotes(ctx context.Context, docID string) ([]models.Footnote, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT marker, text, page_number, in_text_page FROM footnotes
		WHERE document_id = ?
		ORDER BY footnote_index
	`, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to query footnotes: %w", err)
	}
	defer rows.Close()

	var footnotes []models.Footnote
	for rows.Next() {
		var fn models.Footnote
		if err := rows.Scan(&fn.Marker, &fn.Text, &fn.PageNumber, &fn.InTextPage); err != nil {
			return nil, fmt.Errorf("failed to scan footnote: %w", err)
		}
		footnotes = append(footnotes, fn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating footnotes: %w", err)
	}

	return footnotes, nil
}

// GetFootnote retrieves a specific footnote by index (0-indexed)
func (s *SQLiteStore) GetFootnote(ctx context.Context, docID string, footnoteIndex int) (*models.Footnote, error) {
	var fn models.Footnote
	err := s.db.QueryRowContext(ctx, `
		SELECT marker, text, page_number, in_text_page FROM footnotes
		WHERE document_id = ? AND footnote_index = ?
	`, docID, footnoteIndex).Scan(&fn.Marker, &fn.Text, &fn.PageNumber, &fn.InTextPage)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("footnote not found: %s index %d", docID, footnoteIndex)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query footnote: %w", err)
	}

	return &fn, nil
}

// GetEndnotes retrieves all endnotes for a document
func (s *SQLiteStore) GetEndnotes(ctx context.Context, docID string) ([]models.Endnote, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT marker, text, page_number FROM endnotes
		WHERE document_id = ?
		ORDER BY endnote_index
	`, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to query endnotes: %w", err)
	}
	defer rows.Close()

	var endnotes []models.Endnote
	for rows.Next() {
		var en models.Endnote
		if err := rows.Scan(&en.Marker, &en.Text, &en.PageNumber); err != nil {
			return nil, fmt.Errorf("failed to scan endnote: %w", err)
		}
		endnotes = append(endnotes, en)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating endnotes: %w", err)
	}

	return endnotes, nil
}

// GetEndnote retrieves a specific endnote by index (0-indexed)
func (s *SQLiteStore) GetEndnote(ctx context.Context, docID string, endnoteIndex int) (*models.Endnote, error) {
	var en models.Endnote
	err := s.db.QueryRowContext(ctx, `
		SELECT marker, text, page_number FROM endnotes
		WHERE document_id = ? AND endnote_index = ?
	`, docID, endnoteIndex).Scan(&en.Marker, &en.Text, &en.PageNumber)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("endnote not found: %s index %d", docID, endnoteIndex)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query endnote: %w", err)
	}

	return &en, nil
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

// DocumentExists checks if a document with the given ID already exists
func (s *SQLiteStore) DocumentExists(ctx context.Context, docID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM documents WHERE id = ?)`, docID).Scan(&exists)
	if err != nil {
		s.logger.Error("Failed to check document existence for %s: %v", docID, err)
		return false, fmt.Errorf("failed to check document existence: %w", err)
	}
	s.logger.Debug("Document %s exists: %v", docID, exists)
	return exists, nil
}

// GetParsedItem retrieves a complete ParsedItem for a document by ID
func (s *SQLiteStore) GetParsedItem(ctx context.Context, docID string) (*models.ParsedItem, error) {
	// Get metadata
	metadata, err := s.GetMetadata(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Get pages
	pages, err := s.GetPages(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pages: %w", err)
	}

	// Get page mapping to reconstruct page numbers
	pageMapping, err := s.GetPageMapping(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get page mapping: %w", err)
	}

	// Build PageNumbers array from mapping
	pageNumbers := make([]string, len(pages))
	for sourcePageNum, seqNum := range pageMapping {
		if seqNum > 0 && seqNum <= len(pageNumbers) {
			pageNumbers[seqNum-1] = sourcePageNum
		}
	}

	// Get references
	references, err := s.GetReferences(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get references: %w", err)
	}

	// Get images
	images, err := s.GetImages(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get images: %w", err)
	}

	// Get tables
	tables, err := s.GetTables(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	// Get footnotes
	footnotes, err := s.GetFootnotes(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get footnotes: %w", err)
	}

	// Get endnotes
	endnotes, err := s.GetEndnotes(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get endnotes: %w", err)
	}

	// Construct and return ParsedItem
	return &models.ParsedItem{
		Metadata:    *metadata,
		Pages:       pages,
		PageNumbers: pageNumbers,
		References:  references,
		Images:      images,
		Tables:      tables,
		Footnotes:   footnotes,
		Endnotes:    endnotes,
	}, nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Ensure SQLiteStore implements Store interface
var _ Store = (*SQLiteStore)(nil)
