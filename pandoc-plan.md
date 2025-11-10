# Citation and Bibliography Generation Implementation Plan

## Overview
Add support for generating pandoc-style citekeys for parsed documents and processing documents containing citations into formatted output with bibliographies.

## Phase 1: Citekey Generation and Storage

### 1.1 Extend Data Model (models/models.go)
- Add `Citekey` field to `ItemMetadata` struct
- Citekeys will follow format: `author(s)Year` (e.g., `smith2020`, `smithJones2021`)
- For multiple authors (>2), use first author + "EtAl" (e.g., `smithEtAl2020`)
- Handle edge cases: missing authors, missing dates, duplicates

### 1.2 Update Storage Layer (internal/storage/)
- Add `citekey` column to `documents` table in SQLite schema
- Update `StoreParsedItem()` to save citekey
- Update `GetMetadata()` to return citekey
- Add `GetCitekeyMap()` method to retrieve all docID→citekey mappings
- Add `GetDocumentByCitekey()` method for reverse lookup

### 1.3 Implement Citekey Generation (new file: internal/citations/citekey.go)
- `GenerateCitekey(metadata *ItemMetadata, existingCitekeys map[string]bool) string`
- Extract first author's last name, handle special characters
- Append year from publication date
- Handle duplicates by adding suffix (a, b, c, etc.)
- Ensure pandoc compatibility (alphanumerics, underscores, internal punctuation)

### 1.4 Update Document Parsing Flow (internal/operations/operations.go)
- After parsing/retrieving document, generate citekey if not present
- Check for duplicate citekeys in storage
- Store citekey with document metadata

### 1.5 Expose Citekeys in Tools and Resources
- Update `document-parse` tool response to include citekey
- Update metadata resource handler to include citekey in output
- Update `zotero-search` to show citekeys for already-parsed documents

## Phase 2: BibTeX Export

### 2.1 Implement BibTeX Generator (new file: internal/citations/bibtex.go)
- `GenerateBibTeXEntry(docID string, metadata *ItemMetadata, citekey string) string`
- Map `ItemType` to BibTeX entry types (@article, @book, etc.)
- Format all metadata fields according to BibTeX spec
- Handle special characters and escaping

### 2.2 Create BibTeX Export Tool (new file: tools/bibliography-export.go)
- **Tool**: `bibliography-export`
- **Parameters**: 
  - `document_ids` (optional): Export specific documents
  - `format`: "bibtex" (expandable to other formats later)
- **Output**: Complete BibTeX file content
- If no document_ids specified, export entire library

## Phase 3: Citation Processing

### 3.1 Implement Citation Processor (new file: internal/citations/processor.go)
- `ProcessCitations(ctx context.Context, content string, store Store, cslStyle string) (string, error)`
- Parse document to find all `[@citekey]` patterns
- Look up citekeys in storage to get full metadata
- Use `pandoc --citeproc` as subprocess to format citations
- Generate bibliography section
- Return processed document

### 3.2 Create Document Processing Tool (new file: tools/document-process-citations.go)
- **Tool**: `document-process-citations`
- **Parameters**:
  - `content`: Markdown content with pandoc citations
  - `csl_style` (optional): CSL style file path/name (default: APA)
  - `output_format`: "markdown", "html", "docx", etc.
- **Output**: Processed document with formatted citations and bibliography
- Validates that all cited documents exist in storage
- Returns warnings for missing citekeys

## Phase 4: Testing and Documentation

### 4.1 Add Tests
- Unit tests for citekey generation (collisions, special chars, edge cases)
- Integration tests for BibTeX export
- Integration tests for citation processing (requires pandoc)
- Add `-short` flag exclusions for pandoc-dependent tests

### 4.2 Update Documentation
- Update CLAUDE.md with new tools and features
- Add examples of citekey generation
- Document citation processing workflow
- Add examples to README

## Implementation Notes

### Dependencies
- Existing: All Go stdlib + current dependencies
- New: Will shell out to `pandoc --citeproc` for citation formatting

### Citekey Collision Handling
- Generate base citekey from author+year
- Check storage for existing citekeys
- Append letter suffix (a, b, c) if collision detected
- Store collision mapping for consistency

### Integration Points
- Zotero items already have rich metadata for BibTeX export
- Document IDs remain primary keys; citekeys are secondary identifiers
- Resource URIs stay the same (based on docID, not citekey)

### Future Enhancements (not in this plan)
- CSL-JSON export format
- In-memory citation database for non-parsed documents
- Direct CSL processor integration (avoiding pandoc dependency)
- Auto-import BibTeX files to populate citation database
