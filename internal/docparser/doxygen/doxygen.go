// Package doxygen parses C/C++ Doxygen documentation comments into structured
// DocComment values. It supports both /** \tag */ and /// @tag formats.
// Handles \param, \return, \throws, \deprecated, \see, \brief, and \tparam.
package doxygen

import (
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
)

var (
	// \param name description  OR  @param name description
	reParam = regexp.MustCompile(`^[\\@]param\s+(\w+)\s+(.*)`)
	// \param[in] name desc, \param[out] name desc, \param[in,out] name desc
	reParamDir = regexp.MustCompile(`^[\\@]param\[([^\]]+)\]\s+(\w+)\s+(.*)`)
	// \tparam name description
	reTparam = regexp.MustCompile(`^[\\@]tparam\s+(\w+)\s+(.*)`)
	// \return description  OR  \returns description
	reReturn = regexp.MustCompile(`^[\\@]returns?\s+(.*)`)
	// \throws type description  OR  \throw type description
	reThrows = regexp.MustCompile(`^[\\@](?:throws?|exception)\s+(\S+)\s*(.*)`)
	// \brief description
	reBrief = regexp.MustCompile(`^[\\@]brief\s+(.*)`)
	// \deprecated description
	reDepr = regexp.MustCompile(`^[\\@]deprecated\s*(.*)`)
	// \since version
	reSince = regexp.MustCompile(`^[\\@]since\s+(.+)`)
	// \see reference
	reSee = regexp.MustCompile(`^[\\@]see\s+(.+)`)
)

// Parser implements docparser.Parser for Doxygen comments.
type Parser struct{}

// New returns a new Doxygen parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses a Doxygen comment block into a DocComment.
func (p *Parser) Parse(raw string) *docparser.DocComment {
	doc := &docparser.DocComment{}

	raw = stripDoxygenMarkers(raw)
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return doc
	}

	lines := strings.Split(raw, "\n")
	var descLines []string
	var tagBlocks []string
	inTags := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		isTag := false
		if len(trimmed) > 0 && (trimmed[0] == '\\' || trimmed[0] == '@') {
			// Check if it matches a known tag pattern
			if isDoxygenTag(trimmed) {
				isTag = true
			}
		}

		if isTag {
			inTags = true
			tagBlocks = append(tagBlocks, trimmed)
		} else if inTags {
			// Blank lines break tag continuation in Doxygen
			if trimmed == "" {
				inTags = false
				descLines = append(descLines, line)
			} else if len(tagBlocks) > 0 {
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

		if m := reBrief.FindStringSubmatch(block); m != nil {
			// \brief overrides the summary
			doc.Summary = strings.TrimSpace(m[1])
			continue
		}

		if m := reParamDir.FindStringSubmatch(block); m != nil {
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[2],
				Description: strings.TrimSpace(m[3]),
			})
			continue
		}

		if m := reParam.FindStringSubmatch(block); m != nil {
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[1],
				Description: strings.TrimSpace(m[2]),
			})
			continue
		}

		if m := reTparam.FindStringSubmatch(block); m != nil {
			// Template parameters — stored as params with a type indicator
			doc.Params = append(doc.Params, docparser.ParamDoc{
				Name:        m[1],
				Type:        "template",
				Description: strings.TrimSpace(m[2]),
			})
			continue
		}

		if m := reReturn.FindStringSubmatch(block); m != nil {
			doc.Returns = &docparser.ReturnDoc{
				Description: strings.TrimSpace(m[1]),
			}
			continue
		}

		if m := reThrows.FindStringSubmatch(block); m != nil {
			doc.Throws = append(doc.Throws, docparser.ThrowDoc{
				Type:        m[1],
				Description: strings.TrimSpace(m[2]),
			})
			continue
		}

		if m := reSince.FindStringSubmatch(block); m != nil {
			doc.Since = strings.TrimSpace(m[1])
			continue
		}

		if m := reDepr.FindStringSubmatch(block); m != nil {
			doc.Deprecated = strings.TrimSpace(m[1])
			if doc.Deprecated == "" {
				doc.Deprecated = "true"
			}
			continue
		}

		if m := reSee.FindStringSubmatch(block); m != nil {
			doc.See = append(doc.See, strings.TrimSpace(m[1]))
			continue
		}
	}

	return doc
}

// isDoxygenTag checks if a line starts with a known Doxygen tag.
func isDoxygenTag(line string) bool {
	tags := []string{
		`\param`, `\tparam`, `\return`, `\returns`, `\throw`, `\throws`,
		`\exception`, `\brief`, `\deprecated`, `\since`, `\see`,
		`\note`, `\warning`, `\author`, `\version`, `\date`,
		`@param`, `@tparam`, `@return`, `@returns`, `@throw`, `@throws`,
		`@exception`, `@brief`, `@deprecated`, `@since`, `@see`,
		`@note`, `@warning`, `@author`, `@version`, `@date`,
	}
	for _, tag := range tags {
		if strings.HasPrefix(line, tag+" ") || strings.HasPrefix(line, tag+"[") || line == tag {
			return true
		}
	}
	return false
}

// stripDoxygenMarkers removes comment delimiters from Doxygen comments.
func stripDoxygenMarkers(raw string) string {
	lines := strings.Split(raw, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Remove /** and */
		trimmed = strings.TrimPrefix(trimmed, "/**")
		trimmed = strings.TrimPrefix(trimmed, "/*!")
		trimmed = strings.TrimSuffix(trimmed, "*/")
		// Remove leading * in block comments
		if strings.HasPrefix(trimmed, "* ") {
			trimmed = trimmed[2:]
		} else if trimmed == "*" {
			trimmed = ""
		} else if strings.HasPrefix(trimmed, "*") {
			trimmed = trimmed[1:]
		}
		// Remove /// and //! prefixes
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
