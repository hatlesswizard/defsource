// Package pydoc parses Python docstrings into structured DocComment values.
// It auto-detects and supports three formats:
//   - Google style: Args:, Returns:, Raises:
//   - NumPy style: Parameters\n----------
//   - Sphinx style: :param name: desc, :type name: type
package pydoc

import (
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
)

// Format represents the detected docstring format.
type Format int

const (
	FormatGoogle Format = iota
	FormatNumPy
	FormatSphinx
	FormatPlain
)

var (
	// Sphinx patterns
	// :param name: desc  OR  :param type name: desc
	reSphinxParamTyped = regexp.MustCompile(`^:param\s+(\S+)\s+(\w+):\s*(.*)`)
	reSphinxParam      = regexp.MustCompile(`^:param\s+(\w+):\s*(.*)`)
	reSphinxType       = regexp.MustCompile(`^:type\s+(\w+):\s*(.*)`)
	reSphinxRtype      = regexp.MustCompile(`^:rtype:\s*(.*)`)
	reSphinxRet        = regexp.MustCompile(`^:returns?:\s*(.*)`)
	reSphinxRaise      = regexp.MustCompile(`^:raises?\s+(\w+):\s*(.*)`)

	// Google section headers
	reGoogleSection = regexp.MustCompile(`^(Args|Arguments|Parameters|Returns|Return|Raises|Throws|Examples?|Deprecated|Note|Notes|See Also):\s*$`)

	// NumPy section underlines
	reNumpyUnderline = regexp.MustCompile(`^-{3,}\s*$`)

	// Google/NumPy parameter pattern (indented "name (type): desc" or "name : type")
	reGoogleParam = regexp.MustCompile(`^\s{2,}(\w+)\s*\(([^)]*)\)\s*:\s*(.*)`)
	reNumpyParam  = regexp.MustCompile(`^\s{2,}(\w+)\s*:\s*(\S.*)`)

	// Deprecated prefix
	reDeprecated = regexp.MustCompile(`(?i)^deprecated\b[:\s]*(.*)`)
)

// Parser implements docparser.Parser for Python docstrings.
type Parser struct{}

// New returns a new Python docstring parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses a Python docstring into a DocComment. It auto-detects the
// format (Google, NumPy, or Sphinx) from the content.
func (p *Parser) Parse(raw string) *docparser.DocComment {
	raw = strings.TrimSpace(raw)
	// Strip triple quotes if present
	raw = strings.TrimPrefix(raw, `"""`)
	raw = strings.TrimPrefix(raw, `'''`)
	raw = strings.TrimSuffix(raw, `"""`)
	raw = strings.TrimSuffix(raw, `'''`)
	raw = strings.TrimSpace(raw)

	format := detectFormat(raw)

	switch format {
	case FormatSphinx:
		return parseSphinx(raw)
	case FormatNumPy:
		return parseNumPy(raw)
	case FormatGoogle:
		return parseGoogle(raw)
	default:
		return parsePlain(raw)
	}
}

// detectFormat auto-detects which docstring format is used.
func detectFormat(raw string) Format {
	lines := strings.Split(raw, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ":param ") || strings.HasPrefix(trimmed, ":type ") ||
			strings.HasPrefix(trimmed, ":returns:") || strings.HasPrefix(trimmed, ":return:") ||
			strings.HasPrefix(trimmed, ":rtype:") || strings.HasPrefix(trimmed, ":raises ") {
			return FormatSphinx
		}
	}

	for i, line := range lines {
		if i+1 < len(lines) && reNumpyUnderline.MatchString(strings.TrimSpace(lines[i+1])) {
			trimmed := strings.TrimSpace(line)
			if trimmed == "Parameters" || trimmed == "Returns" || trimmed == "Raises" ||
				trimmed == "Examples" || trimmed == "See Also" {
				return FormatNumPy
			}
		}
	}

	for _, line := range lines {
		if reGoogleSection.MatchString(strings.TrimSpace(line)) {
			return FormatGoogle
		}
	}

	return FormatPlain
}

