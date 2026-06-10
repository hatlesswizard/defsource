// Package php — parser.go: byte-scan bridge between the PHPDoc regex
// parser (phpdoc.go) and the tree-sitter AST walker (ast.go).
//
// findPrecedingDoc is a bridge utility: it locates the raw /** ... */ block
// that immediately precedes a declaration, so that the PHPDoc parser can
// process it. It belongs to neither parser exclusively.
package php

// findPrecedingDoc finds the PHPDoc comment immediately preceding a
// declaration at byte position pos. It returns the comment text (including
// /** and */ delimiters) and true, or ("", false) if no doc comment is found.
// It safely handles PHP 8 attributes (#[...]) between the doc comment and the
// declaration, and returns ("", false) without panicking when the declaration
// or its attributes appear at byte offset 0 of the source.
func findPrecedingDoc(content []byte, pos int) (string, bool) {
	i := pos - 1

	// Guard: declaration at byte 0 or empty content — no room for a doc comment.
	if i < 0 {
		return "", false
	}

	for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
		i--
	}

	// Skip PHP 8 attributes: #[...]
	for i >= 0 && content[i] == ']' {
		depth := 1
		i--
		for i >= 0 && depth > 0 {
			if content[i] == ']' {
				depth++
			} else if content[i] == '[' {
				depth--
			}
			if depth > 0 {
				i--
			}
		}
		// Guard: attribute bracket at byte 0 means no preceding '#' exists.
		if i < 1 {
			return "", false
		}
		if content[i] == '[' && content[i-1] == '#' {
			i -= 2
		} else {
			return "", false
		}
		for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
			i--
		}
	}

	if i < 1 || content[i] != '/' || content[i-1] != '*' {
		return "", false
	}

	end := i
	i -= 2
	for i >= 1 {
		if content[i] == '/' && content[i+1] == '*' && (i+2 < len(content) && content[i+2] == '*') {
			docComment := string(content[i : end+1])
			return docComment, true
		}
		i--
	}

	return "", false
}
