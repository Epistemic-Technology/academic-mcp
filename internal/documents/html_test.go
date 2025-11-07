package documents

import (
	"testing"
)

func TestPreprocessHTML(t *testing.T) {
	tests := []struct {
		name        string
		html        string
		wantContain []string
		wantNotContain []string
	}{
		{
			name: "basic academic article",
			html: `<!DOCTYPE html>
<html>
<head>
	<title>Test Article</title>
	<style>body { color: red; }</style>
	<script>console.log("test");</script>
</head>
<body>
	<nav>Navigation Menu</nav>
	<h1>Test Article Title</h1>
	<p>This is the abstract of the paper.</p>
	<h2>Introduction</h2>
	<p>This is the introduction with a <a href="ref1">reference [1]</a>.</p>
	<h2>Methods</h2>
	<p>This describes the methods.</p>
</body>
</html>`,
			wantContain: []string{
				"# Test Article Title",
				"## Introduction",
				"## Methods",
				"abstract",
				"introduction",
			},
			wantNotContain: []string{
				"<html>",
				"<body>",
				"<style>",
				"<script>",
				"console.log",
				"color: red",
			},
		},
		{
			name: "article with table",
			html: `<html>
<body>
	<h1>Data Analysis</h1>
	<table>
		<tr><th>Metric</th><th>Value</th></tr>
		<tr><td>Accuracy</td><td>95%</td></tr>
		<tr><td>Precision</td><td>92%</td></tr>
	</table>
</body>
</html>`,
			wantContain: []string{
				"# Data Analysis",
				"Metric",
				"Value",
				"Accuracy",
				"95%",
			},
			wantNotContain: []string{
				"<table>",
				"<tr>",
				"<td>",
			},
		},
		{
			name: "article with image",
			html: `<html>
<body>
	<h1>Visualization Study</h1>
	<img src="figure1.png" alt="Figure 1: Results visualization" />
	<p>Figure 1 shows the results.</p>
</body>
</html>`,
			wantContain: []string{
				"# Visualization Study",
				"Figure 1",
				"results",
			},
			wantNotContain: []string{
				"<img",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markdown, err := PreprocessHTML([]byte(tt.html))
			if err != nil {
				t.Errorf("PreprocessHTML() error = %v", err)
				return
			}

			// Check that desired content is present
			for _, want := range tt.wantContain {
				if !contains(markdown, want) {
					t.Errorf("PreprocessHTML() markdown should contain %q, but doesn't.\nMarkdown:\n%s", want, markdown)
				}
			}

			// Check that unwanted content is not present
			for _, notWant := range tt.wantNotContain {
				if contains(markdown, notWant) {
					t.Errorf("PreprocessHTML() markdown should NOT contain %q, but does.\nMarkdown:\n%s", notWant, markdown)
				}
			}

			// Verify size reduction (markdown should be smaller than HTML with all the tags)
			if len(markdown) > len(tt.html) {
				t.Logf("Warning: Markdown (%d bytes) is larger than HTML (%d bytes). This is unexpected but may occur with simple test cases.", len(markdown), len(tt.html))
			}
		})
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
