package llm

import (
	"fmt"

	"github.com/Epistemic-Technology/academic-mcp/models"
)

// pageInfo holds detected page number information for validation
type pageInfo struct {
	number     string
	confidence float64
	index      int
}

// validatePageNumbers analyzes detected page numbers and returns a validated page numbering scheme
// Returns a slice of page number strings (one per page) to use for storage/access
func validatePageNumbers(pages []*models.ParsedPage) []string {
	// Extract detected page numbers with confidence
	var detectedPages []pageInfo
	var pageRangeInfo string

	for i, page := range pages {
		if page != nil {
			info := pageInfo{
				number:     page.PageNumberInfo.PageNumber,
				confidence: page.PageNumberInfo.Confidence,
				index:      i,
			}
			detectedPages = append(detectedPages, info)

			// Capture any page range info from early pages
			if i < 3 && page.PageNumberInfo.PageRangeInfo != "" {
				pageRangeInfo = page.PageNumberInfo.PageRangeInfo
			}
		}
	}

	// Try to parse and validate source page numbers
	if useSourceNumbers(detectedPages, pageRangeInfo) {
		return extractSourceNumbers(detectedPages)
	}

	// Fallback to sequential 1-n numbering
	result := make([]string, len(pages))
	for i := range pages {
		result[i] = fmt.Sprintf("%d", i+1)
	}
	return result
}

// useSourceNumbers determines if we should use source page numbers based on validation
func useSourceNumbers(pages []pageInfo, rangeInfo string) bool {
	const minConfidence = 0.7
	const minCoverageRatio = 0.6 // At least 60% of pages must have numbers

	// Count pages with confident numbers
	confidenPages := 0
	for _, p := range pages {
		if p.confidence >= minConfidence && p.number != "" {
			confidenPages++
		}
	}

	// Need sufficient coverage
	coverageRatio := float64(confidenPages) / float64(len(pages))
	if coverageRatio < minCoverageRatio {
		return false
	}

	// Parse numbers and check for valid sequence
	parsedNumbers := make(map[int]int) // index -> numeric page number
	hasNumericSequence := false

	for _, p := range pages {
		if p.confidence >= minConfidence && p.number != "" {
			// Try to parse as integer
			var num int
			_, err := fmt.Sscanf(p.number, "%d", &num)
			if err == nil && num > 0 {
				parsedNumbers[p.index] = num
				hasNumericSequence = true
			}
		}
	}

	if !hasNumericSequence {
		return false
	}

	// Check for monotonicity with allowance for gaps
	return isMonotonic(parsedNumbers)
}

// isMonotonic checks if page numbers generally increase, allowing for small gaps
func isMonotonic(numbers map[int]int) bool {
	if len(numbers) < 2 {
		return false
	}

	// Extract sorted indices
	indices := make([]int, 0, len(numbers))
	for idx := range numbers {
		indices = append(indices, idx)
	}

	// Simple bubble sort for small slices
	for i := 0; i < len(indices)-1; i++ {
		for j := 0; j < len(indices)-i-1; j++ {
			if indices[j] > indices[j+1] {
				indices[j], indices[j+1] = indices[j+1], indices[j]
			}
		}
	}

	// Check that page numbers increase with document order
	// Allow for gaps of up to 3 (for unnumbered pages)
	prevPageNum := numbers[indices[0]]
	violations := 0

	for i := 1; i < len(indices); i++ {
		currPageNum := numbers[indices[i]]
		expectedMin := prevPageNum + 1
		expectedMax := prevPageNum + 4 // Allow up to 3 unnumbered pages

		if currPageNum < expectedMin || currPageNum > expectedMax {
			violations++
		}

		prevPageNum = currPageNum
	}

	// Allow up to 20% violations (e.g., chapter breaks)
	violationRatio := float64(violations) / float64(len(indices)-1)
	return violationRatio <= 0.2
}

// extractSourceNumbers builds the final page number list from detected numbers
func extractSourceNumbers(pages []pageInfo) []string {
	const minConfidence = 0.7

	result := make([]string, len(pages))

	// First pass: use high-confidence detected numbers
	numberedIndices := make(map[int]bool)
	for _, p := range pages {
		if p.confidence >= minConfidence && p.number != "" {
			result[p.index] = p.number
			numberedIndices[p.index] = true
		}
	}

	// Second pass: interpolate missing numbers if they're between known numbers
	for i := range result {
		if !numberedIndices[i] {
			// Try to find surrounding numbers
			prevIdx := -1
			nextIdx := -1

			for j := i - 1; j >= 0; j-- {
				if numberedIndices[j] {
					prevIdx = j
					break
				}
			}

			for j := i + 1; j < len(result); j++ {
				if numberedIndices[j] {
					nextIdx = j
					break
				}
			}

			// If we have both prev and next, interpolate
			if prevIdx >= 0 && nextIdx >= 0 {
				var prevNum, nextNum int
				_, errPrev := fmt.Sscanf(result[prevIdx], "%d", &prevNum)
				_, errNext := fmt.Sscanf(result[nextIdx], "%d", &nextNum)

				if errPrev == nil && errNext == nil {
					// Calculate expected number
					gap := nextIdx - prevIdx
					expectedGap := nextNum - prevNum

					if gap == expectedGap {
						// Exact interpolation
						offset := i - prevIdx
						result[i] = fmt.Sprintf("%d", prevNum+offset)
						numberedIndices[i] = true
					}
				}
			}
		}
	}

	// Final pass: fallback for any remaining unnumbered pages
	for i := range result {
		if result[i] == "" {
			// Use sequential numbering with prefix to indicate uncertainty
			result[i] = fmt.Sprintf("%d", i+1)
		}
	}

	return result
}
