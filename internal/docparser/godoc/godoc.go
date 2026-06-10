// Package godoc parses Go doc comments into structured DocComment values.
// Go does not use structured tags like PHPDoc/JavaDoc — instead it relies
// on prose conventions. This parser extracts structure from:
//   - The first paragraph as summary
//   - "Deprecated:" prefix for deprecation notices
//   - Parameter descriptions from prose patterns
package godoc

import (
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
)

var (
	reDeprecated = regexp.MustCompile(`(?m)^Deprecated:\s*(.+)`)
	reSince      = regexp.MustCompile(`(?m)(?:since|added in)\s+(?:Go\s+)?(\d+\.\d+(?:\.\d+)?)`)
)

// Parser implements docparser.Parser for Go doc comments.
type Parser struct{}

// New returns a new Go doc comment parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses a Go doc comment block into a DocComment.
// The raw input should be the comment text with // or /* */ delimiters
// already stripped to bare text.
func (p *Parser) Parse(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}

	// Strip comment markers if still present
	raw = stripCommentMarkers(raw)
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return doc
	}

	doc.Description = raw

	// Summary is the first paragraph
	parts := strings.SplitN(raw, "\n\n", 2)
	doc.Summary = strings.TrimSpace(parts[0])

	// Check for Deprecated: prefix
	if m := reDeprecated.FindStringSubmatch(raw); m != nil {
		doc.Deprecated = strings.TrimSpace(m[1])
	}

	// Try to extract version from "since Go X.Y" or "added in X.Y"
	if m := reSince.FindStringSubmatch(raw); m != nil {
		doc.Since = m[1]
	}

	return doc
}

// stripCommentMarkers removes Go comment delimiters from raw text.
func stripCommentMarkers(raw string) string {
	lines := strings.Split(raw, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Remove // prefix
		if strings.HasPrefix(trimmed, "// ") {
			trimmed = trimmed[3:]
		} else if strings.HasPrefix(trimmed, "//") {
			trimmed = trimmed[2:]
		}
		// Remove /* */ block comment markers
		trimmed = strings.TrimPrefix(trimmed, "/* ")
		trimmed = strings.TrimPrefix(trimmed, "/*")
		trimmed = strings.TrimSuffix(trimmed, " */")
		trimmed = strings.TrimSuffix(trimmed, "*/")
		result = append(result, trimmed)
	}

	return strings.Join(result, "\n")
}
