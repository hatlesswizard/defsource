// Package cpp provides a documentation source adapter that parses C++ library
// source code from a local clone of any C++ GitHub repository (Boost, Qt, Abseil,
// etc.). It uses tree-sitter C++ grammar to extract classes, structs, functions,
// templates, namespaces, enums, type aliases, and concepts, and parses Doxygen
// documentation comments.
package cpp

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
	"github.com/hatlesswizard/defsource/internal/docparser/doxygen"
	"github.com/hatlesswizard/defsource/internal/source"
)

// Config holds the configuration for a C++ library source.
type Config struct {
	// RepoPath is the local filesystem path to the downloaded source.
	RepoPath string

	// LibraryID is the canonical library ID (e.g., "cpp/boost", "cpp/abseil").
	LibraryID string

	// Name is the display name (e.g., "Boost", "Abseil").
	Name string

	// Description is a short description of the library.
	Description string

	// SourceURL is the GitHub URL (e.g., "https://github.com/abseil/abseil-cpp").
	SourceURL string

	// Version is the library version tag.
	Version string

	// IncludeDirs lists relative directory paths that contain public headers.
	// If empty, defaults to {"include"}.
	IncludeDirs []string

	// SkipDirs lists relative directory paths to exclude from discovery.
	// Common patterns like "internal", "detail", "impl" are always skipped.
	SkipDirs []string

	// Ref is the git ref for source-code links. Defaults to "main".
	Ref string
}

// Source is the C++ documentation source adapter implementing source.Source.
type Source struct {
	cfg       Config
	index     *codebaseIndex
	doxygenP  *doxygen.Parser
}

var _ source.Source = (*Source)(nil)

// New constructs a new C++ Source with the given config.
func New(cfg Config) *Source {
	if len(cfg.IncludeDirs) == 0 {
		cfg.IncludeDirs = []string{"include"}
	}
	if cfg.Ref == "" {
		cfg.Ref = "main"
	}
	return &Source{
		cfg:      cfg,
		index:    emptyIndex(),
		doxygenP: doxygen.New(),
	}
}

// ID returns the canonical library ID.
func (s *Source) ID() string { return s.cfg.LibraryID }

// Meta returns metadata for the library record.
func (s *Source) Meta() source.LibraryMeta {
	return source.LibraryMeta{
		Name:        s.cfg.Name,
		Description: s.cfg.Description,
		SourceURL:   s.cfg.SourceURL,
		Version:     s.cfg.Version,
		Language:    "cpp",
		TrustScore:  0.90,
	}
}

// DiscoverEntities walks the local repository and returns entity identifiers.
func (s *Source) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	idx, err := buildCodebaseIndex(s.cfg.RepoPath, s.cfg.IncludeDirs, s.cfg.SkipDirs)
	if err != nil {
		return nil, fmt.Errorf("buildCodebaseIndex: %w", err)
	}
	s.index = idx
	ids := idx.buildEntityList()
	log.Printf("Discovered %d C++ entities from %s", len(ids), s.cfg.RepoPath)
	return ids, nil
}

// ParseEntity parses a single entity from file content.
func (s *Source) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	filePath, fragment := splitFragment(entityID)
	if fragment == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Find the matching entity by qualified name
	entity, methods := s.findEntity(analysis, fragment, filePath, content)
	if entity == nil {
		return nil, nil, fmt.Errorf("entity %q not found in %s", fragment, filePath)
	}

	return entity, methods, nil
}

// ParseMethod parses a single method/function from file content.
func (s *Source) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	filePath, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Fragment is "Namespace::Class::method" or "Namespace::function"
	method := s.findMethod(analysis, fragment, filePath, content)
	if method == nil {
		return nil, fmt.Errorf("method %q not found in %s", fragment, filePath)
	}

	return method, nil
}

