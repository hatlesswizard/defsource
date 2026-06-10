// Package golang provides a Source interface implementation for parsing Go
// source code. It supports any Go package (stdlib, Gin, Echo, GORM, etc.)
// by walking Go source directories, extracting exported types, functions,
// and methods via tree-sitter, and detecting wrapper delegation patterns.
package golang

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hatlesswizard/defsource/internal/docparser/godoc"
	"github.com/hatlesswizard/defsource/internal/source"
)

// Config defines the settings for a Go source adapter instance.
type Config struct {
	// LibraryID is the canonical library identifier (e.g., "go/stdlib", "go/gin").
	LibraryID string

	// Name is the human-readable library name.
	Name string

	// Description is a short description of the library.
	Description string

	// SourceURL is the upstream repository URL.
	SourceURL string

	// Ref is the git tag/branch for source-code link generation.
	Ref string

	// RootDirs are the directories to walk for discovery relative to repoPath.
	// Empty means walk the entire repoPath.
	RootDirs []string

	// ExcludeDirs are directory names to skip during discovery.
	ExcludeDirs []string

	// GitHubOwnerRepo is "owner/repo" for generating GitHub blob URLs.
	// If empty, source URLs will be file-relative.
	GitHubOwnerRepo string
}

// Source implements the source.Source interface for Go packages.
type Source struct {
	repoPath string
	config   Config
	index    *codebaseIndex
	docP     *godoc.Parser
}

var _ source.Source = (*Source)(nil)

// New constructs a Go Source pointing at a local clone of a Go package.
func New(repoPath string, cfg Config) *Source {
	return &Source{
		repoPath: repoPath,
		config:   cfg,
		index:    emptyIndex(),
		docP:     godoc.New(),
	}
}

// ID returns the canonical library ID.
func (s *Source) ID() string { return s.config.LibraryID }

// Meta returns metadata for the library record.
func (s *Source) Meta() source.LibraryMeta {
	return source.LibraryMeta{
		Name:        s.config.Name,
		Description: s.config.Description,
		SourceURL:   s.config.SourceURL,
		Version:     s.config.Ref,
		Language:    "go",
		TrustScore:  0.95,
	}
}

// DiscoverEntities walks the local repository and returns entity identifiers
// for all exported Go types, functions, and constants.
func (s *Source) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	idx, err := buildCodebaseIndex(s.repoPath, s.config)
	if err != nil {
		return nil, fmt.Errorf("buildCodebaseIndex: %w", err)
	}
	s.index = idx
	ids := idx.buildEntityList()
	log.Printf("Discovered %d Go entities from %s", len(ids), s.repoPath)
	return ids, nil
}

// ParseEntity parses a single Go entity from file content.
func (s *Source) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	filePath, entityName := splitFragment(entityID)
	if entityName == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	parsed := parseFile(content)
	if parsed == nil {
		return nil, nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Try structs
	for i := range parsed.Structs {
		if parsed.Structs[i].Name == entityName {
			return s.buildStructEntity(filePath, content, &parsed.Structs[i], parsed)
		}
	}

	// Try interfaces
	for i := range parsed.Interfaces {
		if parsed.Interfaces[i].Name == entityName {
			return s.buildInterfaceEntity(filePath, content, &parsed.Interfaces[i])
		}
	}

	// Try functions
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == entityName {
			return s.buildFunctionEntity(filePath, content, &parsed.Functions[i])
		}
	}

	// Try type aliases
	for i := range parsed.TypeAliases {
		if parsed.TypeAliases[i].Name == entityName {
			return s.buildTypeAliasEntity(filePath, content, &parsed.TypeAliases[i])
		}
	}

	// Try constants
	for i := range parsed.Constants {
		if parsed.Constants[i].Name == entityName {
			return s.buildConstantEntity(filePath, content, &parsed.Constants[i])
		}
	}

	return nil, nil, fmt.Errorf("entity %q not found in %s", entityName, filePath)
}

// ParseMethod parses a method on a type from file content.
func (s *Source) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	filePath, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	typeName, methodName, ok := strings.Cut(fragment, ".")
	if !ok {
		return nil, fmt.Errorf("invalid method fragment %q: expected TypeName.MethodName", fragment)
	}

	parsed := parseFile(content)
	if parsed == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}

	for i := range parsed.Methods {
		m := &parsed.Methods[i]
		if m.ReceiverType == typeName && m.Name == methodName {
			return s.buildMethod(filePath, content, m)
		}
	}

	return nil, fmt.Errorf("method %s.%s not found in %s", typeName, methodName, filePath)
}

// DetectWrapper analyzes a method's source code for wrapper patterns.
func (s *Source) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil {
		return false, "", ""
	}
	return detectWrapper([]byte(method.SourceCode), s.index)
}

