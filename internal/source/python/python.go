// Package python provides a documentation source adapter that reads
// Python source code from a local checkout of a Python framework
// repository (Django, Flask, FastAPI, etc.). It parses raw Python source
// via tree-sitter to extract classes, functions, async functions, methods,
// properties, and docstrings without making any network calls during parsing.
package python

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser/pydoc"
	"github.com/hatlesswizard/defsource/internal/source"
)

// PythonSource is a documentation source adapter that reads Python source
// code from a local repository checkout. It is configurable for any Python
// framework (Django, Flask, FastAPI, etc.).
type PythonSource struct {
	repoPath  string
	libraryID string // e.g., "/python/django"
	meta      source.LibraryMeta
	ref       string // git tag/branch for source-code links
	index     *codebaseIndex
	docParser *pydoc.Parser
}

var _ source.Source = (*PythonSource)(nil)

// Option customises a PythonSource at construction time.
type Option func(*PythonSource)

// WithRef sets the git ref (tag or branch) for source-code links,
// e.g. WithRef("4.2") produces links under .../blob/4.2/.
func WithRef(ref string) Option {
	return func(s *PythonSource) { s.ref = ref }
}

// Config holds the framework-specific configuration for a PythonSource.
type Config struct {
	// LibraryID is the canonical identifier (e.g., "/python/django").
	LibraryID string

	// Name is the display name for the library (e.g., "Django Reference").
	Name string

	// Description is the library description.
	Description string

	// SourceURL is the GitHub URL for the framework (e.g., "https://github.com/django/django").
	SourceURL string

	// Version is the version string for the library.
	Version string

	// TrustScore is the relevance trust score (0-1).
	TrustScore float64

	// SourceRoots are the directory paths relative to the repo root that
	// contain the framework's source code (e.g., ["django"] for Django,
	// ["flask"] for Flask, ["src/fastapi"] for FastAPI).
	SourceRoots []string
}