// DetectWrapper analyzes a method for wrapper/delegation patterns.
func (s *Source) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil || method.SourceCode == "" {
		return false, "", ""
	}
	return detectWrapper(method.SourceCode, s.index)
}

// ResolveWrapperURL constructs a target identifier for a wrapped function.
func (s *Source) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "function":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
	case "method":
		// targetName is "Class::method" — look up class file
		parts := strings.SplitN(targetName, "::", 2)
		if len(parts) != 2 {
			return ""
		}
		className := parts[0]
		if path, ok := s.index.FileForClass(className); ok {
			return path + "#" + targetName
		}
	}
	return ""
}

// ParseSourceCode extracts the source code of a specific entity.
func (s *Source) ParseSourceCode(entityID string, content []byte) (string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	analysis := parseFile(content)
	if analysis == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	// Search classes
	for _, cls := range analysis.Classes {
		if cls.QualifiedName == fragment {
			return extractSourceRange(content, cls.StartPos, cls.EndPos), nil
		}
		for _, m := range cls.Methods {
			methodQN := cls.QualifiedName + "::" + m.Name
			if methodQN == fragment {
				return extractSourceRange(content, m.StartPos, m.EndPos), nil
			}
		}
	}

	// Search structs
	for _, st := range analysis.Structs {
		if st.QualifiedName == fragment {
			return extractSourceRange(content, st.StartPos, st.EndPos), nil
		}
		for _, m := range st.Methods {
			methodQN := st.QualifiedName + "::" + m.Name
			if methodQN == fragment {
				return extractSourceRange(content, m.StartPos, m.EndPos), nil
			}
		}
	}

	// Search functions
	for _, fn := range analysis.Functions {
		if fn.QualifiedName == fragment {
			return extractSourceRange(content, fn.StartPos, fn.EndPos), nil
		}
	}

	// Search enums
	for _, en := range analysis.Enums {
		if en.QualifiedName == fragment {
			return extractSourceRange(content, en.StartPos, en.EndPos), nil
		}
	}

	// Search type aliases
	for _, ta := range analysis.TypeAliases {
		if ta.QualifiedName == fragment {
			return extractSourceRange(content, ta.StartPos, ta.EndPos), nil
		}
	}

	// Search concepts
	for _, c := range analysis.Concepts {
		if c.QualifiedName == fragment {
			return extractSourceRange(content, c.StartPos, c.EndPos), nil
		}
	}

	return "", fmt.Errorf("entity %q not found in source", fragment)
}

// findEntity locates an entity in the analysis by qualified name and builds
// the source.Entity and method ID list.
func (s *Source) findEntity(analysis *fileAnalysis, fragment, filePath string, content []byte) (*source.Entity, []string) {
	// Check classes
	for _, cls := range analysis.Classes {
		if cls.QualifiedName == fragment {
			return s.buildClassEntity(&cls, filePath, content)
		}
	}

	// Check structs
	for _, st := range analysis.Structs {
		if st.QualifiedName == fragment {
			return s.buildStructEntity(&st, filePath, content)
		}
	}

	// Check functions
	for _, fn := range analysis.Functions {
		if fn.QualifiedName == fragment {
			return s.buildFunctionEntity(&fn, filePath, content), nil
		}
	}

	// Check enums
	for _, en := range analysis.Enums {
		if en.QualifiedName == fragment {
			return s.buildEnumEntity(&en, filePath, content), nil
		}
	}

	// Check type aliases
	for _, ta := range analysis.TypeAliases {
		if ta.QualifiedName == fragment {
			return s.buildTypeAliasEntity(&ta, filePath, content), nil
		}
	}

	// Check concepts
	for _, c := range analysis.Concepts {
		if c.QualifiedName == fragment {
			return s.buildConceptEntity(&c, filePath, content), nil
		}
	}

	// Check namespaces
	for _, ns := range analysis.Namespaces {
		if ns.QualifiedName == fragment {
			return s.buildNamespaceEntity(&ns, filePath, content), nil
		}
	}

	return nil, nil
}

