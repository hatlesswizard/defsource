// Package rust provides a documentation source adapter that reads Rust source
// code from a local checkout of any Rust crate repository. It parses raw Rust
// source via tree-sitter to extract structs, enums, traits, functions, type
// aliases, macros, and constants without making any network calls during parsing.
package rust

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser/rustdoc"
	"github.com/hatlesswizard/defsource/internal/source"
)

// Config holds configuration for a Rust crate source adapter.
type Config struct {
	// Owner is the GitHub owner (e.g., "tokio-rs").
	Owner string
	// Repo is the GitHub repository name (e.g., "tokio").
	Repo string
	// CrateName is the crate name for display (e.g., "tokio").
	CrateName string
	// CratePath is the subdirectory within the repo containing the crate source.
	// Empty means the repo root (e.g., "tokio" for tokio-rs/tokio monorepo).
	CratePath string
	// Version is the git tag/branch used for source-code links.
	Version string
	// Description is a human-readable crate description.
	Description string
}

// RustSource is a documentation source adapter that reads Rust source code
// from a local checkout and parses it via tree-sitter.
type RustSource struct {
	cfg      Config
	repoPath string
	index    *codebaseIndex
}

var _ source.Source = (*RustSource)(nil)

// New constructs a new RustSource pointing at a local checkout. The index is
// initialised empty so that DetectWrapper and ResolveWrapperURL are safe to
// call before DiscoverEntities.
func New(repoPath string, cfg Config) *RustSource {
	return &RustSource{
		cfg:      cfg,
		repoPath: repoPath,
		index:    emptyIndex(),
	}
}

// ID returns the canonical library ID (e.g., "rust/tokio").
func (s *RustSource) ID() string {
	return "rust/" + s.cfg.CrateName
}

// Meta returns metadata for the library record.
func (s *RustSource) Meta() source.LibraryMeta {
	desc := s.cfg.Description
	if desc == "" {
		desc = fmt.Sprintf("Rust crate: %s", s.cfg.CrateName)
	}
	return source.LibraryMeta{
		Name:        s.cfg.CrateName,
		Description: desc,
		SourceURL:   fmt.Sprintf("https://github.com/%s/%s", s.cfg.Owner, s.cfg.Repo),
		Version:     s.cfg.Version,
		Language:    "rust",
		TrustScore:  0.90,
	}
}

// DiscoverEntities walks the source tree and returns entity identifiers.
func (s *RustSource) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	root := s.srcRoot()
	idx, err := buildCodebaseIndex(root)
	if err != nil {
		return nil, fmt.Errorf("buildCodebaseIndex: %w", err)
	}
	s.index = idx
	ids := idx.buildEntityList()
	log.Printf("Discovered %d Rust entities from %s", len(ids), root)
	return ids, nil
}

// ParseEntity parses a single entity from file content.
func (s *RustSource) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, nil, fmt.Errorf("failed to parse source for %s", entityID)
	}

	filePath := stripFragment(entityID)
	rel := s.relativePath(filePath)
	blobBase := s.blobBase()

	// Look for the entity by name
	for i := range analysis.Structs {
		st := &analysis.Structs[i]
		if st.Name == fragment {
			return s.buildStructEntity(st, content, rel, blobBase, filePath)
		}
	}
	for i := range analysis.Enums {
		en := &analysis.Enums[i]
		if en.Name == fragment {
			return s.buildEnumEntity(en, content, rel, blobBase)
		}
	}
	for i := range analysis.Traits {
		tr := &analysis.Traits[i]
		if tr.Name == fragment {
			return s.buildTraitEntity(tr, content, rel, blobBase, filePath)
		}
	}
	for i := range analysis.Functions {
		fn := &analysis.Functions[i]
		if fn.Name == fragment {
			return s.buildFunctionEntity(fn, content, rel, blobBase)
		}
	}
	for i := range analysis.TypeAliases {
		ta := &analysis.TypeAliases[i]
		if ta.Name == fragment {
			return s.buildTypeAliasEntity(ta, content, rel, blobBase)
		}
	}
	for i := range analysis.Macros {
		m := &analysis.Macros[i]
		if m.Name == fragment {
			return s.buildMacroEntity(m, content, rel, blobBase)
		}
	}
	for i := range analysis.Constants {
		c := &analysis.Constants[i]
		if c.Name == fragment {
			return s.buildConstantEntity(c, content, rel, blobBase)
		}
	}

	return nil, nil, fmt.Errorf("entity %q not found in %s", fragment, filePath)
}