// New constructs a new PythonSource for the given local repository path
// and framework configuration.
func New(repoPath string, cfg Config, opts ...Option) *PythonSource {
	s := &PythonSource{
		repoPath:  repoPath,
		libraryID: cfg.LibraryID,
		meta: source.LibraryMeta{
			Name:        cfg.Name,
			Description: cfg.Description,
			SourceURL:   cfg.SourceURL,
			Version:     cfg.Version,
			Language:    "python",
			TrustScore:  cfg.TrustScore,
		},
		index:     emptyIndex(),
		docParser: pydoc.New(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// blobBase returns the GitHub blob URL prefix for source-code links.
func (s *PythonSource) blobBase() string {
	ref := s.ref
	if ref == "" {
		ref = "main"
	}
	return strings.TrimSuffix(s.meta.SourceURL, "/") + "/blob/" + ref + "/"
}

// ID returns the canonical library ID.
func (s *PythonSource) ID() string { return s.libraryID }

// Meta returns metadata for the library record.
func (s *PythonSource) Meta() source.LibraryMeta { return s.meta }

// DiscoverEntities walks the local repository and returns a sorted list
// of entity identifiers in the form "filepath#EntityName".
func (s *PythonSource) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	roots := make([]string, 0, len(s.index.sourceRoots))
	if len(s.index.sourceRoots) > 0 {
		for _, r := range s.index.sourceRoots {
			roots = append(roots, filepath.Join(s.repoPath, r))
		}
	} else {
		// Infer roots from the config stored during construction
		roots = s.inferRoots()
	}

	idx, err := buildCodebaseIndex(s.repoPath, roots)
	if err != nil {
		return nil, fmt.Errorf("buildCodebaseIndex: %w", err)
	}
	s.index = idx
	ids := idx.buildEntityList()
	log.Printf("Discovered %d entities from %s", len(ids), s.repoPath)
	return ids, nil
}

// inferRoots returns the source roots based on the library configuration.
func (s *PythonSource) inferRoots() []string {
	// Use repoPath itself as the sole root if nothing else is available
	return []string{s.repoPath}
}

// ParseEntity parses a single entity from file content, identified by entityID.
// The entityID is of the form "filepath#EntityName".
func (s *PythonSource) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	filePath, fragment := splitFragment(entityID)
	if fragment == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Look for a class first
	for i := range analysis.Classes {
		cls := &analysis.Classes[i]
		if cls.Name == fragment {
			return s.buildClassEntity(cls, content, filePath)
		}
	}

	// Then look for a function
	for i := range analysis.Functions {
		fn := &analysis.Functions[i]
		if fn.Name == fragment {
			return s.buildFunctionEntity(fn, content, filePath)
		}
	}

	return nil, nil, fmt.Errorf("entity %q not found in %s", fragment, filePath)
}

// buildClassEntity constructs an Entity and method list from a parsed class.
func (s *PythonSource) buildClassEntity(cls *classDef, content []byte, filePath string) (*source.Entity, []string, error) {
	var docStr string
	if cls.Docstring != "" {
		docStr = cls.Docstring
	}

	doc := s.docParser.Parse(docStr)
	kind := classKind(cls)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + filepath.ToSlash(rel)
	if cls.StartLine > 0 {
		githubURL += fmt.Sprintf("#L%d", cls.StartLine)
	}

	properties := make([]source.Property, 0, len(cls.Properties))
	for _, p := range cls.Properties {
		properties = append(properties, source.Property{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Visibility:  p.Visibility,
		})
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(cls.Name),
		Name:        cls.Name,
		Kind:        kind,
		Description: doc.Summary,
		SourceFile:  rel,
		SourceCode:  string(content[cls.StartPos:cls.EndPos]),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}

	// Build method IDs
	var methodIDs []string
	for _, m := range cls.Methods {
		if strings.HasPrefix(m.Name, "_") && m.Name != "__init__" {
			continue
		}
		methodID := filePath + "#" + cls.Name + "." + m.Name
		methodIDs = append(methodIDs, methodID)
	}

	return entity, methodIDs, nil
}

// buildFunctionEntity constructs an Entity from a parsed function.
func (s *PythonSource) buildFunctionEntity(fn *functionDef, content []byte, filePath string) (*source.Entity, []string, error) {
	doc := s.docParser.Parse(fn.Docstring)

	kind := source.KindFunction
	if fn.Async {
		kind = "async_function"
	}

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + filepath.ToSlash(rel)
	if fn.StartLine > 0 {
		githubURL += fmt.Sprintf("#L%d", fn.StartLine)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(fn.Name),
		Name:        fn.Name,
		Kind:        kind,
		Description: doc.Summary,
		SourceFile:  rel,
		SourceCode:  string(content[fn.StartPos:fn.EndPos]),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

// ParseMethod parses a single method/function from file content.
// The methodID is of the form "filepath#ClassName.methodName".
func (s *PythonSource) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	filePath, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	className, methodName, ok := strings.Cut(fragment, ".")
	if !ok {
		return nil, fmt.Errorf("invalid method fragment %q: expected ClassName.methodName", fragment)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}

	var cls *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].Name == className {
			cls = &analysis.Classes[i]
			break
		}
	}
	if cls == nil {
		return nil, fmt.Errorf("class %q not found in %s", className, filePath)
	}

	var method *methodDef
	for i := range cls.Methods {
		if cls.Methods[i].Name == methodName {
			method = &cls.Methods[i]
			break
		}
	}
	if method == nil {
		return nil, fmt.Errorf("method %q not found in class %q", methodName, className)
	}

	doc := s.docParser.Parse(method.Docstring)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + filepath.ToSlash(rel)
	if method.StartLine > 0 {
		githubURL += fmt.Sprintf("#L%d", method.StartLine)
	}

	parameters := make([]source.Parameter, 0, len(method.Params))
	for _, p := range method.Params {
		if p.Name == "self" || p.Name == "cls" {
			continue
		}
		param := source.Parameter{
			Name:     p.Name,
			Type:     p.Type,
			Required: p.Default == "",
		}
		// Enrich from docstring if available
		for _, dp := range doc.Params {
			if dp.Name == p.Name {
				param.Description = dp.Description
				if param.Type == "" {
					param.Type = dp.Type
				}
				break
			}
		}
		parameters = append(parameters, param)
	}

	signature := buildMethodSignature(className, method)

	var returnType, returnDesc string
	if doc.Returns != nil {
		returnType = doc.Returns.Type
		returnDesc = doc.Returns.Description
	}
	if returnType == "" && method.ReturnType != "" {
		returnType = method.ReturnType
	}

	var relations []source.Relation
	for _, see := range doc.See {
		relations = append(relations, source.Relation{
			Kind:       "uses",
			TargetName: see,
		})
	}

	return &source.Method{
		Slug:        strings.ToLower(method.Name),
		Name:        method.Name,
		Signature:   signature,
		Description: doc.Summary,
		Parameters:  parameters,
		ReturnType:  returnType,
		ReturnDesc:  returnDesc,
		SourceCode:  string(content[method.StartPos:method.EndPos]),
		URL:         githubURL,
		Since:       doc.Since,
		Deprecated:  doc.Deprecated != "",
		Relations:   relations,
	}, nil
}

