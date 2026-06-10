// Package javadoc parses JavaDoc comment blocks (/** ... */) into structured
// DocComment values. It handles @param, @return, @throws, @since,
// @deprecated, @see, and @version tags.
package javadoc

import (
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
)

var (
	reParamTag  = regexp.MustCompile(`^@param\s+(\w+)\s+(.*)`)
	reReturnTag = regexp.MustCompile(`^@returns?\s+(.*)`)
	reThrowsTag = regexp.MustCompile(`^@(?:throws|exception)\s+(\S+)\s*(.*)`)
	reSinceTag  = regexp.MustCompile(`^@since\s+(.+)`)
	reDeprTag   = regexp.MustCompile(`^@deprecated\s*(.*)`)
	reSeeTag    = regexp.MustCompile(`^@see\s+(.+)`)
)

// Parser implements docparser.Parser for JavaDoc comments.
type Parser struct{}

// New returns a new JavaDoc parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses a /** ... */ JavaDoc block into a DocComment.
func (p *Parser) Parse(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}

	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "/**")
	raw = strings.TrimSuffix(raw, "*/")

	rawLines := strings.Split(raw, "\n")
	var lines []string
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "* ") {
			line = line[2:]
		} else if line == "*" {
			line = ""
		} else if strings.HasPrefix(line, "*") {
			line = line[1:]
		}
		lines = append(lines, line)
	}

	var descLines []string
	var tagBlocks []string
	inTags := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@") {
			inTags = true
			tagBlocks = append(tagBlocks, trimmed)
		} else if inTags {
			if len(tagBlocks) > 0 {
				tagBlocks[len(tagBlocks)-1] += " " + trimmed
			}
		} else {
			descLines = append(descLines, line)
		}
	}

	fullDesc := strings.TrimSpace(strings.Join(descLines, "\n"))
	doc.Description = fullDesc

	parts := strings.SplitN(fullDesc, "\n\n", 2)
	doc.Summary = strings.TrimSpace(parts[0])

	for _, block := range tagBlocks {
		block = strings.TrimSpace(block)

		if m := reParamTag.FindStringSubmatch(block); m != nil {
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[1],
				Description: strings.TrimSpace(m[2]),
			})
			continue
		}

		if m := reReturnTag.FindStringSubmatch(block); m != nil {
			doc.Returns = &docparser.ReturnDoc{
				Description: strings.TrimSpace(m[1]),
			}
			continue
		}

		if m := reThrowsTag.FindStringSubmatch(block); m != nil {
			doc.Throws = append(doc.Throws, docparser.ThrowDoc{
				Type:        m[1],
				Description: strings.TrimSpace(m[2]),
			})
			continue
		}

		if m := reSinceTag.FindStringSubmatch(block); m != nil {
			doc.Since = strings.TrimSpace(m[1])
			continue
		}

		if m := reDeprTag.FindStringSubmatch(block); m != nil {
			doc.Deprecated = strings.TrimSpace(m[1])
			if doc.Deprecated == "" {
				doc.Deprecated = "true"
			}
			continue
		}

		if m := reSeeTag.FindStringSubmatch(block); m != nil {
			doc.See = append(doc.See, strings.TrimSpace(m[1]))
			continue
		}
	}

	return doc
}
