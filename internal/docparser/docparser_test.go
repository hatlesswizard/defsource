package docparser_test

import (
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/docparser"
	"github.com/hatlesswizard/defsource/internal/docparser/doxygen"
	"github.com/hatlesswizard/defsource/internal/docparser/godoc"
	"github.com/hatlesswizard/defsource/internal/docparser/javadoc"
	"github.com/hatlesswizard/defsource/internal/docparser/jsdoc"
	"github.com/hatlesswizard/defsource/internal/docparser/phpdoc"
	"github.com/hatlesswizard/defsource/internal/docparser/pydoc"
	"github.com/hatlesswizard/defsource/internal/docparser/rustdoc"
	"github.com/hatlesswizard/defsource/internal/docparser/xmldoc"
	"github.com/hatlesswizard/defsource/internal/docparser/yard"
)

// parserEntry associates a parser with its name for table-driven tests.
type parserEntry struct {
	name   string
	parser docparser.Parser
}

func allParsers() []parserEntry {
	return []parserEntry{
		{"phpdoc", phpdoc.New()},
		{"javadoc", javadoc.New()},
		{"jsdoc", jsdoc.New()},
		{"pydoc", pydoc.New()},
		{"godoc", godoc.New()},
		{"rustdoc", rustdoc.New()},
		{"doxygen", doxygen.New()},
		{"xmldoc", xmldoc.New()},
		{"yard", yard.New()},
	}
}

// TestAllParsers_EmptyInput verifies that every parser handles empty input
// gracefully and returns a non-nil DocComment.
func TestAllParsers_EmptyInput(t *testing.T) {
	for _, p := range allParsers() {
		t.Run(p.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Parse(\"\") panicked: %v", r)
				}
			}()
			doc := p.parser.Parse("")
			if doc == nil {
				t.Error("Parse(\"\") returned nil, expected non-nil *DocComment")
			}
		})
	}
}

// TestAllParsers_WhitespaceOnly verifies that parsers handle whitespace-only input.
func TestAllParsers_WhitespaceOnly(t *testing.T) {
	inputs := []string{"   ", "\t\t", "\n\n\n", "  \r\n  "}
	for _, p := range allParsers() {
		t.Run(p.name, func(t *testing.T) {
			for _, input := range inputs {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Parse(whitespace) panicked: %v", r)
					}
				}()
				doc := p.parser.Parse(input)
				if doc == nil {
					t.Error("Parse(whitespace) returned nil")
				}
			}
		})
	}
}

// TestAllParsers_MalformedInput verifies that parsers handle malformed input
// without panicking.
func TestAllParsers_MalformedInput(t *testing.T) {
	malformed := []string{
		"@@@@@",
		"<<<<>>>>",
		"\\\\\\\\",
		"/** unclosed",
		"*/ close without open",
		"@param",
		"@param @param @param",
		string([]byte{0x00, 0x01, 0x02}),
		"<!-- xml comment -->",
		"###",
		"`````",
		"@{@{@{",
		"<summary><param name=\"\"></summary>",
	}

	for _, p := range allParsers() {
		t.Run(p.name, func(t *testing.T) {
			for i, input := range malformed {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Parse(malformed[%d]) panicked: %v", i, r)
					}
				}()
				doc := p.parser.Parse(input)
				if doc == nil {
					t.Errorf("Parse(malformed[%d]) returned nil", i)
				}
			}
		})
	}
}

// TestAllParsers_ExtractSummary verifies that every parser extracts at least
// a Summary when given documentation content.
func TestAllParsers_ExtractSummary(t *testing.T) {
	tests := []struct {
		parser string
		input  string
	}{
		{"phpdoc", "/**\n * This is the summary.\n * @param string $name The name.\n */"},
		{"javadoc", "/**\n * This is the summary.\n * @param name The name.\n */"},
		{"jsdoc", "/**\n * This is the summary.\n * @param {string} name - The name.\n */"},
		{"pydoc", "This is the summary.\n\nArgs:\n    name (str): The name."},
		{"godoc", "This is the summary.\n\nIt has more detail below."},
		{"rustdoc", "/// This is the summary.\n///\n/// More detail below."},
		{"doxygen", "/**\n * \\brief This is the summary.\n * \\param name The name.\n */"},
		{"xmldoc", "/// <summary>This is the summary.</summary>\n/// <param name=\"name\">The name.</param>"},
		{"yard", "# This is the summary.\n# @param name [String] the name"},
	}

	parsers := map[string]docparser.Parser{
		"phpdoc":  phpdoc.New(),
		"javadoc": javadoc.New(),
		"jsdoc":   jsdoc.New(),
		"pydoc":   pydoc.New(),
		"godoc":   godoc.New(),
		"rustdoc": rustdoc.New(),
		"doxygen": doxygen.New(),
		"xmldoc":  xmldoc.New(),
		"yard":    yard.New(),
	}

	for _, tc := range tests {
		t.Run(tc.parser, func(t *testing.T) {
			p := parsers[tc.parser]
			doc := p.Parse(tc.input)
			if doc == nil {
				t.Fatal("Parse returned nil")
			}
			if doc.Summary == "" {
				t.Error("Summary is empty despite input containing documentation")
			}
		})
	}
}

