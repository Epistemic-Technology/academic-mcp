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
   - Stores metadata, pages, references, images, and tables in separate normalized tables
   - Generates document IDs based on Zotero ID, DOI, URL hash, or title hash (in priority order)

5. **Resources Layer** (`resources/`):
   - `PDFResourceHandler` translates URI patterns to storage queries
   - Supports hierarchical URIs like `pdf://{docID}/pages/{pageIndex}`
   - Returns JSON-formatted content for all resource types

6. **Internal Packages**:
   - `internal/llm/`: OpenAI API integration using Responses API with structured outputs
   - `internal/pdf/`: PDF utilities (splitting, fetching from URL/Zotero)

7. **Models Layer** (`models/models.go`): Shared data structures used across all layers.

### PDF Parsing Flow

The `pdf-parse` tool supports three mutually exclusive input methods:
- **`zotero_id`**: Fetches PDF from Zotero library (requires `ZOTERO_API_KEY` and `ZOTERO_LIBRARY_ID` env vars)
- **`url`**: Downloads PDF from a URL
- **`raw_data`**: Accepts base64-encoded PDF data directly

The parsing process:
1. Retrieves PDF data from one of the three sources
2. Splits PDF into individual pages using `pdfcpu` library
3. Processes pages **in parallel** with goroutines (see `internal/llm/openai.go:ParsePDF`)
4. For each page, sends to OpenAI Responses API with GPT-5 Mini model
5. Uses structured output (JSON schema) to extract per-page data
6. Aggregates results from all pages into a single `models.ParsedItem`
7. Stores in SQLite database with generated document ID
8. Returns document ID and resource URIs for accessing content

### Resource URI System

After parsing, document content is accessible via standardized URIs:
- `pdf://{docID}` - Document summary with counts
- `pdf://{docID}/metadata` - Title, authors, DOI, abstract, etc.
- `pdf://{docID}/pages` - All page content
- `pdf://{docID}/pages/{pageIndex}` - Specific page (0-indexed)
- `pdf://{docID}/references` - All bibliographic references
- `pdf://{docID}/references/{refIndex}` - Specific reference (0-indexed)
- `pdf://{docID}/images` - All images with captions
- `pdf://{docID}/images/{imageIndex}` - Specific image (0-indexed)
- `pdf://{docID}/tables` - All tables with structured data
- `pdf://{docID}/tables/{tableIndex}` - Specific table (0-indexed)

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

### Adding New Storage Methods

If you need to add new storage capabilities:

1. Add method signature to `Store` interface in `internal/storage/storage.go`
2. Implement method in `internal/storage/sqlite.go` for `SQLiteStore`
3. Update schema in `initSchema()` if new tables are needed
4. Add corresponding resource handler methods in `resources/pdf-resources.go` if exposing via URIs

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
