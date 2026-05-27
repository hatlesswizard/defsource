// Package wpgithub — phpdoc.go: regex-based PHPDoc comment parser.
// This file is intentionally separate from ast.go (tree-sitter AST walker)
// because the two parsers have different inputs (string vs []byte), different
// dependencies (regexp vs go-tree-sitter), and different reasons to change.
package wpgithub

import (
	"regexp"
	"strings"
)

// phpDoc holds the structured data extracted from a /** ... */ comment block.
type phpDoc struct {
	Summary     string
	Description string
	Params      []docParam
	Return      docReturn
	Since       string
	Deprecated  string
	See         []string
	Uses        []string
	VarType     string
}

// docParam holds a single @param entry extracted from a PHPDoc block.
type docParam struct {
	Name     string
	Type     string
	Desc     string
	Optional bool
}

// docReturn holds the @return entry extracted from a PHPDoc block.
type docReturn struct {
	Type string
	Desc string
}

// Compiled tag patterns used by parsePhpDoc. Package-level so they are
// compiled once at program start (regexp.MustCompile panics on bad pattern,
// which is the correct failure mode for hard-coded patterns).
var (
	reParamTag = regexp.MustCompile(
		`^@param\s+(\S+)\s+(\$\w+)\s*(.*)`)

	reReturnTag = regexp.MustCompile(
		`^@return\s+(\S+)\s*(.*)`)

	reSinceTag     = regexp.MustCompile(`^@since\s+(.+)`)
	reDeprTag      = regexp.MustCompile(`^@deprecated\s+(.+)`)
	reSeeTag       = regexp.MustCompile(`^@see\s+(.+)`)
	reUsesTag      = regexp.MustCompile(`^@uses\s+(.+)`)
	reVarTag       = regexp.MustCompile(`^@var\s+(\S+)`)
	reOptionalWord = regexp.MustCompile(`(?i)\boptional\b`)
)

// isOptionalParam reports whether a PHPDoc @param description marks the
// parameter as optional. The rule:
//   - The description must contain the whole word "optional" (case-insensitive).
//   - The description must NOT contain "non-optional" or "not optional".
//
// This prevents false positives on descriptions like "non-optional base URL"
// or "this parameter is not optional".
func isOptionalParam(desc string) bool {
	lower := strings.ToLower(desc)
	if strings.Contains(lower, "non-optional") || strings.Contains(lower, "not optional") {
		return false
	}
	return reOptionalWord.MatchString(desc)
}

// parsePhpDoc parses a /** ... */ PHPDoc block into structured data.
func parsePhpDoc(comment string) phpDoc {
	var doc phpDoc

	comment = strings.TrimSpace(comment)
	comment = strings.TrimPrefix(comment, "/**")
	comment = strings.TrimSuffix(comment, "*/")

	rawLines := strings.Split(comment, "\n")
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
			tagBlocks[len(tagBlocks)-1] += " " + trimmed
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
			dp := docParam{
				Type:     m[1],
				Name:     m[2],
				Desc:     strings.TrimSpace(m[3]),
				Optional: isOptionalParam(strings.TrimSpace(m[3])),
			}
			doc.Params = append(doc.Params, dp)
			continue
		}

		if m := reReturnTag.FindStringSubmatch(block); m != nil {
			doc.Return = docReturn{
				Type: m[1],
				Desc: strings.TrimSpace(m[2]),
			}
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

		if m := reUsesTag.FindStringSubmatch(block); m != nil {
			doc.Uses = append(doc.Uses, strings.TrimSpace(m[1]))
			continue
		}

		if m := reVarTag.FindStringSubmatch(block); m != nil {
			doc.VarType = strings.TrimSpace(m[1])
			continue
		}
	}

	return doc
}