func (s *Source) buildClassEntity(cls *classDef, filePath string, content []byte) (*source.Entity, []string) {
	doc := s.doxygenP.Parse(cls.DocComment)

	properties := make([]source.Property, 0, len(cls.Fields))
	for _, f := range cls.Fields {
		properties = append(properties, source.Property{
			Name:        f.Name,
			Type:        f.Type,
			Description: f.DocComment,
			Visibility:  f.Visibility,
		})
	}

	var methodIDs []string
	for _, m := range cls.Methods {
		methodQN := cls.QualifiedName + "::" + m.Name
		methodIDs = append(methodIDs, filePath+"#"+methodQN)
	}

	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, cls.StartPos, content)

	return &source.Entity{
		Slug:        slugFromQualifiedName(cls.QualifiedName),
		Name:        cls.QualifiedName,
		Kind:        source.KindClass,
		Description: docDescription(doc),
		SourceFile:  rel,
		SourceCode:  extractSourceRange(content, cls.StartPos, cls.EndPos),
		URL:         url,
		Visibility:  cls.Visibility,
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}, methodIDs
}

func (s *Source) buildStructEntity(st *structDef, filePath string, content []byte) (*source.Entity, []string) {
	doc := s.doxygenP.Parse(st.DocComment)

	properties := make([]source.Property, 0, len(st.Fields))
	for _, f := range st.Fields {
		properties = append(properties, source.Property{
			Name:        f.Name,
			Type:        f.Type,
			Description: f.DocComment,
			Visibility:  f.Visibility,
		})
	}

	var methodIDs []string
	for _, m := range st.Methods {
		methodQN := st.QualifiedName + "::" + m.Name
		methodIDs = append(methodIDs, filePath+"#"+methodQN)
	}

	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, st.StartPos, content)

	return &source.Entity{
		Slug:        slugFromQualifiedName(st.QualifiedName),
		Name:        st.QualifiedName,
		Kind:        source.KindStruct,
		Description: docDescription(doc),
		SourceFile:  rel,
		SourceCode:  extractSourceRange(content, st.StartPos, st.EndPos),
		URL:         url,
		Visibility:  "public", // structs default public
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}, methodIDs
}

func (s *Source) buildFunctionEntity(fn *functionDef, filePath string, content []byte) *source.Entity {
	doc := s.doxygenP.Parse(fn.DocComment)

	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, fn.StartPos, content)

	return &source.Entity{
		Slug:        slugFromQualifiedName(fn.QualifiedName),
		Name:        fn.QualifiedName,
		Kind:        source.KindFunction,
		Description: docDescription(doc),
		SourceFile:  rel,
		SourceCode:  extractSourceRange(content, fn.StartPos, fn.EndPos),
		URL:         url,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}
}

func (s *Source) buildEnumEntity(en *enumDef, filePath string, content []byte) *source.Entity {
	doc := s.doxygenP.Parse(en.DocComment)

	properties := make([]source.Property, 0, len(en.Values))
	for _, v := range en.Values {
		properties = append(properties, source.Property{
			Name:        v.Name,
			Type:        v.Value,
			Description: v.DocComment,
		})
	}

	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, en.StartPos, content)

	return &source.Entity{
		Slug:        slugFromQualifiedName(en.QualifiedName),
		Name:        en.QualifiedName,
		Kind:        source.KindEnum,
		Description: docDescription(doc),
		SourceFile:  rel,
		SourceCode:  extractSourceRange(content, en.StartPos, en.EndPos),
		URL:         url,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}
}

func (s *Source) buildTypeAliasEntity(ta *typeAliasDef, filePath string, content []byte) *source.Entity {
	doc := s.doxygenP.Parse(ta.DocComment)

	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, ta.StartPos, content)

	return &source.Entity{
		Slug:        slugFromQualifiedName(ta.QualifiedName),
		Name:        ta.QualifiedName,
		Kind:        source.KindTypeAlias,
		Description: docDescription(doc),
		SourceFile:  rel,
		SourceCode:  extractSourceRange(content, ta.StartPos, ta.EndPos),
		URL:         url,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}
}

