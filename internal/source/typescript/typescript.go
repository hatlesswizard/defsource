// Package typescript provides a Source interface implementation for parsing
// TypeScript source code from any TypeScript-based library (Angular, NestJS,
// RxJS, Zod, etc.). It uses tree-sitter for AST parsing and the shared JSDoc
// parser for documentation extraction.
package typescript

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser/jsdoc"
	"github.com/hatlesswizard/defsource/internal/source"
)

// Config holds the configuration for a TypeScript source adapter.
// Each TypeScript library (Angular, NestJS, RxJS, etc.) provides its
// own Config with appropriate repository and directory settings.
type Config struct {
	// LibraryID is the canonical library ID (e.g., "typescript/angular").
	LibraryID string

	// Name is the human-readable library name (e.g., "Angular").
	Name string

	// Description is a brief library description.
	Description string

	// SourceURL is the canonical URL (e.g., "https://github.com/angular/angular").
	SourceURL string

	// Owner is the GitHub repository owner.
	Owner string

	// Repo is the GitHub repository name.
	Repo string

	// SourceDirs are relative directories within the repo to scan.
	// If empty, defaults to ["src/"].
	SourceDirs []string

	// ExcludeDirs are directory names to skip during discovery.
	// Always includes: node_modules, test, tests, __tests__, dist, build, .git
	ExcludeDirs []string

	// IncludeDeclarationFiles controls whether .d.ts files are indexed.
	// Useful for type-only libraries.
	IncludeDeclarationFiles bool
}

// Source is a documentation source adapter that reads TypeScript source code
// from a local clone of a GitHub repository. It uses tree-sitter for AST
// parsing and JSDoc for documentation extraction.
type Source struct {
	cfg      Config
	repoPath string
	ref      string
	index    *codebaseIndex
	jsdoc    *jsdoc.Parser
}

var _ source.Source = (*Source)(nil)

// Option customises a Source at construction time.
type Option func(*Source)

// WithRef sets the git ref (tag or branch) for source-code links.
func WithRef(ref string) Option {
	return func(s *Source) { s.ref = ref }
}

// New constructs a new TypeScript Source for the given config and local repo path.
func New(cfg Config, repoPath string, opts ...Option) *Source {
	if len(cfg.SourceDirs) == 0 {
		cfg.SourceDirs = []string{"src/"}
	}
	s := &Source{
		cfg:      cfg,
		repoPath: repoPath,
		index:    emptyIndex(),
		jsdoc:    jsdoc.New(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ID returns the canonical library ID.
func (s *Source) ID() string { return s.cfg.LibraryID }

// Meta returns metadata for the library record.
func (s *Source) Meta() source.LibraryMeta {
	version := s.ref
	if version == "" {
		version = "unknown"
	}
	return source.LibraryMeta{
		Name:        s.cfg.Name,
		Description: s.cfg.Description,
		SourceURL:   s.cfg.SourceURL,
		Version:     version,
		Language:    "typescript",
		TrustScore:  0.90,
	}
}

// blobBase returns the GitHub blob URL prefix for source-code links.
func (s *Source) blobBase() string {
	ref := s.ref
	if ref == "" {
		ref = "main"
	}
	return fmt.Sprintf("https://github.com/%s/%s/blob/%s/", s.cfg.Owner, s.cfg.Repo, ref)
}

// DiscoverEntities walks the local repository and returns identifiers for all
// exported TypeScript entities. The fetch parameter is unused -- all discovery
// reads files locally.
func (s *Source) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	idx, err := buildCodebaseIndex(s.repoPath, s.cfg)
	if err != nil {
		return nil, fmt.Errorf("buildCodebaseIndex: %w", err)
	}
	s.index = idx
	ids := idx.buildEntityList()
	log.Printf("Discovered %d TypeScript entities from %s", len(ids), s.repoPath)
	return ids, nil
}

// ParseEntity parses a single entity from file content.
func (s *Source) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	filePath, fragment := splitFragment(entityID)
	if fragment == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	analysis := parseFile(content, filePath)
	if analysis == nil {
		return nil, nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Find the entity by name
	for _, cls := range analysis.Classes {
		if cls.Name == fragment {
			return s.buildClassEntity(cls, content, filePath, entityID)
		}
	}
	for _, iface := range analysis.Interfaces {
		if iface.Name == fragment {
			return s.buildInterfaceEntity(iface, content, filePath, entityID)
		}
	}
	for _, ta := range analysis.TypeAliases {
		if ta.Name == fragment {
			return s.buildTypeAliasEntity(ta, content, filePath)
		}
	}
	for _, en := range analysis.Enums {
		if en.Name == fragment {
			return s.buildEnumEntity(en, content, filePath)
		}
	}
	for _, fn := range analysis.Functions {
		if fn.Name == fragment {
			return s.buildFunctionEntity(fn, content, filePath)
		}
	}
	for _, ns := range analysis.Namespaces {
		if ns.Name == fragment {
			return s.buildNamespaceEntity(ns, content, filePath)
		}
	}

	return nil, nil, fmt.Errorf("entity %q not found in %s", fragment, filePath)
}

// ParseMethod parses a single method from file content.
func (s *Source) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	filePath, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	// Fragment format: ClassName.methodName or ClassName::methodName
	className, methodName, found := splitMethodFragment(fragment)
	if !found {
		return nil, fmt.Errorf("invalid method fragment %q: expected ClassName.methodName", fragment)
	}

	analysis := parseFile(content, filePath)
	if analysis == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Look in classes
	for _, cls := range analysis.Classes {
		if cls.Name == className {
			for _, m := range cls.Methods {
				if m.Name == methodName {
					return s.buildMethod(m, className, content, filePath), nil
				}
			}
			return nil, fmt.Errorf("method %q not found in class %q", methodName, className)
		}
	}

	// Look in interfaces
	for _, iface := range analysis.Interfaces {
		if iface.Name == className {
			for _, m := range iface.Methods {
				if m.Name == methodName {
					return s.buildMethod(m, className, content, filePath), nil
				}
			}
			return nil, fmt.Errorf("method %q not found in interface %q", methodName, className)
		}
	}

	return nil, fmt.Errorf("class/interface %q not found in %s", className, filePath)
}

// DetectWrapper analyzes a method's source code for wrapper patterns.
func (s *Source) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil || method.SourceCode == "" {
		return false, "", ""
	}
	return detectWrapper(method.SourceCode, s.index)
}

