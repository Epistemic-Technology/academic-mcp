package main

import (
	"context"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Initialize logger with default configuration
	log, err := logger.NewLogger(logger.LogConfig{})
	if err != nil {
		// Fall back to stderr if logger initialization fails
		panic(err)
	}

	log.Info("Starting academic-mcp server")

	srv := server.CreateServer(log)
	err = srv.Run(context.Background(), &mcp.StdioTransport{})
	if err != nil {
		log.Fatal("Server failed: %v", err)
	}
}
