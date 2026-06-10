// Package docparser provides a unified documentation comment parsing
// infrastructure with sub-packages for each documentation format across
// multiple programming languages.
package docparser

// DocComment holds structured documentation extracted from any doc comment
// format. All language-specific parsers produce this common type.
type DocComment struct {
	Summary     string
	Description string
	Params      []ParamDoc
	Returns     *ReturnDoc
	Deprecated  string
	Since       string
	See         []string
	Examples    []string
	Throws      []ThrowDoc
}

// ParamDoc holds a single parameter documentation entry.
type ParamDoc struct {
	Name        string
	Type        string
	Description string
	Optional    bool
	Default     string
}

// ReturnDoc holds the return value documentation.
type ReturnDoc struct {
	Type        string
	Description string
}

// ThrowDoc holds a single exception/error documentation entry.
type ThrowDoc struct {
	Type        string
	Description string
}

// Parser is the interface that all documentation format parsers implement.
type Parser interface {
	Parse(raw string) *DocComment
}