// ParseMethod parses a method identified by entityID#TypeName::method_name.
func (s *RustSource) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	_, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	typeName, methodName, ok := strings.Cut(fragment, "::")
	if !ok {
		return nil, fmt.Errorf("invalid method fragment %q: expected TypeName::method_name", fragment)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, fmt.Errorf("failed to parse source for %s", methodID)
	}

	// Search impl blocks for this type
	for i := range analysis.ImplBlocks {
		impl := &analysis.ImplBlocks[i]
		if impl.TypeName != typeName {
			continue
		}
		for j := range impl.Methods {
			m := &impl.Methods[j]
			if m.Name == methodName {
				return s.buildMethod(typeName, m, content, methodID)
			}
		}
	}

	// Search trait definitions for trait methods
	for i := range analysis.Traits {
		tr := &analysis.Traits[i]
		if tr.Name != typeName {
			continue
		}
		for j := range tr.Methods {
			m := &tr.Methods[j]
			if m.Name == methodName {
				return s.buildMethod(typeName, m, content, methodID)
			}
		}
	}

	return nil, fmt.Errorf("method %q not found for type %q", methodName, typeName)
}

// DetectWrapper analyzes a method's source code for wrapper patterns.
func (s *RustSource) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil || method.SourceCode == "" {
		return false, "", ""
	}
	return detectWrapper(method.SourceCode)
}

// ResolveWrapperURL constructs the identifier to fetch the wrapped target's source.
func (s *RustSource) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "function":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
	case "method":
		// targetName is "TypeName::method_name"
		parts := strings.SplitN(targetName, "::", 2)
		if len(parts) != 2 {
			return ""
		}
		typeName := parts[0]
		if path, ok := s.index.FileForType(typeName); ok {
			return path + "#" + targetName
		}
	case "self_method":
		// Same type, different method
		if path, ok := s.index.FileForType(entitySlug); ok {
			return path + "#" + entitySlug + "::" + targetName
		}
	}
	return ""
}

// ParseSourceCode extracts the source code for a specific entity or method.
func (s *RustSource) ParseSourceCode(entityID string, content []byte) (string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	analysis := parseFile(content)
	if analysis == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	// Check for TypeName::method format
	if typeName, methodName, ok := strings.Cut(fragment, "::"); ok {
		for i := range analysis.ImplBlocks {
			impl := &analysis.ImplBlocks[i]
			if impl.TypeName != typeName {
				continue
			}
			for j := range impl.Methods {
				m := &impl.Methods[j]
				if m.Name == methodName {
					return safeSlice(content, m.StartPos, m.EndPos), nil
				}
			}
		}
		for i := range analysis.Traits {
			tr := &analysis.Traits[i]
			if tr.Name != typeName {
				continue
			}
			for j := range tr.Methods {
				m := &tr.Methods[j]
				if m.Name == methodName {
					return safeSlice(content, m.StartPos, m.EndPos), nil
				}
			}
		}
		return "", fmt.Errorf("method %q not found for type %q", methodName, typeName)
	}

	// Look for top-level entity
	for _, st := range analysis.Structs {
		if st.Name == fragment {
			return safeSlice(content, st.StartPos, st.EndPos), nil
		}
	}
	for _, en := range analysis.Enums {
		if en.Name == fragment {
			return safeSlice(content, en.StartPos, en.EndPos), nil
		}
	}
	for _, tr := range analysis.Traits {
		if tr.Name == fragment {
			return safeSlice(content, tr.StartPos, tr.EndPos), nil
		}
	}
	for _, fn := range analysis.Functions {
		if fn.Name == fragment {
			return safeSlice(content, fn.StartPos, fn.EndPos), nil
		}
	}
	for _, ta := range analysis.TypeAliases {
		if ta.Name == fragment {
			return safeSlice(content, ta.StartPos, ta.EndPos), nil
		}
	}
	for _, m := range analysis.Macros {
		if m.Name == fragment {
			return safeSlice(content, m.StartPos, m.EndPos), nil
		}
	}
	for _, c := range analysis.Constants {
		if c.Name == fragment {
			return safeSlice(content, c.StartPos, c.EndPos), nil
		}
	}

	return "", fmt.Errorf("entity %q not found", fragment)
}

// srcRoot returns the filesystem path to the crate src/ directory.
// When the conventional <crate>/src layout does not exist (flattened
// crates, or configs that point directly at a source directory), it
// falls back to the crate path itself so discovery still finds code.
func (s *RustSource) srcRoot() string {
	base := s.repoPath
	if s.cfg.CratePath != "" {
		base = filepath.Join(s.repoPath, s.cfg.CratePath)
	}
	withSrc := filepath.Join(base, "src")
	if fi, err := os.Stat(withSrc); err == nil && fi.IsDir() {
		return withSrc
	}
	return base
}