// ResolveWrapperURL constructs a URL for a wrapped target.
func (s *Source) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "function":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
	case "method":
		// targetName is "TypeName.MethodName"
		parts := strings.SplitN(targetName, ".", 2)
		if len(parts) != 2 {
			return ""
		}
		if path, ok := s.index.FileForType(parts[0]); ok {
			return path + "#" + targetName
		}
	}
	return ""
}

// ParseSourceCode extracts source code for a specific entity from file content.
func (s *Source) ParseSourceCode(entityID string, content []byte) (string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	parsed := parseFile(content)
	if parsed == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	// Check if it's a method reference (TypeName.MethodName)
	if typeName, methodName, ok := strings.Cut(fragment, "."); ok && isExported(typeName) && isExported(methodName) {
		for _, m := range parsed.Methods {
			if m.ReceiverType == typeName && m.Name == methodName {
				return extractSource(content, m.StartPos, m.EndPos), nil
			}
		}
		return "", fmt.Errorf("method %s not found", fragment)
	}

	// Look for types
	for _, st := range parsed.Structs {
		if st.Name == fragment {
			return extractSource(content, st.StartPos, st.EndPos), nil
		}
	}
	for _, iface := range parsed.Interfaces {
		if iface.Name == fragment {
			return extractSource(content, iface.StartPos, iface.EndPos), nil
		}
	}
	for _, fn := range parsed.Functions {
		if fn.Name == fragment {
			return extractSource(content, fn.StartPos, fn.EndPos), nil
		}
	}
	for _, ta := range parsed.TypeAliases {
		if ta.Name == fragment {
			return extractSource(content, ta.StartPos, ta.EndPos), nil
		}
	}
	for _, c := range parsed.Constants {
		if c.Name == fragment {
			return extractSource(content, c.StartPos, c.EndPos), nil
		}
	}

	return "", fmt.Errorf("entity %q not found", fragment)
}

func (s *Source) buildStructEntity(filePath string, content []byte, st *structDef, parsed *fileAnalysis) (*source.Entity, []string, error) {
	doc := s.docP.Parse(st.DocComment)
	rel := s.relativePath(filePath)
	url := s.buildURL(rel, content, st.StartPos)

	pkg := parsed.Package
	slug := buildSlug(pkg, st.Name)

	properties := make([]source.Property, 0, len(st.Fields))
	for _, f := range st.Fields {
		if !isExported(f.Name) && f.Name != "" {
			continue
		}
		properties = append(properties, source.Property{
			Name:        f.Name,
			Type:        f.Type,
			Description: f.Tag,
			Visibility:  "public",
		})
	}

	entity := &source.Entity{
		Slug:        slug,
		Name:        st.Name,
		Kind:        source.KindStruct,
		Description: doc.Summary,
		SourceFile:  rel,
		SourceCode:  extractSource(content, st.StartPos, st.EndPos),
		URL:         url,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}

	// Collect methods for this struct
	methodIDs := s.collectMethodIDs(filePath, st.Name, parsed)

	return entity, methodIDs, nil
}

func (s *Source) buildInterfaceEntity(filePath string, content []byte, iface *interfaceDef) (*source.Entity, []string, error) {
	parsed := parseFile(content)
	doc := s.docP.Parse(iface.DocComment)
	rel := s.relativePath(filePath)
	url := s.buildURL(rel, content, iface.StartPos)

	pkg := ""
	if parsed != nil {
		pkg = parsed.Package
	}
	slug := buildSlug(pkg, iface.Name)

	properties := make([]source.Property, 0, len(iface.Methods))
	for _, m := range iface.Methods {
		properties = append(properties, source.Property{
			Name:       m.Name,
			Type:       m.Signature,
			Visibility: "public",
		})
	}

	entity := &source.Entity{
		Slug:        slug,
		Name:        iface.Name,
		Kind:        source.KindInterface,
		Description: doc.Summary,
		SourceFile:  rel,
		SourceCode:  extractSource(content, iface.StartPos, iface.EndPos),
		URL:         url,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}

	return entity, nil, nil
}

