package citations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
	"github.com/Epistemic-Technology/academic-mcp/internal/storage"
)

// ProcessCitations processes a document containing pandoc-style citations [@citekey]
// and returns formatted output with a bibliography section.
// It uses pandoc --citeproc as a subprocess to format citations according to the CSL style.
func ProcessCitations(ctx context.Context, content string, store storage.Store, cslStyle string, outputFormat string, log logger.Logger) (string, []string, error) {
	// Find all citations in the document
	citekeys, err := ExtractCitekeys(content)
	if err != nil {
		return "", nil, fmt.Errorf("failed to extract citekeys: %w", err)
	}

	if len(citekeys) == 0 {
		log.Info("No citations found in document")
		return content, nil, nil
	}

	log.Info("Found %d unique citations: %v", len(citekeys), citekeys)

	// Validate that all cited documents exist in storage
	warnings := []string{}
	existingCitekeys := make(map[string]bool)
	for _, citekey := range citekeys {
		docID, err := store.GetDocumentByCitekey(ctx, citekey)
		if err != nil || docID == "" {
			warning := fmt.Sprintf("Citation @%s not found in library", citekey)
			warnings = append(warnings, warning)
			log.Warn(warning)
		} else {
			existingCitekeys[citekey] = true
			log.Info("Found document for citation @%s: %s", citekey, docID)
		}
	}

	// If no valid citations exist, return early
	if len(existingCitekeys) == 0 {
		return "", warnings, fmt.Errorf("none of the cited documents exist in the library")
	}

	// Generate BibTeX bibliography from cited documents
	bibContent, err := generateBibliographyForCitekeys(ctx, store, existingCitekeys, log)
	if err != nil {
		return "", warnings, fmt.Errorf("failed to generate bibliography: %w", err)
	}

	// Create temporary files for pandoc processing
	tmpDir, err := os.MkdirTemp("", "academic-mcp-citations-")
	if err != nil {
		return "", warnings, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write input markdown file
	inputPath := filepath.Join(tmpDir, "input.md")
	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		return "", warnings, fmt.Errorf("failed to write input file: %w", err)
	}

	// Write bibliography file
	bibPath := filepath.Join(tmpDir, "references.bib")
	if err := os.WriteFile(bibPath, []byte(bibContent), 0644); err != nil {
		return "", warnings, fmt.Errorf("failed to write bibliography file: %w", err)
	}

	// Prepare pandoc command
	outputPath := filepath.Join(tmpDir, "output."+getOutputExtension(outputFormat))
	args := []string{
		inputPath,
		"--citeproc",
		"--bibliography", bibPath,
		"-o", outputPath,
	}

	// Add CSL style if specified
	if cslStyle != "" {
		args = append(args, "--csl", cslStyle)
	}

	// Add format-specific options
	switch strings.ToLower(outputFormat) {
	case "markdown", "md":
		args = append(args, "-t", "markdown")
	case "html":
		args = append(args, "-t", "html")
	case "docx":
		args = append(args, "-t", "docx")
	case "pdf":
		args = append(args, "-t", "pdf", "--pdf-engine=xelatex")
	default:
		args = append(args, "-t", "markdown") // Default to markdown
	}

	log.Info("Running pandoc command: pandoc %s", strings.Join(args, " "))

	// Run pandoc with context support for cancellation
	cmd := exec.CommandContext(ctx, "pandoc", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Pandoc failed: %v\nOutput: %s", err, string(output))
		return "", warnings, fmt.Errorf("pandoc processing failed: %w\nOutput: %s", err, string(output))
	}

	// Read the output file
	processedContent, err := os.ReadFile(outputPath)
	if err != nil {
		return "", warnings, fmt.Errorf("failed to read output file: %w", err)
	}

	log.Info("Successfully processed citations with pandoc")

	return string(processedContent), warnings, nil
}

// ExtractCitekeys finds all pandoc-style citation keys in the document
// Supports formats like [@key], [@key1; @key2], @key (in-text), etc.
func ExtractCitekeys(content string) ([]string, error) {
	// Pandoc citation patterns:
	// [@key] - standard citation
	// [@key1; @key2] - multiple citations
	// @key - in-text citation
	// [-@key] - suppress author
	// [see @key, p. 10] - with prefix/suffix

	// This regex captures the citekey part after @
	re := regexp.MustCompile(`@([a-zA-Z0-9_][a-zA-Z0-9_:.#$%&\-+?<>~/]*)`)

	matches := re.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return []string{}, nil
	}

	// Use map to deduplicate
	citekeysMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			citekey := match[1]
			citekeysMap[citekey] = true
		}
	}

	// Convert map to slice
	citekeys := make([]string, 0, len(citekeysMap))
	for key := range citekeysMap {
		citekeys = append(citekeys, key)
	}

	return citekeys, nil
}

// generateBibliographyForCitekeys creates a BibTeX bibliography containing only the cited documents
func generateBibliographyForCitekeys(ctx context.Context, store storage.Store, citekeys map[string]bool, log logger.Logger) (string, error) {
	var entries []string

	for citekey := range citekeys {
		// Get document ID for this citekey
		docID, err := store.GetDocumentByCitekey(ctx, citekey)
		if err != nil {
			log.Warn("Could not find document for citekey %s: %v", citekey, err)
			continue
		}

		// Get metadata
		metadata, err := store.GetMetadata(ctx, docID)
		if err != nil {
			log.Warn("Could not get metadata for document %s: %v", docID, err)
			continue
		}

		// Generate BibTeX entry
		entry := GenerateBibTeXEntry(docID, metadata, citekey)
		entries = append(entries, entry)
		log.Info("Added BibTeX entry for @%s", citekey)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no valid bibliography entries could be generated")
	}

	return GenerateBibTeXFile(entries), nil
}

// getOutputExtension returns the file extension for the output format
func getOutputExtension(format string) string {
	switch strings.ToLower(format) {
	case "markdown", "md":
		return "md"
	case "html":
		return "html"
	case "docx":
		return "docx"
	case "pdf":
		return "pdf"
	default:
		return "md"
	}
}
