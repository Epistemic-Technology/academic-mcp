# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an MCP (Model Context Protocol) server for academic research that provides tools for parsing and analyzing academic documents in multiple formats (PDF, HTML, Markdown, plain text, and DOCX). The server is written in Go and uses OpenAI's vision capabilities to extract structured data from academic papers.

## Build and Development Commands

```bash
# Build all binaries (outputs to bin/ directory)
make build

# Run short tests (default - excludes OpenAI integration tests)
make test
go test -short ./...

# Run all tests including OpenAI integration tests
go test ./...

# Clean build artifacts
make clean

# Add local MCP server to Claude Code configuration
make cc-add-mcp

# Run MCP inspector for debugging
make inspect
```

**Important**: 
- This project requires `GOEXPERIMENT=jsonv2` which is set automatically in the Makefile.
- **Always run short tests by default** (`go test -short ./...`) to avoid making real OpenAI API calls during development. Only run full integration tests when specifically needed to verify OpenAI API integration.

## Architecture

### Layered Architecture

The codebase follows a clean layered architecture:

1. **Entry Point** (`cmd/academic-mcp-local-server/main.go`): Minimal main function that creates the server and runs it with stdio transport.

2. **Server Layer** (`server/server.go`): 
   - Defines the MCP server implementation using `mcp.NewServer()`
   - Registers all available tools via `mcp.AddTool()`
   - Initializes storage backend (SQLite by default)
   - Registers resource templates for accessing parsed PDF content via URIs
   - Handles storage initialization at `~/.academic-mcp/academic.db` (configurable via `ACADEMIC_MCP_DB_PATH`)

3. **Tools Layer** (`tools/`): Contains MCP tool implementations. Each tool must provide:
   - A tool definition function (e.g., `DocumentParseTool()`) that returns `*mcp.Tool` with JSON schema
   - A handler function (e.g., `DocumentParseToolHandler()`) that processes requests
   - Query and Response types for structured input/output

4. **Storage Layer** (`internal/storage/`):
   - `storage.go`: Defines the `Store` interface for all storage operations
   - `sqlite.go`: SQLite implementation with document storage, retrieval, and indexing
   - `resources.go`: Helper functions like `CalculateResourcePaths()` for generating resource URIs
   - Stores metadata, pages, references, images, tables, footnotes, and endnotes in separate normalized tables
   - Generates document IDs based on Zotero ID, URL hash, or PDF data hash (in priority order)
   - Provides methods for checking document existence and retrieving complete parsed items

5. **Resources Layer** (`resources/`):
   - `PDFResourceHandler` translates URI patterns to storage queries
   - Supports hierarchical URIs like `pdf://{docID}/pages/{pageIndex}`
   - Returns JSON-formatted content for all resource types

6. **Internal Packages**:
   - `internal/llm/`: OpenAI API integration using Responses API with structured outputs (parsing and summarization)
   - `internal/documents/`: Document utilities including:
     - PDF splitting using pdfcpu library
     - Document type detection from magic bytes/headers
     - Fetching documents from URL/Zotero
     - Zotero web archive (ZIP) extraction
     - HTML-to-markdown conversion (`PreprocessHTML()`) to reduce context window usage
   - `internal/operations/`: Shared business logic used across multiple tools
   - `internal/logger/`: Logging infrastructure for the server

7. **Models Layer** (`models/models.go`): Shared data structures used across all layers.

### Document Parsing Flow

The `document-parse` tool supports multiple input methods and document types:

**Input Methods** (mutually exclusive):
- **`zotero_id`**: Fetches document from Zotero library (requires `ZOTERO_API_KEY` and `ZOTERO_LIBRARY_ID` env vars)
  - Automatically detects document type
  - Handles Zotero web archive ZIP files by extracting HTML content
- **`url`**: Downloads document from a URL
- **`raw_data`**: Accepts raw document bytes directly
- **`doc_type`**: Optional parameter to override automatic type detection (e.g., "pdf", "html", "md", "txt")

**Supported Document Types**:
- **PDF**: Uses vision-based extraction with page splitting
- **HTML**: Single-pass parsing with vision model
- **Markdown**: Single-pass parsing optimized for text extraction
- **Plain Text**: Single-pass parsing optimized for text extraction
- **DOCX**: Planned (not yet implemented)

