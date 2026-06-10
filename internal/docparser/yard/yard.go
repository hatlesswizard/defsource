// Package yard parses Ruby YARD documentation comments into structured
// DocComment values. It handles @param, @return, @raise, @deprecated,
// @see, @example, and @option tags.
package yard

import (
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
)

var (
	// @param [Type] name description
	// @param name [Type] description
	reParamBracket = regexp.MustCompile(
		`^@param\s+\[([^\]]+)\]\s+(\w+)\s*(.*)`)
	reParamName = regexp.MustCompile(
		`^@param\s+(\w+)\s+\[([^\]]+)\]\s*(.*)`)

	// @return [Type] description
	reReturn = regexp.MustCompile(
		`^@return\s+\[([^\]]+)\]\s*(.*)`)

	// @raise [Type] description
	reRaise = regexp.MustCompile(
		`^@raise\s+\[([^\]]+)\]\s*(.*)`)

	// @option name [Type] :key description
	reOption = regexp.MustCompile(
		`^@option\s+(\w+)\s+\[([^\]]+)\]\s+:(\w+)\s*(.*)`)

	reDeprTag    = regexp.MustCompile(`^@deprecated\s*(.*)`)
	reSeeTag     = regexp.MustCompile(`^@see\s+(.+)`)
	reSinceTag   = regexp.MustCompile(`^@since\s+(.+)`)
	reExampleTag = regexp.MustCompile(`^@example\s*(.*)`)
)

// Parser implements docparser.Parser for Ruby YARD comments.
type Parser struct{}

// New returns a new YARD parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses a Ruby YARD comment block into a DocComment.
func (p *Parser) Parse(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}

	// Strip # comment markers
	raw = stripRubyCommentMarkers(raw)

	lines := strings.Split(raw, "\n")
	var descLines []string
	var tagBlocks []string
	inTags := false
	inExample := false
	var exampleLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inExample {
			if strings.HasPrefix(trimmed, "@") {
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
			rest := reExampleTag.FindStringSubmatch(trimmed)
			if rest != nil && rest[1] != "" {
				// Example title on the same line
				exampleLines = append(exampleLines, rest[1])
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

	// Flush remaining example
	if inExample && len(exampleLines) > 0 {
		doc.Examples = append(doc.Examples, strings.TrimSpace(strings.Join(exampleLines, "\n")))
	}

	fullDesc := strings.TrimSpace(strings.Join(descLines, "\n"))
	doc.Description = fullDesc
	parts := strings.SplitN(fullDesc, "\n\n", 2)
	doc.Summary = strings.TrimSpace(parts[0])

	for _, block := range tagBlocks {
		block = strings.TrimSpace(block)

		if m := reParamBracket.FindStringSubmatch(block); m != nil {
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Type:        m[1],
				Name:        m[2],
				Description: strings.TrimSpace(m[3]),
			})
			continue
		}

		if m := reParamName.FindStringSubmatch(block); m != nil {
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[1],
				Type:        m[2],
				Description: strings.TrimSpace(m[3]),
			})
			continue
		}

		if m := reReturn.FindStringSubmatch(block); m != nil {
			doc.Returns = &docparser.ReturnDoc{
				Type:        m[1],
				Description: strings.TrimSpace(m[2]),
			}
			continue
		}

		if m := reRaise.FindStringSubmatch(block); m != nil {
			doc.Throws = append(doc.Throws, docparser.ThrowDoc{
				Type:        m[1],
				Description: strings.TrimSpace(m[2]),
			})
			continue
		}

		if m := reOption.FindStringSubmatch(block); m != nil {
			// Options are treated as sub-parameters
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[1] + "." + m[3],
				Type:        m[2],
				Description: strings.TrimSpace(m[4]),
				Optional:    true,
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

// stripRubyCommentMarkers removes # prefixes from lines.
func stripRubyCommentMarkers(raw string) string {
	lines := strings.Split(raw, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			trimmed = trimmed[2:]
		} else if trimmed == "#" {
			trimmed = ""
		} else if strings.HasPrefix(trimmed, "#") {
			trimmed = trimmed[1:]
		}
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}
