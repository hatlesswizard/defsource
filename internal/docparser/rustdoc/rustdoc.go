// Package rustdoc parses Rust documentation comments (/// and //!) into
// structured DocComment values. It extracts structure from markdown sections:
// # Arguments, # Returns, # Errors, # Panics, # Examples, # Safety.
package rustdoc

import (
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
)

var (
	reSection = regexp.MustCompile(`^#+\s+(.+)`)
	// "* `name` - description" pattern common in # Arguments
	reArgEntry = regexp.MustCompile(`^\s*\*\s+` + "`" + `(\w+)` + "`" + `\s*[-–—]\s*(.*)`)
	// Deprecated since X.Y.Z: (captures version without trailing colon/punctuation)
	reDeprecated = regexp.MustCompile(`(?i)deprecated\s+since\s+([\d.]+)`)
)

// Parser implements docparser.Parser for Rust doc comments.
type Parser struct{}

// New returns a new Rust doc comment parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses a Rust doc comment into a DocComment.
func (p *Parser) Parse(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}

	raw = stripRustCommentMarkers(raw)
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return doc
	}

	lines := strings.Split(raw, "\n")
	var descLines []string
	currentSection := ""
	var sectionContent []string
	var safetyContent string

	flushSection := func() {
		if currentSection == "" {
			return
		}
		content := strings.TrimSpace(strings.Join(sectionContent, "\n"))
		switch strings.ToLower(currentSection) {
		case "arguments", "parameters":
			parseRustArgs(doc, sectionContent)
		case "returns", "return value":
			doc.Returns = &docparser.ReturnDoc{
				Description: content,
			}
		case "errors":
			parseRustErrors(doc, sectionContent)
		case "panics":
			doc.Throws = append(doc.Throws, docparser.ThrowDoc{
				Type:        "panic",
				Description: content,
			})
		case "examples", "example":
			doc.Examples = append(doc.Examples, content)
		case "safety":
			safetyContent = content
		}
		sectionContent = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if m := reSection.FindStringSubmatch(trimmed); m != nil {
			flushSection()
			currentSection = m[1]
			continue
		}

		if currentSection != "" {
			sectionContent = append(sectionContent, line)
		} else {
			descLines = append(descLines, line)
		}
	}
	flushSection()

	fullDesc := strings.TrimSpace(strings.Join(descLines, "\n"))
	doc.Description = fullDesc
	if safetyContent != "" {
		doc.Description += "\n\nSafety: " + safetyContent
	}

	parts := strings.SplitN(fullDesc, "\n\n", 2)
	doc.Summary = strings.TrimSpace(parts[0])

	// Check for deprecated marker
	if m := reDeprecated.FindStringSubmatch(raw); m != nil {
		doc.Deprecated = m[1]
	} else if strings.Contains(strings.ToLower(raw), "deprecated") {
		// Look for inline deprecated mention
		for _, line := range lines {
			lower := strings.ToLower(strings.TrimSpace(line))
			if strings.HasPrefix(lower, "deprecated") {
				doc.Deprecated = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Deprecated"))
				doc.Deprecated = strings.TrimPrefix(doc.Deprecated, ":")
				doc.Deprecated = strings.TrimSpace(doc.Deprecated)
				if doc.Deprecated == "" {
					doc.Deprecated = "true"
				}
				break
			}
		}
	}

	return doc
}

// parseRustArgs extracts parameters from "* `name` - desc" entries.
func parseRustArgs(doc *docparser.DocComment, lines []string) {
	for _, line := range lines {
		if m := reArgEntry.FindStringSubmatch(line); m != nil {
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[1],
				Description: strings.TrimSpace(m[2]),
			})
		}
	}
}

// parseRustErrors extracts error types from the Errors section.
func parseRustErrors(doc *docparser.DocComment, lines []string) {
	content := strings.TrimSpace(strings.Join(lines, "\n"))
	if content != "" {
		doc.Throws = append(doc.Throws, docparser.ThrowDoc{
			Type:        "Error",
			Description: content,
		})
	}
}

// stripRustCommentMarkers removes /// and //! prefixes from lines.
func stripRustCommentMarkers(raw string) string {
	lines := strings.Split(raw, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "/// ") {
			trimmed = trimmed[4:]
		} else if strings.HasPrefix(trimmed, "///") {
			trimmed = trimmed[3:]
		} else if strings.HasPrefix(trimmed, "//! ") {
			trimmed = trimmed[4:]
		} else if strings.HasPrefix(trimmed, "//!") {
			trimmed = trimmed[3:]
		}
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}