// blobBase returns the GitHub blob URL prefix for source-code links.
func (s *RustSource) blobBase() string {
	ref := s.cfg.Version
	if ref == "" {
		ref = "main"
	}
	return fmt.Sprintf("https://github.com/%s/%s/blob/%s/", s.cfg.Owner, s.cfg.Repo, ref)
}

// relativePath returns absPath relative to repoPath.
func (s *RustSource) relativePath(absPath string) string {
	if s.repoPath == "" {
		return absPath
	}
	rel, err := filepath.Rel(s.repoPath, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}

// buildStructEntity creates an Entity from a parsed struct.
func (s *RustSource) buildStructEntity(st *structDef, content []byte, rel, blobBase, filePath string) (*source.Entity, []string, error) {
	doc := parseDocComment(st.DocComment)

	githubURL := blobBase + rel
	if ln := lineNumber(content, st.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	properties := make([]source.Property, 0, len(st.Fields))
	for _, f := range st.Fields {
		fdoc := parseDocComment(f.DocComment)
		properties = append(properties, source.Property{
			Name:        f.Name,
			Type:        f.Type,
			Description: fdoc.Summary,
			Visibility:  f.Visibility,
		})
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(st.Name),
		Name:        st.Name,
		Kind:        source.KindStruct,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  safeSlice(content, st.StartPos, st.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}

	// Collect method IDs from associated impl blocks
	var methodIDs []string
	for i := range s.index.implBlocks {
		impl := &s.index.implBlocks[i]
		if impl.TypeName == st.Name && impl.TraitName == "" {
			for _, m := range impl.Methods {
				if m.Visibility == "public" {
					methodIDs = append(methodIDs, filePath+"#"+st.Name+"::"+m.Name)
				}
			}
		}
	}

	return entity, methodIDs, nil
}

// buildEnumEntity creates an Entity from a parsed enum.
func (s *RustSource) buildEnumEntity(en *enumDef, content []byte, rel, blobBase string) (*source.Entity, []string, error) {
	doc := parseDocComment(en.DocComment)

	githubURL := blobBase + rel
	if ln := lineNumber(content, en.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	// Enum variants as properties
	properties := make([]source.Property, 0, len(en.Variants))
	for _, v := range en.Variants {
		vdoc := parseDocComment(v.DocComment)
		typ := ""
		if v.Fields != "" {
			typ = v.Fields
		}
		properties = append(properties, source.Property{
			Name:        v.Name,
			Type:        typ,
			Description: vdoc.Summary,
			Visibility:  "public",
		})
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(en.Name),
		Name:        en.Name,
		Kind:        source.KindEnum,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  safeSlice(content, en.StartPos, en.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}

	return entity, nil, nil
}

// buildTraitEntity creates an Entity from a parsed trait.
func (s *RustSource) buildTraitEntity(tr *traitDef, content []byte, rel, blobBase, filePath string) (*source.Entity, []string, error) {
	doc := parseDocComment(tr.DocComment)

	githubURL := blobBase + rel
	if ln := lineNumber(content, tr.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	// Associated types as properties
	properties := make([]source.Property, 0, len(tr.AssocTypes))
	for _, at := range tr.AssocTypes {
		properties = append(properties, source.Property{
			Name:        at.Name,
			Type:        at.Bound,
			Description: "",
			Visibility:  "public",
		})
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(tr.Name),
		Name:        tr.Name,
		Kind:        source.KindTrait,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  safeSlice(content, tr.StartPos, tr.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
		Properties:  properties,
	}

	// Methods as method IDs
	var methodIDs []string
	for _, m := range tr.Methods {
		methodIDs = append(methodIDs, filePath+"#"+tr.Name+"::"+m.Name)
	}

	return entity, methodIDs, nil
}

// buildFunctionEntity creates an Entity from a parsed function.
func (s *RustSource) buildFunctionEntity(fn *functionDef, content []byte, rel, blobBase string) (*source.Entity, []string, error) {
	doc := parseDocComment(fn.DocComment)

	githubURL := blobBase + rel
	if ln := lineNumber(content, fn.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(fn.Name),
		Name:        fn.Name,
		Kind:        source.KindFunction,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  safeSlice(content, fn.StartPos, fn.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

// buildTypeAliasEntity creates an Entity from a parsed type alias.
func (s *RustSource) buildTypeAliasEntity(ta *typeAliasDef, content []byte, rel, blobBase string) (*source.Entity, []string, error) {
	doc := parseDocComment(ta.DocComment)

	githubURL := blobBase + rel
	if ln := lineNumber(content, ta.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(ta.Name),
		Name:        ta.Name,
		Kind:        source.KindTypeAlias,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  safeSlice(content, ta.StartPos, ta.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

// buildMacroEntity creates an Entity from a parsed macro.
func (s *RustSource) buildMacroEntity(m *macroDef, content []byte, rel, blobBase string) (*source.Entity, []string, error) {
	doc := parseDocComment(m.DocComment)

	githubURL := blobBase + rel
	if ln := lineNumber(content, m.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(m.Name),
		Name:        m.Name,
		Kind:        source.KindMacro,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  safeSlice(content, m.StartPos, m.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

// buildConstantEntity creates an Entity from a parsed constant.
func (s *RustSource) buildConstantEntity(c *constantDef, content []byte, rel, blobBase string) (*source.Entity, []string, error) {
	doc := parseDocComment(c.DocComment)

	githubURL := blobBase + rel
	if ln := lineNumber(content, c.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(c.Name),
		Name:        c.Name,
		Kind:        source.KindConstant,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  safeSlice(content, c.StartPos, c.EndPos),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

// buildMethod creates a Method from a parsed method definition.
func (s *RustSource) buildMethod(typeName string, m *methodDef, content []byte, methodID string) (*source.Method, error) {
	doc := parseDocComment(m.DocComment)

	filePath := stripFragment(methodID)
	rel := s.relativePath(filePath)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, m.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	params := make([]source.Parameter, 0, len(m.Params))
	for _, p := range m.Params {
		if p.Name == "self" || p.Name == "&self" || p.Name == "&mut self" || p.Name == "mut self" {
			continue
		}
		desc := ""
		for _, pd := range doc.Params {
			if pd.Name == p.Name {
				desc = pd.Description
				break
			}
		}
		params = append(params, source.Parameter{
			Name:        p.Name,
			Type:        p.Type,
			Required:    !p.HasDefault,
			Description: desc,
		})
	}

	returnType := m.ReturnType
	returnDesc := ""
	if doc.Returns != nil {
		returnDesc = doc.Returns.Description
		if returnType == "" {
			returnType = doc.Returns.Type
		}
	}

	return &source.Method{
		Slug:       strings.ToLower(m.Name),
		Name:       m.Name,
		Signature:  m.Signature,
		Description: doc.Description,
		Parameters: params,
		ReturnType: returnType,
		ReturnDesc: returnDesc,
		SourceCode: safeSlice(content, m.StartPos, m.EndPos),
		URL:        githubURL,
		Since:      doc.Since,
		Deprecated: doc.Deprecated != "",
	}, nil
}

// splitFragment returns the URL/path up to "#" and the fragment after "#".
func splitFragment(id string) (base, fragment string) {
	base, fragment, _ = strings.Cut(id, "#")
	return base, fragment
}

// stripFragment returns the part before "#".
func stripFragment(id string) string {
	base, _, _ := strings.Cut(id, "#")
	return base
}

// safeSlice safely extracts a substring from content using byte offsets.
func safeSlice(content []byte, start, end int) string {
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

// rustDocParser is a package-level instance reused across all parse calls.
var rustDocParser = rustdoc.New()

// getRustDocParser returns the shared rustdoc parser instance.
func getRustDocParser() *rustdoc.Parser {
	return rustDocParser
}

// parseDocComment parses a Rust doc comment string using the rustdoc parser.
func parseDocComment(raw string) *parsedDoc {
	if raw == "" {
		return &parsedDoc{}
	}
	doc := getRustDocParser().Parse(raw)
	result := &parsedDoc{
		Summary:     doc.Summary,
		Description: doc.Description,
		Deprecated:  doc.Deprecated,
		Since:       doc.Since,
	}
	for _, param := range doc.Params {
		result.Params = append(result.Params, paramInfo{
			Name:        param.Name,
			Description: param.Description,
		})
	}
	if doc.Returns != nil {
		result.Returns = &returnInfo{
			Type:        doc.Returns.Type,
			Description: doc.Returns.Description,
		}
	}
	return result
}

// parsedDoc holds the structured information extracted from a doc comment.
type parsedDoc struct {
	Summary     string
	Description string
	Deprecated  string
	Since       string
	Params      []paramInfo
	Returns     *returnInfo
}

// paramInfo holds parameter documentation.
type paramInfo struct {
	Name        string
	Description string
}

// returnInfo holds return value documentation.
type returnInfo struct {
	Type        string
	Description string
}
