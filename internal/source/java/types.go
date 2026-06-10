package java

import "github.com/hatlesswizard/defsource/internal/docparser"

// fileAnalysis holds all type definitions extracted from one Java source file.
type fileAnalysis struct {
	Types []javaType
	Calls []callRef
}

// findType locates a type by name, supporting dotted notation for inner classes.
// e.g., "OuterClass.InnerClass" finds InnerClass nested within OuterClass.
func (fa *fileAnalysis) findType(name string) *javaType {
	if fa == nil {
		return nil
	}
	parts := splitDot(name)
	if len(parts) == 0 {
		return nil
	}

	// Find the top-level type.
	var top *javaType
	for i := range fa.Types {
		if fa.Types[i].Name == parts[0] {
			top = &fa.Types[i]
			break
		}
	}
	if top == nil {
		return nil
	}
	if len(parts) == 1 {
		return top
	}

	// Walk into nested types.
	current := top
	for _, part := range parts[1:] {
		found := false
		for i := range current.InnerTypes {
			if current.InnerTypes[i].Name == part {
				current = &current.InnerTypes[i]
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return current
}

// splitDot splits a dotted name into parts.
func splitDot(name string) []string {
	if name == "" {
		return nil
	}
	var parts []string
	start := 0
	depth := 0
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '<':
			depth++
		case '>':
			depth--
		case '.':
			if depth == 0 {
				parts = append(parts, name[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, name[start:])
	return parts
}

// javaType holds data extracted from a class, interface, enum, record, or
// annotation declaration.
type javaType struct {
	Name        string
	Kind        string // source.KindClass, KindInterface, KindEnum, KindRecord, KindAnnotation
	Visibility  string // "public", "protected", "private", "package"
	TypeParams  string // Generic type parameters (e.g., "<T extends Comparable<T>>")
	Extends     string // Superclass name
	Implements  []string
	Annotations []string
	Sealed      bool     // Java 17+ sealed class/interface
	Permits     []string // Permitted subclasses for sealed types
	Methods     []javaMethod
	Fields      []javaField
	InnerTypes  []javaType
	Doc         *docparser.DocComment
	StartPos    int
	EndPos      int
}

// javaMethod holds data extracted from a method or constructor declaration.
type javaMethod struct {
	Name        string
	Visibility  string // "public", "protected", "private", "package"
	Static      bool
	Abstract    bool
	Final       bool
	Default     bool   // default interface method
	TypeParams  string // method-level generics
	ReturnType  string
	Params      []javaParam
	Throws      []string
	Annotations []string
	Deprecated  bool
	Doc         *docparser.DocComment
	StartPos    int
	EndPos      int
}

// paramSignature returns a unique signature suffix for overloaded methods.
// Format: (Type1,Type2,...) with type names only (no param names).
func (m *javaMethod) paramSignature() string {
	if len(m.Params) == 0 {
		return "()"
	}
	var b []byte
	b = append(b, '(')
	for i, p := range m.Params {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, []byte(p.Type)...)
	}
	b = append(b, ')')
	return string(b)
}

// javaParam holds data for a single method parameter.
type javaParam struct {
	Name        string
	Type        string
	Variadic    bool   // true for Type... params
	HasDefault  bool   // always false in Java (no default params)
	Description string // from JavaDoc @param
}

// javaField holds data for a class field.
type javaField struct {
	Name        string
	Type        string
	Visibility  string
	Static      bool
	Final       bool
	Annotations []string
	Doc         *docparser.DocComment
	StartPos    int
	EndPos      int
}

// callRef is a lightweight call-site reference collected during AST walking.
type callRef struct {
	Name string
	Kind string // "method", "static", "constructor"
}
