package server

import (
	"github.com/Epistemic-Technology/academic-mcp/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func CreateServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "academic-mcp", Version: "v0.0.1"}, nil)

	mcp.AddTool(server, tools.PDFParseTool(), tools.PDFParseToolHandler)
	return server
}
