// Package xmldoc parses C# XML documentation comments (/// lines or
// /** ... */ blocks with XML tags) into structured DocComment values.
// It handles <summary>, <param>, <returns>, <exception>, <see>,
// <remarks>, and <example> elements.
package xmldoc

import (
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
)

var (
	reSummary   = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)
	reRemarks   = regexp.MustCompile(`(?s)<remarks>(.*?)</remarks>`)
	reReturns   = regexp.MustCompile(`(?s)<returns>(.*?)</returns>`)
	reExample   = regexp.MustCompile(`(?s)<example>(.*?)</example>`)
	reParam     = regexp.MustCompile(`(?s)<param\s+name="([^"]+)">(.*?)</param>`)
	reException = regexp.MustCompile(`(?s)<exception\s+cref="([^"]+)">(.*?)</exception>`)
	reSeeCref   = regexp.MustCompile(`<see\s+cref="([^"]+)"\s*/?>`)
	reSeeHref   = regexp.MustCompile(`<see\s+href="([^"]+)"[^/]*/?>`)
	reXMLTags   = regexp.MustCompile(`<[^>]+>`)
)

// Parser implements docparser.Parser for C# XML doc comments.
type Parser struct{}

// New returns a new C# XML doc parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses C# XML documentation comment into a DocComment.
func (p *Parser) Parse(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}

	// Strip /// prefixes if present
	raw = stripXMLCommentMarkers(raw)

	// Extract <summary>
	if m := reSummary.FindStringSubmatch(raw); m != nil {
		summary := cleanXMLContent(m[1])
		doc.Summary = summary
		doc.Description = summary
	}

	// Extract <remarks> to extend description
	if m := reRemarks.FindStringSubmatch(raw); m != nil {
		remarks := cleanXMLContent(m[1])
		if doc.Description != "" {
			doc.Description += "\n\n" + remarks
		} else {
			doc.Description = remarks
		}
	}

	// Extract <param> elements
	paramMatches := reParam.FindAllStringSubmatch(raw, -1)
	for _, m := range paramMatches {
		doc.Params = append(doc.Params, docparser.ParamDoc{
			Name:        m[1],
			Description: cleanXMLContent(m[2]),
		})
	}

	// Extract <returns>
	if m := reReturns.FindStringSubmatch(raw); m != nil {
		doc.Returns = &docparser.ReturnDoc{
			Description: cleanXMLContent(m[1]),
		}
	}

	// Extract <exception> elements
	excMatches := reException.FindAllStringSubmatch(raw, -1)
	for _, m := range excMatches {
		doc.Throws = append(doc.Throws, docparser.ThrowDoc{
			Type:        m[1],
			Description: cleanXMLContent(m[2]),
		})
	}

	// Extract <see cref=""> references
	seeMatches := reSeeCref.FindAllStringSubmatch(raw, -1)
	for _, m := range seeMatches {
		// Only add unique cref values that are not inside other elements
		doc.See = append(doc.See, m[1])
	}
	seeHrefMatches := reSeeHref.FindAllStringSubmatch(raw, -1)
	for _, m := range seeHrefMatches {
		doc.See = append(doc.See, m[1])
	}

	// Extract <example>
	exampleMatches := reExample.FindAllStringSubmatch(raw, -1)
	for _, m := range exampleMatches {
		doc.Examples = append(doc.Examples, cleanXMLContent(m[1]))
	}

	return doc
}

// stripXMLCommentMarkers removes /// prefixes from lines.
func stripXMLCommentMarkers(raw string) string {
	lines := strings.Split(raw, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "/// ") {
			trimmed = trimmed[4:]
		} else if strings.HasPrefix(trimmed, "///") {
			trimmed = trimmed[3:]
		}
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}

// cleanXMLContent strips XML tags and normalizes whitespace within content.
func cleanXMLContent(s string) string {
	// Replace <see cref="X"/> with X for inline references
	s = reSeeCref.ReplaceAllString(s, "$1")
	// Remove other XML tags
	s = reXMLTags.ReplaceAllString(s, "")
	// Normalize whitespace
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, " ")
}