func (s *Source) buildConceptEntity(c *conceptDef, filePath string, content []byte) *source.Entity {
	doc := s.doxygenP.Parse(c.DocComment)

	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, c.StartPos, content)

	return &source.Entity{
		Slug:        slugFromQualifiedName(c.QualifiedName),
		Name:        c.QualifiedName,
		Kind:        source.KindConcept,
		Description: docDescription(doc),
		SourceFile:  rel,
		SourceCode:  extractSourceRange(content, c.StartPos, c.EndPos),
		URL:         url,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}
}

func (s *Source) buildNamespaceEntity(ns *namespaceDef, filePath string, content []byte) *source.Entity {
	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, ns.StartPos, content)

	return &source.Entity{
		Slug:        slugFromQualifiedName(ns.QualifiedName),
		Name:        ns.QualifiedName,
		Kind:        source.KindNamespace,
		Description: "",
		SourceFile:  rel,
		SourceCode:  extractSourceRange(content, ns.StartPos, ns.EndPos),
		URL:         url,
		Visibility:  "public",
	}
}

// findMethod locates a method in the analysis by qualified name.
func (s *Source) findMethod(analysis *fileAnalysis, fragment, filePath string, content []byte) *source.Method {
	// Try as a class/struct method: "Namespace::Class::method"
	for _, cls := range analysis.Classes {
		for _, m := range cls.Methods {
			methodQN := cls.QualifiedName + "::" + m.Name
			if methodQN == fragment {
				return s.buildMethod(&m, &cls, filePath, content)
			}
		}
	}
	for _, st := range analysis.Structs {
		for _, m := range st.Methods {
			methodQN := st.QualifiedName + "::" + m.Name
			if methodQN == fragment {
				return s.buildMethodFromStruct(&m, &st, filePath, content)
			}
		}
	}

	// Try as a free function
	for _, fn := range analysis.Functions {
		if fn.QualifiedName == fragment {
			return s.buildFreeFunction(&fn, filePath, content)
		}
	}

	return nil
}

func (s *Source) buildMethod(m *methodDef, cls *classDef, filePath string, content []byte) *source.Method {
	doc := s.doxygenP.Parse(m.DocComment)

	params := buildParams(doc, m)
	relations := buildRelations(doc)

	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, m.StartPos, content)

	return &source.Method{
		Slug:        strings.ToLower(m.Name),
		Name:        m.Name,
		Signature:   m.Signature,
		Description: docDescription(doc),
		Parameters:  params,
		ReturnType:  m.ReturnType,
		ReturnDesc:  returnDesc(doc),
		SourceCode:  extractSourceRange(content, m.StartPos, m.EndPos),
		URL:         url,
		Since:       doc.Since,
		Deprecated:  doc.Deprecated != "",
		Relations:   relations,
	}
}

func (s *Source) buildMethodFromStruct(m *methodDef, st *structDef, filePath string, content []byte) *source.Method {
	doc := s.doxygenP.Parse(m.DocComment)

	params := buildParams(doc, m)
	relations := buildRelations(doc)

	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, m.StartPos, content)

	return &source.Method{
		Slug:        strings.ToLower(m.Name),
		Name:        m.Name,
		Signature:   m.Signature,
		Description: docDescription(doc),
		Parameters:  params,
		ReturnType:  m.ReturnType,
		ReturnDesc:  returnDesc(doc),
		SourceCode:  extractSourceRange(content, m.StartPos, m.EndPos),
		URL:         url,
		Since:       doc.Since,
		Deprecated:  doc.Deprecated != "",
		Relations:   relations,
	}
}