// ResolveWrapperURL constructs an identifier to fetch the wrapped target's source.
func (s *Source) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "function":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
	case "method":
		// targetName is "ClassName.methodName"
		parts := strings.SplitN(targetName, ".", 2)
		if len(parts) == 2 {
			className := parts[0]
			if path, ok := s.index.FileForClass(className); ok {
				return path + "#" + className + "." + parts[1]
			}
		}
	case "re_export":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
		if path, ok := s.index.FileForClass(targetName); ok {
			return path + "#" + targetName
		}
	}
	return ""
}

// ParseSourceCode extracts just the source code of a specific entity or method
// from file content.
func (s *Source) ParseSourceCode(entityID string, content []byte) (string, error) {
	filePath, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	analysis := parseFile(content, filePath)
	if analysis == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	// Check for method fragment (ClassName.methodName)
	if className, methodName, ok := splitMethodFragment(fragment); ok {
		for _, cls := range analysis.Classes {
			if cls.Name == className {
				for _, m := range cls.Methods {
					if m.Name == methodName {
						return extractSource(content, m.StartPos, m.EndPos), nil
					}
				}
			}
		}
		for _, iface := range analysis.Interfaces {
			if iface.Name == className {
				for _, m := range iface.Methods {
					if m.Name == methodName {
						return extractSource(content, m.StartPos, m.EndPos), nil
					}
				}
			}
		}
		return "", fmt.Errorf("method %q.%q not found", className, methodName)
	}

	// Look for top-level entities
	for _, cls := range analysis.Classes {
		if cls.Name == fragment {
			return extractSource(content, cls.StartPos, cls.EndPos), nil
		}
	}
	for _, iface := range analysis.Interfaces {
		if iface.Name == fragment {
			return extractSource(content, iface.StartPos, iface.EndPos), nil
		}
	}
	for _, ta := range analysis.TypeAliases {
		if ta.Name == fragment {
			return extractSource(content, ta.StartPos, ta.EndPos), nil
		}
	}
	for _, en := range analysis.Enums {
		if en.Name == fragment {
			return extractSource(content, en.StartPos, en.EndPos), nil
		}
	}
	for _, fn := range analysis.Functions {
		if fn.Name == fragment {
			return extractSource(content, fn.StartPos, fn.EndPos), nil
		}
	}
	for _, ns := range analysis.Namespaces {
		if ns.Name == fragment {
			return extractSource(content, ns.StartPos, ns.EndPos), nil
		}
	}

	return "", fmt.Errorf("entity %q not found", fragment)
}