// TestAllParsers_UnicodeContent verifies all parsers handle Unicode in doc comments
// without panicking. Not all parsers extract Summary from plain text (e.g., xmldoc
// requires XML tags), so we only verify no panic and non-nil return.
func TestAllParsers_UnicodeContent(t *testing.T) {
	unicodeInputs := []string{
		"Berechnet die Wärme des Systems.",
		"Возвращает результат вычисления.",
		"日本語のドキュメント。",
		"함수를 실행합니다.",
		"Emoji test: \xf0\x9f\x94\xa5 \xf0\x9f\x92\xaf \xe2\x9a\xa1",
		"Mixed: cafe resume naive uber",
	}

	// Parsers that only extract from structured tags (not plain text)
	structuredOnly := map[string]bool{"xmldoc": true}

	for _, p := range allParsers() {
		t.Run(p.name, func(t *testing.T) {
			for i, input := range unicodeInputs {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("Parse(unicode[%d]) panicked: %v", i, r)
						}
					}()
					doc := p.parser.Parse(input)
					if doc == nil {
						t.Errorf("Parse(unicode[%d]) returned nil", i)
						return
					}
					// Only check Summary for parsers that handle plain text
					if !structuredOnly[p.name] && doc.Summary == "" {
						t.Errorf("Parse(unicode[%d]) produced empty summary for %q", i, input)
					}
				}()
			}
		})
	}
}

// TestAllParsers_VeryLongDocComment verifies parsers handle very long doc comments
// without panicking or hanging.
func TestAllParsers_VeryLongDocComment(t *testing.T) {
	// Build a 100KB doc comment
	var sb strings.Builder
	sb.WriteString("This is the summary.\n\n")
	for i := 0; i < 2000; i++ {
		sb.WriteString("This is line ")
		sb.WriteString(strings.Repeat("x", 50))
		sb.WriteString(" of the documentation.\n")
	}
	longDoc := sb.String()
	if len(longDoc) < 100_000 {
		t.Fatalf("expected >100KB, got %d bytes", len(longDoc))
	}

	// Parsers that only extract from structured tags
	structuredOnly := map[string]bool{"xmldoc": true}

	for _, p := range allParsers() {
		t.Run(p.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Parse(longDoc) panicked: %v", r)
				}
			}()
			doc := p.parser.Parse(longDoc)
			if doc == nil {
				t.Error("Parse(longDoc) returned nil")
				return
			}
			if !structuredOnly[p.name] && doc.Summary == "" {
				t.Error("Parse(longDoc) produced empty summary")
			}
		})
	}
}

// TestAllParsers_DeprecatedTag verifies parsers detect deprecation.
func TestAllParsers_DeprecatedTag(t *testing.T) {
	tests := []struct {
		parser string
		input  string
	}{
		{"phpdoc", "/**\n * Summary.\n * @deprecated Since 5.0\n */"},
		{"javadoc", "/**\n * Summary.\n * @deprecated Use newMethod instead.\n */"},
		{"jsdoc", "/**\n * Summary.\n * @deprecated Use newMethod instead.\n */"},
		{"pydoc", "Summary.\n\nDeprecated:\n    Use new_method instead."},
		{"godoc", "Summary.\n\nDeprecated: Use NewMethod instead."},
		{"rustdoc", "/// Summary.\n///\n/// Deprecated since 1.0: Use new_method."},
		{"doxygen", "/**\n * \\brief Summary.\n * \\deprecated Use new_method.\n */"},
		{"xmldoc", "/// <summary>Summary.</summary>"},
		{"yard", "# Summary.\n# @deprecated Use new_method instead."},
	}

	parsers := map[string]docparser.Parser{
		"phpdoc":  phpdoc.New(),
		"javadoc": javadoc.New(),
		"jsdoc":   jsdoc.New(),
		"pydoc":   pydoc.New(),
		"godoc":   godoc.New(),
		"rustdoc": rustdoc.New(),
		"doxygen": doxygen.New(),
		"xmldoc":  xmldoc.New(),
		"yard":    yard.New(),
	}

	for _, tc := range tests {
		t.Run(tc.parser, func(t *testing.T) {
			p := parsers[tc.parser]
			doc := p.Parse(tc.input)
			if doc == nil {
				t.Fatal("Parse returned nil")
			}
			// Skip xmldoc - it doesn't have a standard deprecated element
			if tc.parser == "xmldoc" {
				return
			}
			if doc.Deprecated == "" {
				t.Error("Deprecated is empty despite input containing deprecation tag")
			}
		})
	}
}

