.PHONY: build clean test help cc-add-mcp inspect

# Go parameters
GOEXPERIMENT := jsonv2
BINARY_DIR := bin

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

build: ## Build all binaries
	GOEXPERIMENT=$(GOEXPERIMENT) go build -o $(BINARY_DIR)/ ./cmd/...

clean: ## Remove build artifacts
	rm -rf $(BINARY_DIR)

test: ## Run tests
	go test ./...

cc-add-mcp: ## Add local MCP server to Claude Code
	@echo "Adding academic-mcp server to Claude Code configuration..."
	@BINARY_PATH=$$(pwd)/$(BINARY_DIR)/academc-mcp-local-server; \
	if [ ! -f "$$BINARY_PATH" ]; then \
		echo "Binary not found. Building first..."; \
		$(MAKE) build; \
	fi; \
	claude mcp add academic-mcp --scope project -- "$$BINARY_PATH"

inspect: ## Run the MCP inspector on local server
	npx @modelcontextprotocol/inspector $(PWD)/$(BINARY_DIR)/academc-mcp-local-server