func (s *Source) buildFreeFunction(fn *functionDef, filePath string, content []byte) *source.Method {
	doc := s.doxygenP.Parse(fn.DocComment)

	params := make([]source.Parameter, 0, len(doc.Params))
	for _, p := range doc.Params {
		params = append(params, source.Parameter{
			Name:        p.Name,
			Type:        p.Type,
			Required:    !p.Optional,
			Description: p.Description,
		})
	}

	relations := buildRelations(doc)

	rel := relativePath(s.cfg.RepoPath, filePath)
	url := s.buildSourceURL(rel, fn.StartPos, content)

	return &source.Method{
		Slug:        strings.ToLower(fn.Name),
		Name:        fn.Name,
		Signature:   fn.Signature,
		Description: docDescription(doc),
		Parameters:  params,
		ReturnType:  fn.ReturnType,
		ReturnDesc:  returnDesc(doc),
		SourceCode:  extractSourceRange(content, fn.StartPos, fn.EndPos),
		URL:         url,
		Since:       doc.Since,
		Deprecated:  doc.Deprecated != "",
		Relations:   relations,
	}
}

// buildSourceURL constructs a GitHub source URL with line number.
func (s *Source) buildSourceURL(rel string, startPos int, content []byte) string {
	base := strings.TrimRight(s.cfg.SourceURL, "/") + "/blob/" + s.cfg.Ref + "/"
	url := base + rel
	if ln := lineNumber(content, startPos); ln > 0 {
		url += fmt.Sprintf("#L%d", ln)
	}
	return url
}

// buildParams converts docparser params and method definition into source.Parameter.
func buildParams(doc *docparser.DocComment, m *methodDef) []source.Parameter {
	params := make([]source.Parameter, 0, len(doc.Params))
	for _, p := range doc.Params {
		params = append(params, source.Parameter{
			Name:        p.Name,
			Type:        p.Type,
			Required:    !p.Optional,
			Description: p.Description,
		})
	}
	return params
}

// buildRelations converts docparser see references into source.Relation.
func buildRelations(doc *docparser.DocComment) []source.Relation {
	var relations []source.Relation
	for _, see := range doc.See {
		relations = append(relations, source.Relation{
			Kind:       "uses",
			TargetName: see,
		})
	}
	return relations
}

// returnDesc extracts return description from doc comment.
func returnDesc(doc *docparser.DocComment) string {
	if doc.Returns != nil {
		return doc.Returns.Description
	}
	return ""
}

// docDescription returns the best available description from a doc comment.
// Uses Description if available, falls back to Summary (from \brief).
func docDescription(doc *docparser.DocComment) string {
	if doc.Description != "" {
		return doc.Description
	}
	return doc.Summary
}

// splitFragment splits "path#fragment" into ("path", "fragment").
func splitFragment(id string) (string, string) {
	if idx := strings.LastIndex(id, "#"); idx >= 0 {
		return id[:idx], id[idx+1:]
	}
	return id, ""
}

// extractSourceRange safely extracts source bytes.
func extractSourceRange(content []byte, start, end int) string {
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

// lineNumber returns the 1-based line number for byte offset pos.
func lineNumber(content []byte, pos int) int {
	if pos <= 0 || pos > len(content) {
		return 0
	}
	line := 1
	for i := 0; i < pos; i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}

// relativePath returns the path of file relative to root.
func relativePath(root, file string) string {
	// Normalize separators
	root = strings.ReplaceAll(root, "\\", "/")
	file = strings.ReplaceAll(file, "\\", "/")
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}
	if strings.HasPrefix(file, root) {
		return file[len(root):]
	}
	return file
}

// slugFromQualifiedName converts "std::vector" to "std-vector".
func slugFromQualifiedName(qn string) string {
	slug := strings.ReplaceAll(qn, "::", "-")
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "<", "-")
	slug = strings.ReplaceAll(slug, ">", "")
	slug = strings.ReplaceAll(slug, ",", "")
	return strings.ToLower(slug)
}