// parseSphinx handles :param, :type, :returns:, :rtype:, :raises tags.
func parseSphinx(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}
	lines := strings.Split(raw, "\n")

	var descLines []string
	paramTypes := make(map[string]string)
	inDesc := true

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Try typed param first: :param type name: desc
		if m := reSphinxParamTyped.FindStringSubmatch(trimmed); m != nil {
			inDesc = false
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[2],
				Type:        m[1],
				Description: strings.TrimSpace(m[3]),
			})
			continue
		}

		if m := reSphinxParam.FindStringSubmatch(trimmed); m != nil {
			inDesc = false
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[1],
				Description: strings.TrimSpace(m[2]),
			})
			continue
		}

		if m := reSphinxType.FindStringSubmatch(trimmed); m != nil {
			inDesc = false
			paramTypes[m[1]] = strings.TrimSpace(m[2])
			continue
		}

		if m := reSphinxRet.FindStringSubmatch(trimmed); m != nil {
			inDesc = false
			if doc.Returns == nil {
				doc.Returns = &docparser.ReturnDoc{}
			}
			doc.Returns.Description = strings.TrimSpace(m[1])
			continue
		}

		if m := reSphinxRtype.FindStringSubmatch(trimmed); m != nil {
			inDesc = false
			if doc.Returns == nil {
				doc.Returns = &docparser.ReturnDoc{}
			}
			doc.Returns.Type = strings.TrimSpace(m[1])
			continue
		}

		if m := reSphinxRaise.FindStringSubmatch(trimmed); m != nil {
			inDesc = false
			doc.Throws = append(doc.Throws, docparser.ThrowDoc{
				Type:        m[1],
				Description: strings.TrimSpace(m[2]),
			})
			continue
		}

		if inDesc {
			descLines = append(descLines, line)
		}
	}

	// Assign types to params
	for i, p := range doc.Params {
		if t, ok := paramTypes[p.Name]; ok {
			doc.Params[i].Type = t
		}
	}

	fullDesc := strings.TrimSpace(strings.Join(descLines, "\n"))
	doc.Description = fullDesc
	parts := strings.SplitN(fullDesc, "\n\n", 2)
	doc.Summary = strings.TrimSpace(parts[0])

	return doc
}

// parseGoogle handles Google-style sections: Args:, Returns:, Raises:, etc.
func parseGoogle(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}
	lines := strings.Split(raw, "\n")

	var descLines []string
	currentSection := ""
	var sectionContent []string

	flushSection := func() {
		if currentSection == "" {
			return
		}
		switch strings.ToLower(currentSection) {
		case "args", "arguments", "parameters":
			parseGoogleParams(doc, sectionContent)
		case "returns", "return":
			parseGoogleReturns(doc, sectionContent)
		case "raises", "throws":
			parseGoogleRaises(doc, sectionContent)
		case "examples", "example":
			doc.Examples = append(doc.Examples, strings.TrimSpace(strings.Join(sectionContent, "\n")))
		case "deprecated":
			doc.Deprecated = strings.TrimSpace(strings.Join(sectionContent, " "))
		case "see also":
			for _, line := range sectionContent {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					doc.See = append(doc.See, trimmed)
				}
			}
		}
		sectionContent = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if reGoogleSection.MatchString(trimmed) {
			flushSection()
			currentSection = strings.TrimSuffix(trimmed, ":")
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
	parts := strings.SplitN(fullDesc, "\n\n", 2)
	doc.Summary = strings.TrimSpace(parts[0])

	return doc
}

// parseGoogleParams parses indented parameter lines from a Google-style Args section.
func parseGoogleParams(doc *docparser.DocComment, lines []string) {
	for _, line := range lines {
		if m := reGoogleParam.FindStringSubmatch(line); m != nil {
			optional := strings.Contains(strings.ToLower(m[2]), "optional")
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[1],
				Type:        m[2],
				Description: strings.TrimSpace(m[3]),
				Optional:    optional,
			})
		}
	}
}

// parseGoogleReturns parses the Returns section content.
func parseGoogleReturns(doc *docparser.DocComment, lines []string) {
	content := strings.TrimSpace(strings.Join(lines, " "))
	if content == "" {
		return
	}
	// Try "type: description" format
	if idx := strings.Index(content, ":"); idx > 0 {
		doc.Returns = &docparser.ReturnDoc{
			Type:        strings.TrimSpace(content[:idx]),
			Description: strings.TrimSpace(content[idx+1:]),
		}
	} else {
		doc.Returns = &docparser.ReturnDoc{
			Description: content,
		}
	}
}