// DetectWrapper analyzes a method's source code for wrapper patterns.
func (s *PythonSource) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil || method.SourceCode == "" {
		return false, "", ""
	}
	return detectWrapper([]byte(method.SourceCode), s.index)
}

// ResolveWrapperURL constructs the identifier to fetch the wrapped target's source.
func (s *PythonSource) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "function":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
	case "method":
		// targetName is "ClassName.methodName"
		if className, _, ok := strings.Cut(targetName, "."); ok {
			if path, ok := s.index.FileForClass(className); ok {
				return path + "#" + targetName
			}
		}
	case "delegate_method":
		// targetName is "attribute.methodName" — look up the class by entitySlug
		if path, canonical, ok := s.index.LookupClass(entitySlug); ok {
			return path + "#" + canonical + "." + targetName
		}
	}
	return ""
}

// ParseSourceCode extracts just the source code of a specific entity or
// method from file content, using the fragment in the identifier.
func (s *PythonSource) ParseSourceCode(entityID string, content []byte) (string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	analysis := parseFile(content)
	if analysis == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	// Try "ClassName.methodName" first
	if className, methodName, ok := strings.Cut(fragment, "."); ok {
		for i := range analysis.Classes {
			if analysis.Classes[i].Name == className {
				for _, m := range analysis.Classes[i].Methods {
					if m.Name == methodName {
						return string(content[m.StartPos:m.EndPos]), nil
					}
				}
				return "", fmt.Errorf("method %q not found in class %q", methodName, className)
			}
		}
	}

	// Try as a class name
	for _, cls := range analysis.Classes {
		if cls.Name == fragment {
			return string(content[cls.StartPos:cls.EndPos]), nil
		}
	}

	// Try as a function name
	for _, fn := range analysis.Functions {
		if fn.Name == fragment {
			return string(content[fn.StartPos:fn.EndPos]), nil
		}
	}

	return "", fmt.Errorf("entity %q not found", fragment)
}

// classKind determines the entity kind for a Python class based on its
// decorators and base classes.
func classKind(cls *classDef) string {
	for _, dec := range cls.Decorators {
		if dec == "dataclass" || dec == "dataclasses.dataclass" {
			return "dataclass"
		}
	}
	for _, base := range cls.Bases {
		if base == "Protocol" || strings.HasSuffix(base, ".Protocol") {
			return "protocol"
		}
		if base == "Enum" || base == "IntEnum" || base == "StrEnum" ||
			strings.HasSuffix(base, ".Enum") || strings.HasSuffix(base, ".IntEnum") || strings.HasSuffix(base, ".StrEnum") {
			return source.KindEnum
		}
	}
	return source.KindClass
}

// buildMethodSignature constructs a Python method signature string.
func buildMethodSignature(className string, method *methodDef) string {
	var b strings.Builder

	// Add decorators
	for _, dec := range method.Decorators {
		b.WriteString("@")
		b.WriteString(dec)
		b.WriteString("\n")
	}

	if method.Async {
		b.WriteString("async ")
	}
	b.WriteString("def ")
	b.WriteString(className)
	b.WriteString(".")
	b.WriteString(method.Name)
	b.WriteString("(")

	var parts []string
	for _, p := range method.Params {
		parts = append(parts, formatParam(p))
	}
	b.WriteString(strings.Join(parts, ", "))
	b.WriteString(")")

	if method.ReturnType != "" {
		b.WriteString(" -> ")
		b.WriteString(method.ReturnType)
	}

	return b.String()
}

// formatParam formats a single parameter for display in a signature.
func formatParam(p paramDef) string {
	var b strings.Builder
	if p.StarStar {
		b.WriteString("**")
	} else if p.Star {
		b.WriteString("*")
	}
	b.WriteString(p.Name)
	if p.Type != "" {
		b.WriteString(": ")
		b.WriteString(p.Type)
	}
	if p.Default != "" {
		b.WriteString(" = ")
		b.WriteString(p.Default)
	}
	return b.String()
}

// splitFragment splits "path#fragment" into (path, fragment).
func splitFragment(id string) (string, string) {
	if idx := strings.LastIndex(id, "#"); idx >= 0 {
		return id[:idx], id[idx+1:]
	}
	return id, ""
}

// relativePath returns the relative path from base to target.
func relativePath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}