func (s *Source) buildFunctionEntity(filePath string, content []byte, fn *functionDef) (*source.Entity, []string, error) {
	parsed := parseFile(content)
	doc := s.docP.Parse(fn.DocComment)
	rel := s.relativePath(filePath)
	url := s.buildURL(rel, content, fn.StartPos)

	pkg := ""
	if parsed != nil {
		pkg = parsed.Package
	}
	slug := buildSlug(pkg, fn.Name)

	entity := &source.Entity{
		Slug:        slug,
		Name:        fn.Name,
		Kind:        source.KindFunction,
		Description: doc.Summary,
		SourceFile:  rel,
		SourceCode:  extractSource(content, fn.StartPos, fn.EndPos),
		URL:         url,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

func (s *Source) buildTypeAliasEntity(filePath string, content []byte, ta *typeAliasDef) (*source.Entity, []string, error) {
	parsed := parseFile(content)
	doc := s.docP.Parse(ta.DocComment)
	rel := s.relativePath(filePath)
	url := s.buildURL(rel, content, ta.StartPos)

	pkg := ""
	if parsed != nil {
		pkg = parsed.Package
	}
	slug := buildSlug(pkg, ta.Name)

	entity := &source.Entity{
		Slug:        slug,
		Name:        ta.Name,
		Kind:        source.KindTypeAlias,
		Description: doc.Summary,
		SourceFile:  rel,
		SourceCode:  extractSource(content, ta.StartPos, ta.EndPos),
		URL:         url,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

func (s *Source) buildConstantEntity(filePath string, content []byte, c *constantDef) (*source.Entity, []string, error) {
	parsed := parseFile(content)
	doc := s.docP.Parse(c.DocComment)
	rel := s.relativePath(filePath)
	url := s.buildURL(rel, content, c.StartPos)

	pkg := ""
	if parsed != nil {
		pkg = parsed.Package
	}
	slug := buildSlug(pkg, c.Name)

	entity := &source.Entity{
		Slug:        slug,
		Name:        c.Name,
		Kind:        source.KindConstant,
		Description: doc.Summary,
		SourceFile:  rel,
		SourceCode:  extractSource(content, c.StartPos, c.EndPos),
		URL:         url,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

func (s *Source) buildMethod(filePath string, content []byte, m *methodDef) (*source.Method, error) {
	doc := s.docP.Parse(m.DocComment)
	rel := s.relativePath(filePath)
	url := s.buildURL(rel, content, m.StartPos)

	params := make([]source.Parameter, 0, len(m.Params))
	for _, p := range m.Params {
		params = append(params, source.Parameter{
			Name:     p.Name,
			Type:     p.Type,
			Required: !p.Variadic,
		})
	}

	sig := buildMethodSignature(m)

	return &source.Method{
		Slug:        strings.ToLower(m.Name),
		Name:        m.Name,
		Signature:   sig,
		Description: doc.Summary,
		Parameters:  params,
		ReturnType:  m.ReturnType,
		SourceCode:  extractSource(content, m.StartPos, m.EndPos),
		URL:         url,
		Since:       doc.Since,
		Deprecated:  doc.Deprecated != "",
	}, nil
}

func (s *Source) collectMethodIDs(filePath string, typeName string, parsed *fileAnalysis) []string {
	var ids []string
	for _, m := range parsed.Methods {
		if m.ReceiverType == typeName && isExported(m.Name) {
			ids = append(ids, filePath+"#"+typeName+"."+m.Name)
		}
	}
	return ids
}

func (s *Source) relativePath(absPath string) string {
	if s.repoPath == "" {
		return absPath
	}
	rel, err := filepath.Rel(s.repoPath, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}

func (s *Source) buildURL(rel string, content []byte, pos int) string {
	if s.config.GitHubOwnerRepo == "" {
		return rel
	}
	ref := s.config.Ref
	if ref == "" {
		ref = "main"
	}
	url := fmt.Sprintf("https://github.com/%s/blob/%s/%s", s.config.GitHubOwnerRepo, ref, rel)
	if ln := lineNumber(content, pos); ln > 0 {
		url += fmt.Sprintf("#L%d", ln)
	}
	return url
}

// splitFragment returns the path before "#" and the fragment after "#".
func splitFragment(id string) (base, fragment string) {
	base, fragment, _ = strings.Cut(id, "#")
	return base, fragment
}

// isExported reports whether a Go identifier is exported (starts with uppercase).
func isExported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

// buildSlug constructs a slug from package name and entity name.
func buildSlug(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "/" + name
}

// extractSource safely extracts source code from content given byte offsets.
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

// lineNumber returns the 1-indexed line number for the given byte offset.
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

// buildMethodSignature constructs a Go method signature string.
func buildMethodSignature(m *methodDef) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if m.ReceiverType != "" {
		sb.WriteString("(")
		if m.ReceiverName != "" {
			sb.WriteString(m.ReceiverName)
			sb.WriteString(" ")
		}
		if m.PointerReceiver {
			sb.WriteString("*")
		}
		sb.WriteString(m.ReceiverType)
		sb.WriteString(") ")
	}
	sb.WriteString(m.Name)
	sb.WriteString("(")
	for i, p := range m.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(p.Name)
		sb.WriteString(" ")
		if p.Variadic {
			sb.WriteString("...")
		}
		sb.WriteString(p.Type)
	}
	sb.WriteString(")")
	if m.ReturnType != "" {
		sb.WriteString(" ")
		sb.WriteString(m.ReturnType)
	}
	return sb.String()
}
