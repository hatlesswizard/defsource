// Package search provides ranking, formatting, and token-counting utilities
// for documentation query results returned by the defSource library.
package search

import "strings"

// ApproxTokenCount returns an approximate token count for the given text.
// It multiplies the whitespace-delimited word count by 1.5 to account for
// the higher token-per-word ratio typical of code-heavy documentation (PHP
// signatures, type declarations, and source snippets tokenize more densely
// than prose). The result is an approximation suitable for token-budget
// gating; it is not calibrated to any specific LLM tokeniser.
func ApproxTokenCount(text string) int {
	words := len(strings.Fields(text))
	return int(float64(words) * 1.5)
}
