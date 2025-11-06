# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an MCP (Model Context Protocol) server for academic research that provides tools for parsing and analyzing academic PDFs. The server is written in Go and uses OpenAI's vision capabilities to extract structured data from academic papers.

## Build and Development Commands

```bash
# Build all binaries (outputs to bin/ directory)
make build

# Run tests
make test

# Clean build artifacts
make clean

# Add local MCP server to Claude Code configuration
make cc-add-mcp

# Run MCP inspector for debugging
make inspect
```

**Important**: This project requires `GOEXPERIMENT=jsonv2` which is set automatically in the Makefile.

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
   - A tool definition function (e.g., `PDFParseTool()`) that returns `*mcp.Tool` with JSON schema
   - A handler function (e.g., `PDFParseToolHandler()`) that processes requests
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
   - `internal/pdf/`: PDF utilities (splitting, fetching from URL/Zotero)
   - `internal/operations/`: Shared business logic used across multiple tools

7. **Models Layer** (`models/models.go`): Shared data structures used across all layers.

### PDF Parsing Flow

The `pdf-parse` tool supports three mutually exclusive input methods:
- **`zotero_id`**: Fetches PDF from Zotero library (requires `ZOTERO_API_KEY` and `ZOTERO_LIBRARY_ID` env vars)
- **`url`**: Downloads PDF from a URL
- **`raw_data`**: Accepts base64-encoded PDF data directly

The parsing process:
1. Retrieves PDF data from one of the three sources
2. Splits PDF into individual pages using `pdfcpu` library
3. Processes pages **in parallel** with goroutines (see `internal/llm/openai.go:ParseDocument`)
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

### pdf-parse
Parses a PDF document and extracts structured data including metadata, content, references, images, tables, footnotes, and endnotes. The parsed document is stored in SQLite and accessible via resource URIs.

**Input Parameters** (mutually exclusive):
- `zotero_id`: Fetch PDF from Zotero library
- `url`: Download PDF from URL
- `raw_data`: Base64-encoded PDF bytes

**Returns**: Document ID and resource URIs for accessing parsed content.

### pdf-summarize
Generates a concise 1-3 paragraph summary of a PDF document using GPT-5 Mini. If the document hasn't been parsed yet, it will automatically parse it first using `GetOrParsePDF()`. The summary uses a detached academic tone and expository prose.

**Input Parameters** (mutually exclusive):
- `zotero_id`: Fetch PDF from Zotero library
- `url`: Download PDF from URL
- `raw_data`: Base64-encoded PDF bytes

**Returns**: Document ID, resource URIs, document title, and generated summary.

### Shared Operations

Both tools use the `internal/operations/GetOrParsePDF()` function, which:
1. Generates a document ID from the source information
2. Checks if the document already exists in storage
3. If it exists, retrieves it; otherwise parses and stores it
4. Returns the document ID and parsed item

This pattern ensures documents are only parsed once and can be efficiently reused across multiple tools.

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

### Using GetOrParsePDF for Tool Development

When creating tools that need parsed PDF documents, use `internal/operations/GetOrParsePDF()` to avoid duplicate parsing:

```go
import "github.com/Epistemic-Technology/academic-mcp/internal/operations"

func MyToolHandler(ctx context.Context, req *mcp.CallToolRequest, query MyQuery, store storage.Store) (*mcp.CallToolResult, *MyResponse, error) {
    // GetOrParsePDF handles fetching, parsing, and storage automatically
    docID, parsedItem, err := operations.GetOrParsePDF(ctx, query.ZoteroID, query.URL, query.RawData, store)
    if err != nil {
        return nil, nil, err
    }
    
    // Use parsedItem for your tool's specific functionality
    // ...
}
```

This pattern:
- Automatically generates consistent document IDs
- Checks if the document exists before parsing
- Parses and stores new documents
- Returns existing documents from storage
- Handles all three input methods (Zotero, URL, raw data)

### Adding New Storage Methods

If you need to add new storage capabilities:

1. Add method signature to `Store` interface in `internal/storage/storage.go`
2. Implement method in `internal/storage/sqlite.go` for `SQLiteStore`
3. Update schema in `initSchema()` if new tables are needed
4. Add corresponding resource handler methods in `resources/pdf-resources.go` if exposing via URIs
5. Update `CalculateResourcePaths()` in `internal/storage/resources.go` if adding new resource URI patterns

## Environment Variables

Required for PDF parsing:
- `OPENAI_API_KEY`: OpenAI API key (required for all PDF operations)
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

## Testing with MCP Inspector

Use `make inspect` to launch the MCP inspector, which provides an interactive UI for testing tools during development. This is particularly useful for:
- Debugging JSON schema issues in tool definitions
- Testing tool handlers with different inputs
- Verifying resource URI patterns and responses
- Inspecting structured output from OpenAI API