// buildClassEntity creates an Entity from a parsed class definition.
func (s *Source) buildClassEntity(cls classDef, content []byte, filePath, entityID string) (*source.Entity, []string, error) {
	doc := s.jsdoc.Parse(cls.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, cls.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	properties := make([]source.Property, 0, len(cls.Properties))
	for _, p := range cls.Properties {
		pdoc := s.jsdoc.Parse(p.DocComment)
		properties = append(properties, source.Property{
			Name:        p.Name,
			Type:        p.Type,
			Description: pdoc.Summary,
			Visibility:  p.Visibility,
		})
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(cls.Name),
		Name:        cls.Name,
		Kind:        source.KindClass,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  extractSource(content, cls.StartPos, cls.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}

	methodIDs := make([]string, 0, len(cls.Methods))
	for _, m := range cls.Methods {
		methodIDs = append(methodIDs, filePath+"#"+cls.Name+"."+m.Name)
	}

	return entity, methodIDs, nil
}

// buildInterfaceEntity creates an Entity from a parsed interface definition.
func (s *Source) buildInterfaceEntity(iface interfaceDef, content []byte, filePath, entityID string) (*source.Entity, []string, error) {
	doc := s.jsdoc.Parse(iface.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, iface.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	properties := make([]source.Property, 0, len(iface.Properties))
	for _, p := range iface.Properties {
		pdoc := s.jsdoc.Parse(p.DocComment)
		properties = append(properties, source.Property{
			Name:        p.Name,
			Type:        p.Type,
			Description: pdoc.Summary,
		})
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(iface.Name),
		Name:        iface.Name,
		Kind:        source.KindInterface,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  extractSource(content, iface.StartPos, iface.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}

	methodIDs := make([]string, 0, len(iface.Methods))
	for _, m := range iface.Methods {
		methodIDs = append(methodIDs, filePath+"#"+iface.Name+"."+m.Name)
	}

	return entity, methodIDs, nil
}

// buildTypeAliasEntity creates an Entity from a parsed type alias definition.
func (s *Source) buildTypeAliasEntity(ta typeAliasDef, content []byte, filePath string) (*source.Entity, []string, error) {
	doc := s.jsdoc.Parse(ta.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, ta.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(ta.Name),
		Name:        ta.Name,
		Kind:        source.KindTypeAlias,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  extractSource(content, ta.StartPos, ta.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

// buildEnumEntity creates an Entity from a parsed enum definition.
func (s *Source) buildEnumEntity(en enumDef, content []byte, filePath string) (*source.Entity, []string, error) {
	doc := s.jsdoc.Parse(en.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, en.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	properties := make([]source.Property, 0, len(en.Members))
	for _, m := range en.Members {
		properties = append(properties, source.Property{
			Name: m.Name,
			Type: m.Value,
		})
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(en.Name),
		Name:        en.Name,
		Kind:        source.KindEnum,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  extractSource(content, en.StartPos, en.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}

	return entity, nil, nil
}

// buildFunctionEntity creates an Entity from a parsed function definition.
func (s *Source) buildFunctionEntity(fn functionDef, content []byte, filePath string) (*source.Entity, []string, error) {
	doc := s.jsdoc.Parse(fn.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, fn.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(fn.Name),
		Name:        fn.Name,
		Kind:        source.KindFunction,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  extractSource(content, fn.StartPos, fn.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

// buildNamespaceEntity creates an Entity from a parsed namespace/module definition.
func (s *Source) buildNamespaceEntity(ns namespaceDef, content []byte, filePath string) (*source.Entity, []string, error) {
	doc := s.jsdoc.Parse(ns.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, ns.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(ns.Name),
		Name:        ns.Name,
		Kind:        source.KindNamespace,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  extractSource(content, ns.StartPos, ns.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

// buildMethod creates a Method from a parsed method definition.
func (s *Source) buildMethod(m methodDef, parentName string, content []byte, filePath string) *source.Method {
	doc := s.jsdoc.Parse(m.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, m.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	parameters := make([]source.Parameter, 0, len(doc.Params))
	for _, p := range doc.Params {
		parameters = append(parameters, source.Parameter{
			Name:        p.Name,
			Type:        p.Type,
			Required:    !p.Optional,
			Description: p.Description,
		})
	}

	// If no JSDoc params, use the AST-extracted params
	if len(parameters) == 0 {
		for _, p := range m.Params {
			parameters = append(parameters, source.Parameter{
				Name:     p.Name,
				Type:     p.Type,
				Required: !p.Optional,
			})
		}
	}

	var returnType, returnDesc string
	if doc.Returns != nil {
		returnType = doc.Returns.Type
		returnDesc = doc.Returns.Description
	} else if m.ReturnType != "" {
		returnType = m.ReturnType
	}

	signature := buildMethodSignature(parentName, m)

	var relations []source.Relation
	for _, see := range doc.See {
		relations = append(relations, source.Relation{
			Kind:       "uses",
			TargetName: see,
		})
	}

	return &source.Method{
		Slug:        strings.ToLower(m.Name),
		Name:        m.Name,
		Signature:   signature,
		Description: doc.Description,
		Parameters:  parameters,
		ReturnType:  returnType,
		ReturnDesc:  returnDesc,
		SourceCode:  extractSource(content, m.StartPos, m.EndPos),
		URL:         githubURL,
		Since:       doc.Since,
		Deprecated:  doc.Deprecated != "",
		Relations:   relations,
	}
}

// splitFragment splits an entityID into base path and fragment.
func splitFragment(id string) (base, fragment string) {
	base, fragment, _ = strings.Cut(id, "#")
	return base, fragment
}

// splitMethodFragment splits a method fragment like "ClassName.methodName" or
// "ClassName::methodName" into class name and method name.
func splitMethodFragment(fragment string) (className, methodName string, ok bool) {
	if idx := strings.LastIndex(fragment, "."); idx > 0 {
		return fragment[:idx], fragment[idx+1:], true
	}
	if className, methodName, found := strings.Cut(fragment, "::"); found {
		return className, methodName, true
	}
	return "", "", false
}

// relativePath returns absPath relative to repoPath, using forward slashes.
func relativePath(repoPath, absPath string) string {
	if repoPath == "" {
		return absPath
	}
	rel, err := filepath.Rel(repoPath, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}

// lineNumber returns the 1-indexed line number for the byte offset within content.
func lineNumber(content []byte, pos int) int {
	if pos < 0 {
		return 0
	}
	if pos > len(content) {
		pos = len(content)
	}
	count := 1
	for i := range pos {
		if content[i] == '\n' {
			count++
		}
	}
	return count
}

// extractSource extracts a source code slice from content.
func extractSource(content []byte, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(content) {
		end = len(content)
	}
	if start >= end {
		return ""
	}
	return string(content[start:end])
}

// buildMethodSignature constructs a TypeScript method signature.
func buildMethodSignature(parentName string, m methodDef) string {
	var parts []string
	if m.Static {
		parts = append(parts, "static")
	}
	if m.Async {
		parts = append(parts, "async")
	}

	params := make([]string, 0, len(m.Params))
	for _, p := range m.Params {
		param := p.Name
		if p.Optional {
			param += "?"
		}
		if p.Type != "" {
			param += ": " + p.Type
		}
		params = append(params, param)
	}

	sig := parentName + "." + m.Name
	if m.TypeParams != "" {
		sig += "<" + m.TypeParams + ">"
	}
	sig += "(" + strings.Join(params, ", ") + ")"
	if m.ReturnType != "" {
		sig += ": " + m.ReturnType
	}

	if len(parts) > 0 {
		sig = strings.Join(parts, " ") + " " + sig
	}
	return sig
}

// detectVersion tries to read the version from package.json in the repo.
func detectVersion(repoPath string) string {
	if repoPath == "" {
		return "unknown"
	}
	pkgJSON := filepath.Join(repoPath, "package.json")
	content, err := os.ReadFile(pkgJSON)
	if err != nil {
		return "unknown"
	}
	// Simple extraction without pulling in encoding/json
	idx := strings.Index(string(content), `"version"`)
	if idx < 0 {
		return "unknown"
	}
	rest := string(content[idx+9:])
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return "unknown"
	}
	rest = strings.TrimSpace(rest[colon+1:])
	if len(rest) < 2 || rest[0] != '"' {
		return "unknown"
	}
	end := strings.Index(rest[1:], `"`)
	if end < 0 {
		return "unknown"
	}
	return rest[1 : end+1]
}
