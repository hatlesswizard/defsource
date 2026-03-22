package search

import "strings"

// ApproxTokenCount returns an approximate token count for the given text.
// Uses a 1.5x multiplier on word count as a rough heuristic for code-heavy content.
func ApproxTokenCount(text string) int {
	words := len(strings.Fields(text))
	return int(float64(words) * 1.5)
}
