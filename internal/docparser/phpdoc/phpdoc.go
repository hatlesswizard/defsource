// Package phpdoc parses PHPDoc comment blocks (/** ... */) into structured
// DocComment values. It handles @param, @return, @since, @deprecated, @see,
// @throws, and @example tags.
package phpdoc

import (
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
)

// Compiled tag patterns. Package-level so they are compiled once at program
// start (regexp.MustCompile panics on bad pattern, which is the correct
// failure mode for hard-coded patterns).
var (
	reParamTag = regexp.MustCompile(
		`^@param\s+(\S+)\s+(\$\w+)\s*(.*)`)

	reReturnTag = regexp.MustCompile(
		`^@return\s+(\S+)\s*(.*)`)

	reThrowsTag = regexp.MustCompile(
		`^@throws\s+(\S+)\s*(.*)`)

	reSinceTag     = regexp.MustCompile(`^@since\s+(.+)`)
	reDeprTag      = regexp.MustCompile(`^@deprecated\s+(.+)`)
	reSeeTag       = regexp.MustCompile(`^@see\s+(.+)`)
	reExampleTag   = regexp.MustCompile(`^@example\s+(.+)`)
	reOptionalWord = regexp.MustCompile(`(?i)\boptional\b`)
)

// Parser implements docparser.Parser for PHPDoc comments.
type Parser struct{}

// New returns a new PHPDoc parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses a /** ... */ PHPDoc block into a DocComment.
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

	sinceFound := false
	for _, block := range tagBlocks {
		block = strings.TrimSpace(block)

		if m := reParamTag.FindStringSubmatch(block); m != nil {
			desc := strings.TrimSpace(m[3])
			pd := docparser.ParamDoc{
				Type:        m[1],
				Name:        m[2],
				Description: desc,
				Optional:    isOptionalParam(desc),
			}
			doc.Params = append(doc.Params, pd)
			continue
		}

		if m := reReturnTag.FindStringSubmatch(block); m != nil {
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
			if !sinceFound {
				doc.Since = strings.TrimSpace(m[1])
				sinceFound = true
			}
			continue
		}

		if m := reDeprTag.FindStringSubmatch(block); m != nil {
			doc.Deprecated = strings.TrimSpace(m[1])
			continue
		}

		if m := reSeeTag.FindStringSubmatch(block); m != nil {
			doc.See = append(doc.See, strings.TrimSpace(m[1]))
			continue
		}

		if m := reExampleTag.FindStringSubmatch(block); m != nil {
			doc.Examples = append(doc.Examples, strings.TrimSpace(m[1]))
			continue
		}
	}

	return doc
}

// isOptionalParam reports whether a PHPDoc @param description marks the
// parameter as optional.
func isOptionalParam(desc string) bool {
	lower := strings.ToLower(desc)
	if strings.Contains(lower, "non-optional") || strings.Contains(lower, "not optional") {
		return false
	}
	return reOptionalWord.MatchString(desc)
}