**Document Type Detection**:
The system automatically detects document types by examining magic bytes and headers:
- PDF: `%PDF` signature
- HTML: DOCTYPE or `<html>` tags
- Markdown: Common markdown patterns (`#`, `` ``` ``)
- ZIP-based formats: Checks for DOCX or Zotero web archives
- Plain text: Valid UTF-8 with high proportion of printable characters

**PDF Parsing Process** (most complex):
1. Retrieves document data from one of the three sources
2. Splits PDF into individual pages using `pdfcpu` library
3. Processes pages **in parallel** with goroutines (see `internal/llm/openai.go:parsePDF`)
4. For each page, sends to OpenAI Responses API with GPT-5 Mini model
5. Uses structured output (JSON schema) to extract per-page data, including:
   - Document metadata (title, authors, DOI, etc.)
   - Main text content
   - References, images, tables, footnotes, and endnotes
   - **Page numbering information** (printed page numbers with confidence scores)
6. Validates detected page numbers with conservative heuristics:
   - Requires 60%+ coverage with high confidence (≥0.7)
   - Checks for monotonicity (allowing small gaps for unnumbered pages)
   - Interpolates missing page numbers where possible
   - Falls back to sequential 1-n numbering if validation fails
7. Aggregates results from all pages into a single `models.ParsedItem`
8. Stores in SQLite database with both sequential and source page numbers
9. Returns document ID and resource URIs for accessing content

**HTML/Markdown/Text Parsing Process**:
1. Retrieves document data from source
2. **For HTML documents**: Converts HTML to markdown using `github.com/JohannesKaufmann/html-to-markdown/v2` to reduce context window usage (typically 5-10x reduction). This strips scripts, styles, images, and unnecessary markup while preserving document structure (headings, lists, tables, links).
3. Sends document content (markdown-converted HTML, or original markdown/text) to OpenAI API in single request
4. Extracts structured data (metadata, content, references, images, tables)
5. Page numbering fields remain empty for non-PDF documents
6. Stores in SQLite database
7. Returns document ID and resource URIs

### Page Numbering System

The system intelligently detects and uses source page numbers (e.g., journal article pages 125-150) when reliable:

**Detection Strategy:**
- LLM extracts printed page numbers from headers, footers, and margins
- Provides confidence scores (0.0-1.0) for each detection
- Captures page range information from title pages

**Validation (Conservative Approach):**
- Only uses source numbers if 60%+ of pages have confident detections
- Verifies monotonic increase with tolerance for unnumbered pages
- Allows up to 20% violations (e.g., for chapter breaks)
- Prefers false negatives over false positives

**Storage:**
- `pages` table stores both `page_number` (sequential 1-n) and `source_page_number`
- Sequential numbers guarantee stable internal references
- Source numbers enable natural academic references

**Access Patterns:**
- `GetPage(docID, n)` - Get page by sequential number (1-indexed)
- `GetPageBySourceNumber(docID, "125")` - Get page by source number
- `GetPageMapping(docID)` - Get mapping between source and sequential numbers

See `internal/llm/openai.go:validatePageNumbers()` for validation logic.

### Resource URI System

After parsing, document content is accessible via standardized URIs:
- `pdf://{docID}` - Document summary with counts
- `pdf://{docID}/metadata` - Title, authors, DOI, abstract, etc.
- `pdf://{docID}/pages` - All page content with both sequential and source page numbers
- `pdf://{docID}/pages/{sourcePageNumber}` - Specific page by source number (e.g., `pages/125` for journal page 125)
- `pdf://{docID}/references` - All bibliographic references
- `pdf://{docID}/references/{refIndex}` - Specific reference (0-indexed)
- `pdf://{docID}/images` - All images with captions
- `pdf://{docID}/images/{imageIndex}` - Specific image (0-indexed)
- `pdf://{docID}/tables` - All tables with structured data
- `pdf://{docID}/tables/{tableIndex}` - Specific table (0-indexed)
- `pdf://{docID}/footnotes` - All footnotes from the document
- `pdf://{docID}/footnotes/{footnoteIndex}` - Specific footnote (0-indexed)
- `pdf://{docID}/endnotes` - All endnotes from the document
- `pdf://{docID}/endnotes/{endnoteIndex}` - Specific endnote (0-indexed)

**Note:** Pages are accessed by their source page numbers (when detected) rather than sequential indices. For example, if a journal article spans pages 125-150, use `pdf://{docID}/pages/125` not `pdf://{docID}/pages/0`. The `/pages` resource shows the mapping between source and sequential numbers.

**Footnotes vs Endnotes:** Footnotes appear at the bottom of the page where their marker is referenced, while endnotes are collected in a dedicated section at the end of chapters or documents. The LLM distinguishes between these during parsing.

## Available Tools

### document-parse
Parses a document (PDF, HTML, Markdown, plain text, or DOCX) and extracts structured data including metadata, content, references, images, tables, footnotes, and endnotes. The parsed document is stored in SQLite and accessible via resource URIs.

**Input Parameters** (mutually exclusive):
- `zotero_id`: Fetch document from Zotero library (auto-detects type, handles web archives)
- `url`: Download document from URL
- `raw_data`: Raw document bytes
- `doc_type`: Optional type override (e.g., "pdf", "html", "md", "txt")

**Returns**: Document ID, resource URIs, title, and content statistics (page count, reference count, etc.).

### document-summarize
Generates a concise 1-3 paragraph summary of a document using GPT-5 Mini. If the document hasn't been parsed yet, it will automatically parse it first using `GetOrParseDocument()`. The summary uses a detached academic tone and expository prose. Supports all document types (PDF, HTML, Markdown, plain text).

**Input Parameters** (mutually exclusive):
- `zotero_id`: Fetch document from Zotero library
- `url`: Download document from URL
- `raw_data`: Raw document bytes
- `doc_type`: Optional type override

**Returns**: Document ID, resource URIs, document title, and generated summary.

### document-quotations
Extracts representative quotations from a document (PDF, HTML, Markdown, plain text, or DOCX). The document is parsed and summarized first, then an LLM identifies significant quotations with page numbers (for paginated documents). Supports all document types. Use `max_quotations` to limit results (default: 10, 0 = unlimited).

**Input Parameters** (mutually exclusive):
- `zotero_id`: Fetch document from Zotero library
- `url`: Download document from URL
- `raw_data`: Raw document bytes
- `doc_type`: Optional type override
- `max_quotations`: Maximum number of quotations to extract (default: 10)

**Returns**: Document ID, resource URIs, document title, and list of significant quotations with page numbers and relevance explanations.

### zotero-search
Searches for items in a Zotero library and retrieves their metadata and attachment information. This tool provides a user-friendly way to discover documents in your Zotero library before parsing them. Returns bibliographic items (books, articles, etc.) along with their associated file attachments (PDFs, etc.).

**Input Parameters**:
- `query`: Quick search text (searches title, creator, year)
- `tags`: Filter by tags (array of strings)
- `item_types`: Filter by item type (e.g., "book", "article"); prefix with "-" to exclude (e.g., "-attachment")
- `collection`: Filter by collection key (optional) - restricts search to items within a specific collection
- `limit`: Maximum number of results (default: 25)
- `sort`: Sort field (default: "dateModified")

**Returns**: Array of items with:
- `key`: Item key for the bibliographic entry
- `title`: Item title
- `creators`: Array of creator names (authors, editors, etc.)
- `item_type`: Type of item (book, article, etc.)
- `date`: Date added to library
- `attachments`: Array of attachment information:
  - `key`: Attachment key (use this as `zotero_id` in `document-parse`)
  - `filename`: Name of the attached file
  - `content_type`: MIME type (e.g., "application/pdf")
  - `link_mode`: How the file is attached (imported_file, imported_url, etc.)

**Typical Workflow**:
```
1. Use zotero-collections to find a collection key:
   top_level_only=true
   
2. Use zotero-search to find items within that collection:
   collection="ABC123XYZ", query="climate change adaptation", limit=10
   
3. Review results to find the item you want

4. Use the attachment key from the results in document-parse:
   zotero_id="DEF456UVW" (from attachments[0].key)
```

**Note**: This tool requires `ZOTERO_API_KEY` and `ZOTERO_LIBRARY_ID` environment variables to be set.

### zotero-collections
Lists and searches collections in a Zotero library. This tool helps you browse your library's organizational structure and find collection keys for filtering or organizing items.

**Input Parameters**:
- `top_level_only`: Boolean - list only top-level collections (no parent) (default: false)
- `parent_collection`: String - filter by parent collection key to get subcollections
- `limit`: Maximum number of results (default: 100)
- `sort`: Sort field (default: "title")

**Returns**: Array of collections with:
- `key`: Collection key (unique identifier)
- `name`: Collection name
- `parent_collection`: Parent collection key (empty if top-level)
- `count`: Total number of collections returned

**Typical Workflow**:
```
1. List all collections to browse library structure:
   (no parameters - returns all collections)
   
2. List only top-level collections:
   top_level_only=true
   
3. Get subcollections of a specific collection:
   parent_collection="ABC123XYZ"
```

**Note**: This tool requires `ZOTERO_API_KEY` and `ZOTERO_LIBRARY_ID` environment variables to be set.

### Shared Operations

Both tools use the `internal/operations/GetOrParseDocument()` function, which:
1. Retrieves document data from the specified source (Zotero, URL, or raw data)
2. Auto-detects document type or uses provided `doc_type` parameter
3. Generates a document ID from the source information and content hash
4. Checks if the document already exists in storage
5. If it exists, retrieves it; otherwise parses and stores it
6. Returns the document ID and parsed item

This pattern ensures documents are only parsed once and can be efficiently reused across multiple tools. The legacy `GetOrParsePDF()` function still exists as a convenience wrapper that forces the type to "pdf".

### Adding New Tools

To add a new MCP tool:

1. Create tool file in `tools/` directory (e.g., `tools/pdf-summarize.go`)
2. Define Query and Response types with JSON tags
3. Implement tool definition function:
   ```go
   func MyTool() *mcp.Tool {
       schema, err := jsonschema.For[MyQuery](nil)
       if err != nil { panic(err) }
       return &mcp.Tool{
           Name: "my-tool",
           Description: "Tool description",
           InputSchema: schema,
       }
   }
   ```
4. Implement handler function with signature:
   ```go
   func MyToolHandler(ctx context.Context, req *mcp.CallToolRequest, query MyQuery, store storage.Store) (*mcp.CallToolResult, *MyResponse, error)
   ```
5. Register in `server/server.go`:
   ```go
   mcp.AddTool(server, tools.MyTool(), func(ctx context.Context, req *mcp.CallToolRequest, query tools.MyQuery) (*mcp.CallToolResult, *tools.MyResponse, error) {
       return tools.MyToolHandler(ctx, req, query, store)
   })
   ```

### Using GetOrParseDocument for Tool Development

When creating tools that need parsed documents, use `internal/operations/GetOrParseDocument()` to avoid duplicate parsing:

```go
import "github.com/Epistemic-Technology/academic-mcp/internal/operations"

func MyToolHandler(ctx context.Context, req *mcp.CallToolRequest, query MyQuery, store storage.Store, log logger.Logger) (*mcp.CallToolResult, *MyResponse, error) {
    // GetOrParseDocument handles fetching, parsing, and storage automatically
    docID, parsedItem, err := operations.GetOrParseDocument(ctx, query.ZoteroID, query.URL, query.RawData, query.DocType, store, log)
    if err != nil {
        return nil, nil, err
    }
    
    // Use parsedItem for your tool's specific functionality
    // ...
}
```

This pattern:
- Automatically generates consistent document IDs
- Auto-detects document type from content (or uses provided `doc_type`)
- Checks if the document exists before parsing
- Parses and stores new documents
- Returns existing documents from storage
- Handles all input methods (Zotero, URL, raw data)
- Supports multiple document formats (PDF, HTML, Markdown, plain text)

### Adding New Storage Methods

If you need to add new storage capabilities:

1. Add method signature to `Store` interface in `internal/storage/storage.go`
2. Implement method in `internal/storage/sqlite.go` for `SQLiteStore`
3. Update schema in `initSchema()` if new tables are needed
4. Add corresponding resource handler methods in `resources/pdf-resources.go` if exposing via URIs
5. Update `CalculateResourcePaths()` in `internal/storage/resources.go` if adding new resource URI patterns

## Environment Variables

Required for document parsing:
- `OPENAI_API_KEY`: OpenAI API key (required for all document parsing operations)
- `ZOTERO_API_KEY`: Zotero API key (only required when using `zotero_id` parameter)
- `ZOTERO_LIBRARY_ID`: Zotero library ID (only required when using `zotero_id` parameter)
- `ACADEMIC_MCP_DB_PATH`: Optional path to SQLite database (defaults to `~/.academic-mcp/academic.db`)

## Key Dependencies

- `github.com/modelcontextprotocol/go-sdk` - MCP protocol implementation
- `github.com/openai/openai-go/v3` - OpenAI API client using Responses API for structured outputs
- `github.com/Epistemic-Technology/zotero` - Zotero API client
- `github.com/google/jsonschema-go` - JSON schema generation for tool inputs
- `github.com/pdfcpu/pdfcpu` - PDF processing and page extraction
- `github.com/mattn/go-sqlite3` - SQLite driver for persistent storage
- `github.com/JohannesKaufmann/html-to-markdown/v2` - HTML-to-markdown conversion for reducing context window usage

## Testing with MCP Inspector

Use `make inspect` to launch the MCP inspector, which provides an interactive UI for testing tools during development. This is particularly useful for:
- Debugging JSON schema issues in tool definitions
- Testing tool handlers with different inputs
- Verifying resource URI patterns and responses
- Inspecting structured output from OpenAI API
