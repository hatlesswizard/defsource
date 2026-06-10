// Package jsdoc parses JSDoc comment blocks (/** ... */) into structured
// DocComment values. It handles @param {type} name, @returns, @deprecated,
// @see, @example, @throws, @typedef, and @callback tags.
package jsdoc

import (
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
)

var (
	// @param {type} name - description
	// @param {type} [name] - description (optional)
	// @param {type} [name=default] - description (optional with default)
	reParamTag = regexp.MustCompile(
		`^@param\s+\{([^}]*)\}\s+(\[?)([\w.]+)(?:=([^\]]*))?\]?\s*(?:-\s*)?(.*)`)

	// @returns {type} description
	reReturnsTag = regexp.MustCompile(
		`^@returns?\s+\{([^}]*)\}\s*(.*)`)

	// @throws {type} description
	reThrowsTag = regexp.MustCompile(
		`^@throws?\s+\{([^}]*)\}\s*(.*)`)

	reDeprTag    = regexp.MustCompile(`^@deprecated\s*(.*)`)
	reSinceTag   = regexp.MustCompile(`^@since\s+(.+)`)
	reSeeTag     = regexp.MustCompile(`^@see\s+(.+)`)
	reExampleTag = regexp.MustCompile(`^@example`)
)

// Parser implements docparser.Parser for JSDoc comments.
type Parser struct{}

// New returns a new JSDoc parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses a /** ... */ JSDoc block into a DocComment.
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
	inExample := false
	var exampleLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inExample {
			if strings.HasPrefix(trimmed, "@") && !strings.HasPrefix(trimmed, "@example") {
				// End of example block
				doc.Examples = append(doc.Examples, strings.TrimSpace(strings.Join(exampleLines, "\n")))
				exampleLines = nil
				inExample = false
				// Fall through to process this tag
			} else {
				exampleLines = append(exampleLines, line)
				continue
			}
		}

		if reExampleTag.MatchString(trimmed) {
			inTags = true
			inExample = true
			// If there's content on the same line as @example
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "@example"))
			if rest != "" {
				exampleLines = append(exampleLines, rest)
			}
			continue
		}

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

	// Flush any remaining example
	if inExample && len(exampleLines) > 0 {
		doc.Examples = append(doc.Examples, strings.TrimSpace(strings.Join(exampleLines, "\n")))
	}

	fullDesc := strings.TrimSpace(strings.Join(descLines, "\n"))
	doc.Description = fullDesc

	parts := strings.SplitN(fullDesc, "\n\n", 2)
	doc.Summary = strings.TrimSpace(parts[0])

	for _, block := range tagBlocks {
		block = strings.TrimSpace(block)

		if m := reParamTag.FindStringSubmatch(block); m != nil {
			pd := docparser.ParamDoc{
				Type:        m[1],
				Name:        m[3],
				Description: strings.TrimSpace(m[5]),
				Optional:    m[2] == "[",
				Default:     m[4],
			}
			doc.Params = append(doc.Params, pd)
			continue
		}

		if m := reReturnsTag.FindStringSubmatch(block); m != nil {
			doc.Returns = &docparser.ReturnDoc{
				Type:        m[1],
				Description: strings.TrimSpace(m[2]),
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