// TestAllParsers_ParameterExtraction verifies parsers extract parameters.
func TestAllParsers_ParameterExtraction(t *testing.T) {
	tests := []struct {
		parser string
		input  string
	}{
		{"phpdoc", "/**\n * Summary.\n * @param string $name The name.\n * @param int $age The age.\n */"},
		{"javadoc", "/**\n * Summary.\n * @param name The name.\n * @param age The age.\n */"},
		{"jsdoc", "/**\n * Summary.\n * @param {string} name - The name.\n * @param {number} age - The age.\n */"},
		{"pydoc", "Summary.\n\nArgs:\n    name (str): The name.\n    age (int): The age."},
		{"doxygen", "/**\n * \\brief Summary.\n * \\param name The name.\n * \\param age The age.\n */"},
		{"xmldoc", "/// <summary>Summary.</summary>\n/// <param name=\"name\">The name.</param>\n/// <param name=\"age\">The age.</param>"},
		{"yard", "# Summary.\n# @param name [String] the name\n# @param age [Integer] the age"},
	}

	parsers := map[string]docparser.Parser{
		"phpdoc":  phpdoc.New(),
		"javadoc": javadoc.New(),
		"jsdoc":   jsdoc.New(),
		"pydoc":   pydoc.New(),
		"doxygen": doxygen.New(),
		"xmldoc":  xmldoc.New(),
		"yard":    yard.New(),
	}

	for _, tc := range tests {
		t.Run(tc.parser, func(t *testing.T) {
			p := parsers[tc.parser]
			doc := p.Parse(tc.input)
			if doc == nil {
				t.Fatal("Parse returned nil")
			}
			if len(doc.Params) < 2 {
				t.Errorf("Expected at least 2 params, got %d", len(doc.Params))
				return
			}
			// All params should have non-empty Name
			for i, param := range doc.Params {
				if param.Name == "" {
					t.Errorf("Param[%d].Name is empty", i)
				}
			}
		})
	}
}

// TestAllParsers_ReturnExtraction verifies parsers extract return info.
func TestAllParsers_ReturnExtraction(t *testing.T) {
	tests := []struct {
		parser string
		input  string
	}{
		{"phpdoc", "/**\n * Summary.\n * @return string The result.\n */"},
		{"javadoc", "/**\n * Summary.\n * @return The result string.\n */"},
		{"jsdoc", "/**\n * Summary.\n * @returns {string} The result.\n */"},
		{"pydoc", "Summary.\n\nReturns:\n    str: The result."},
		{"doxygen", "/**\n * \\brief Summary.\n * \\return The result string.\n */"},
		{"xmldoc", "/// <summary>Summary.</summary>\n/// <returns>The result string.</returns>"},
		{"yard", "# Summary.\n# @return [String] the result"},
	}

	parsers := map[string]docparser.Parser{
		"phpdoc":  phpdoc.New(),
		"javadoc": javadoc.New(),
		"jsdoc":   jsdoc.New(),
		"pydoc":   pydoc.New(),
		"doxygen": doxygen.New(),
		"xmldoc":  xmldoc.New(),
		"yard":    yard.New(),
	}

	for _, tc := range tests {
		t.Run(tc.parser, func(t *testing.T) {
			p := parsers[tc.parser]
			doc := p.Parse(tc.input)
			if doc == nil {
				t.Fatal("Parse returned nil")
			}
			if doc.Returns == nil {
				t.Error("Returns is nil despite input having return documentation")
			}
		})
	}
}

// TestAllParsers_ConcurrentParse verifies parsers are safe for concurrent use.
func TestAllParsers_ConcurrentParse(t *testing.T) {
	input := "/**\n * Summary text.\n * @param name The name.\n * @return The result.\n */"

	for _, p := range allParsers() {
		t.Run(p.name, func(t *testing.T) {
			const goroutines = 50
			done := make(chan struct{})
			for i := 0; i < goroutines; i++ {
				go func() {
					doc := p.parser.Parse(input)
					_ = doc
					done <- struct{}{}
				}()
			}
			for i := 0; i < goroutines; i++ {
				<-done
			}
		})
	}
}
