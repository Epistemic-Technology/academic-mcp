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
rm -rf bin/

# Add local MCP server to Claude Code configuration
make cc-add-mcp

# Run MCP inspector for debugging
make inspect
```

**Important**: This project requires `GOEXPERIMENT=jsonv2` which is set automatically in the Makefile.

## Architecture

### MCP Server Structure

The codebase follows a layered architecture:

1. **Entry Point** (`cmd/academc-mcp-local-server/main.go`): Minimal main function that creates the server and runs it with stdio transport.

2. **Server Layer** (`server/server.go`): Defines the MCP server implementation and registers all available tools. Currently registers `pdf-parse` tool.

3. **Tools Layer** (`tools/`): Contains individual MCP tool implementations. Each tool must provide:
   - A tool definition function (e.g., `PDFParseTool()`) that returns `*mcp.Tool` with schema
   - A handler function (e.g., `PDFParseToolHandler()`) that processes requests
   - Query and Response types for structured input/output

4. **Models Layer** (`models/models.go`): Shared data structures used across tools, particularly for parsed PDF content.

### PDF Parsing Flow

The `pdf-parse` tool supports three input methods:
- **Zotero ID**: Fetches PDF from Zotero library (requires `ZOTERO_API_KEY` and `ZOTERO_LIBRARY_ID` env vars)
- **URL**: Downloads PDF from a URL
- **Raw Data**: Accepts base64-encoded PDF data directly

The parsing process:
1. Retrieves PDF data from one of the three sources
2. Encodes PDF as base64
3. Sends to OpenAI Responses API with GPT-5 Mini model
4. Uses structured output (JSON schema) to extract:
   - Metadata (title, authors, publication info, DOI, abstract)
   - Page-by-page text content
   - References with DOIs
   - Images with captions
   - Tables with structured data
5. Returns parsed content as `models.ParsedItem`

### Adding New Tools

To add a new MCP tool:

1. Create tool file in `tools/` directory
2. Define Query and Response types
3. Implement tool definition function returning `*mcp.Tool` with JSON schema
4. Implement handler function with signature: `func(context.Context, *mcp.CallToolRequest, QueryType) (*mcp.CallToolResult, *ResponseType, error)`
5. Register tool in `server/server.go` using `mcp.AddTool()`

Example: `tools/pdf-summarize.go` has stub types but is not yet implemented.

## Environment Variables

Required for PDF parsing with Zotero:
- `OPENAI_API_KEY`: OpenAI API key (required for all PDF operations)
- `ZOTERO_API_KEY`: Zotero API key (only required when using `zotero_id` parameter)
- `ZOTERO_LIBRARY_ID`: Zotero library ID (only required when using `zotero_id` parameter)

## Dependencies

Key dependencies:
- `github.com/modelcontextprotocol/go-sdk` - MCP protocol implementation
- `github.com/openai/openai-go/v3` - OpenAI API client using Responses API
- `github.com/Epistemic-Technology/zotero` - Zotero API client
- `github.com/google/jsonschema-go` - JSON schema generation for tool inputs

## Testing with MCP Inspector

Use `make inspect` to launch the MCP inspector, which provides an interactive UI for testing tools during development. This is particularly useful for debugging JSON schema issues and response formats.