// parseGoogleRaises parses the Raises section content.
func parseGoogleRaises(doc *docparser.DocComment, lines []string) {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if idx := strings.Index(trimmed, ":"); idx > 0 {
			doc.Throws = append(doc.Throws, docparser.ThrowDoc{
				Type:        strings.TrimSpace(trimmed[:idx]),
				Description: strings.TrimSpace(trimmed[idx+1:]),
			})
		}
	}
}

// parseNumPy handles NumPy-style sections with underline separators.
func parseNumPy(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}
	lines := strings.Split(raw, "\n")

	var descLines []string
	currentSection := ""
	var sectionContent []string

	flushSection := func() {
		if currentSection == "" {
			return
		}
		switch strings.ToLower(currentSection) {
		case "parameters":
			parseNumpyParams(doc, sectionContent)
		case "returns":
			parseNumpyReturns(doc, sectionContent)
		case "raises":
			parseNumpyRaises(doc, sectionContent)
		case "examples":
			doc.Examples = append(doc.Examples, strings.TrimSpace(strings.Join(sectionContent, "\n")))
		case "see also":
			for _, line := range sectionContent {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					doc.See = append(doc.See, trimmed)
				}
			}
		}
		sectionContent = nil
	}

	skipUnderline := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if skipUnderline {
			skipUnderline = false
			if reNumpyUnderline.MatchString(trimmed) {
				continue
			}
		}

		// Detect section header (next line must be underline)
		isSection := false
		if trimmed == "Parameters" || trimmed == "Returns" || trimmed == "Raises" ||
			trimmed == "Examples" || trimmed == "See Also" {
			isSection = true
		}

		if isSection {
			flushSection()
			currentSection = trimmed
			skipUnderline = true
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
	parts := strings.SplitN(fullDesc, "\n\n", 2)
	doc.Summary = strings.TrimSpace(parts[0])

	return doc
}

// parseNumpyParams parses "name : type" entries from a NumPy Parameters section.
func parseNumpyParams(doc *docparser.DocComment, lines []string) {
	var currentParam *docparser.ParamDoc
	for _, line := range lines {
		if m := reNumpyParam.FindStringSubmatch(line); m != nil {
			if currentParam != nil {
				doc.Params = append(doc.Params, *currentParam)
			}
			optional := strings.Contains(strings.ToLower(m[2]), "optional")
			currentParam = &docparser.ParamDoc{
				Name:     m[1],
				Type:     strings.TrimSuffix(strings.TrimSpace(m[2]), ", optional"),
				Optional: optional,
			}
		} else if currentParam != nil {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if currentParam.Description != "" {
					currentParam.Description += " "
				}
				currentParam.Description += trimmed
			}
		}
	}
	if currentParam != nil {
		doc.Params = append(doc.Params, *currentParam)
	}
}

// parseNumpyReturns parses "name : type" entries from a NumPy Returns section.
func parseNumpyReturns(doc *docparser.DocComment, lines []string) {
	for _, line := range lines {
		if m := reNumpyParam.FindStringSubmatch(line); m != nil {
			doc.Returns = &docparser.ReturnDoc{
				Type: strings.TrimSpace(m[2]),
			}
		} else if doc.Returns != nil {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if doc.Returns.Description != "" {
					doc.Returns.Description += " "
				}
				doc.Returns.Description += trimmed
			}
		}
	}
}

// parseNumpyRaises parses "ExceptionType" entries from a NumPy Raises section.
func parseNumpyRaises(doc *docparser.DocComment, lines []string) {
	var current *docparser.ThrowDoc
	for _, line := range lines {
		if m := reNumpyParam.FindStringSubmatch(line); m != nil {
			if current != nil {
				doc.Throws = append(doc.Throws, *current)
			}
			current = &docparser.ThrowDoc{
				Type: m[1],
			}
		} else if current != nil {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if current.Description != "" {
					current.Description += " "
				}
				current.Description += trimmed
			}
		}
	}
	if current != nil {
		doc.Throws = append(doc.Throws, *current)
	}
}

// parsePlain handles plain docstrings with no structured format.
func parsePlain(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}
	doc.Description = raw

	parts := strings.SplitN(raw, "\n\n", 2)
	doc.Summary = strings.TrimSpace(parts[0])

	// Check for deprecated prefix
	if m := reDeprecated.FindStringSubmatch(doc.Summary); m != nil {
		doc.Deprecated = strings.TrimSpace(m[1])
		if doc.Deprecated == "" {
			doc.Deprecated = "true"
		}
	}

	return doc
}
