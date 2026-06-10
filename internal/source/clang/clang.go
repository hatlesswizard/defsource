// Package clang provides a documentation source adapter that parses C library
// source code from a local checkout of any C project (glibc, SQLite, OpenSSL,
// libcurl, etc.). It reads header files (.h) to discover the public API surface
// and uses tree-sitter for AST parsing plus regex for preprocessor macros.
package clang

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser/doxygen"
	"github.com/hatlesswizard/defsource/internal/source"
)

// CSource is a documentation source adapter for C libraries. It parses
// header files to discover public API entities (functions, structs, enums,
// unions, typedefs, and macros) and uses tree-sitter + regex to extract
// structured documentation.
type CSource struct {
	repoPath  string
	ref       string // git tag/branch for source-code links
	libraryID string // canonical ID like "/c/sqlite"
	meta      source.LibraryMeta
	index     *codebaseIndex
	docParser *doxygen.Parser
}

var _ source.Source = (*CSource)(nil)

// Option customises a CSource at construction time.
type Option func(*CSource)

// WithRef sets the git ref (tag or branch) that source-code links point at.
func WithRef(ref string) Option {
	return func(s *CSource) { s.ref = ref }
}

// Config holds the library-specific configuration for a C source adapter.
type Config struct {
	// LibraryID is the canonical library identifier (e.g., "/c/sqlite").
	LibraryID string

	// Name is the human-readable library name (e.g., "SQLite").
	Name string

	// Description is a short description of the library.
	Description string

	// SourceURL is the upstream repository URL.
	SourceURL string

	// Version is the library version string.
	Version string

	// TrustScore is the documentation trust score (0.0-1.0).
	TrustScore float64

	// HeaderDirs restricts discovery to these subdirectories (relative to repo root).
	// If empty, defaults to scanning the entire repo for .h files.
	HeaderDirs []string

	// ExcludePatterns are path substrings that skip files during discovery.
	ExcludePatterns []string
}

// New constructs a new CSource pointing at a local checkout of a C library.
func New(repoPath string, cfg Config, opts ...Option) *CSource {
	s := &CSource{
		repoPath:  repoPath,
		libraryID: cfg.LibraryID,
		meta: source.LibraryMeta{
			Name:        cfg.Name,
			Description: cfg.Description,
			SourceURL:   cfg.SourceURL,
			Version:     cfg.Version,
			Language:    "c",
			TrustScore:  cfg.TrustScore,
		},
		index:     emptyIndex(),
		docParser: doxygen.New(),
	}
	for _, opt := range opts {
		opt(s)
	}
	// Store config-level header dirs and excludes on the source for discovery.
	s.index.headerDirs = cfg.HeaderDirs
	s.index.excludePatterns = cfg.ExcludePatterns
	return s
}

// ID returns the canonical library ID (e.g., "/c/sqlite").
func (s *CSource) ID() string { return s.libraryID }

// Meta returns metadata for the library record.
func (s *CSource) Meta() source.LibraryMeta { return s.meta }

// DiscoverEntities walks the repository header files and returns a sorted list
// of entity identifiers in the form "filepath#entity_name".
func (s *CSource) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	idx, err := buildCodebaseIndex(s.repoPath, s.index.headerDirs, s.index.excludePatterns)
	if err != nil {
		return nil, fmt.Errorf("buildCodebaseIndex: %w", err)
	}
	s.index = idx
	ids := idx.buildEntityList()
	log.Printf("Discovered %d C entities from %s", len(ids), s.repoPath)
	return ids, nil
}

// ParseEntity parses a single entity from a header file identified by entityID.
// The entityID format is "filepath#entity_name".
func (s *CSource) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	filePath, fragment := splitFragment(entityID)
	if fragment == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	// Parse the file for all entities.
	analysis := parseFileC(content)

	// Also extract macros via regex.
	macros := extractMacros(content)

	// Find the entity matching the fragment.
	entity, methods := s.findEntity(analysis, macros, filePath, fragment, content)
	if entity == nil {
		return nil, nil, fmt.Errorf("entity %q not found in %s", fragment, filePath)
	}

	return entity, methods, nil
}

// ParseMethod parses a function or method from file content.
func (s *CSource) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	_, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	// Check if it is a struct field access (struct_name.field_name pattern).
	if strings.Contains(fragment, ".") {
		parts := strings.SplitN(fragment, ".", 2)
		return s.parseStructMethod(content, parts[0], parts[1])
	}

	// Parse as a standalone function.
	analysis := parseFileC(content)
	for i := range analysis.Functions {
		fn := &analysis.Functions[i]
		if fn.Name == fragment {
			return s.functionToMethod(fn, content), nil
		}
	}

	return nil, fmt.Errorf("function %q not found", fragment)
}

// DetectWrapper analyzes a method's source code for wrapper patterns.
func (s *CSource) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil || method.SourceCode == "" {
		return false, "", ""
	}
	return detectWrapper(method.SourceCode, s.index)
}

// ResolveWrapperURL constructs the identifier to fetch the wrapped target's source.
func (s *CSource) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "function":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
	case "macro":
		if path, ok := s.index.FileForMacro(targetName); ok {
			return path + "#" + targetName
		}
	}
	return ""
}

// ParseSourceCode extracts the source code of a specific entity from file content.
func (s *CSource) ParseSourceCode(entityID string, content []byte) (string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	analysis := parseFileC(content)

	// Check functions.
	for _, fn := range analysis.Functions {
		if fn.Name == fragment {
			return extractSourceRange(content, fn.StartPos, fn.EndPos), nil
		}
	}

	// Check structs.
	for _, st := range analysis.Structs {
		if st.Name == fragment {
			return extractSourceRange(content, st.StartPos, st.EndPos), nil
		}
	}

	// Check enums.
	for _, en := range analysis.Enums {
		if en.Name == fragment {
			return extractSourceRange(content, en.StartPos, en.EndPos), nil
		}
	}

	// Check unions.
	for _, un := range analysis.Unions {
		if un.Name == fragment {
			return extractSourceRange(content, un.StartPos, un.EndPos), nil
		}
	}

	// Check typedefs.
	for _, td := range analysis.Typedefs {
		if td.Name == fragment {
			return extractSourceRange(content, td.StartPos, td.EndPos), nil
		}
	}

	// Check macros via regex.
	macros := extractMacros(content)
	for _, m := range macros {
		if m.Name == fragment {
			return m.Body, nil
		}
	}

	return "", fmt.Errorf("entity %q not found", fragment)
}

// findEntity locates the named entity in a parsed file analysis and returns
// the source.Entity and any associated method identifiers.
func (s *CSource) findEntity(analysis *fileAnalysis, macros []macroDef, filePath, name string, content []byte) (*source.Entity, []string) {
	// Check functions.
	for i := range analysis.Functions {
		fn := &analysis.Functions[i]
		if fn.Name == name {
			doc := s.docParser.Parse(fn.DocComment)
			entity := &source.Entity{
				Slug:        fn.Name,
				Name:        fn.Name,
				Kind:        source.KindFunction,
				Description: doc.Summary,
				SourceFile:  filePath,
				SourceCode:  extractSourceRange(content, fn.StartPos, fn.EndPos),
				Visibility:  "public",
				Deprecated:  doc.Deprecated != "",
			}
			return entity, nil
		}
	}

	// Check structs.
	for i := range analysis.Structs {
		st := &analysis.Structs[i]
		if st.Name == name {
			doc := s.docParser.Parse(st.DocComment)
			props := make([]source.Property, 0, len(st.Fields))
			for _, f := range st.Fields {
				props = append(props, source.Property{
					Name:       f.Name,
					Type:       f.Type,
					Visibility: "public",
				})
			}
			entity := &source.Entity{
				Slug:        st.Name,
				Name:        st.Name,
				Kind:        source.KindStruct,
				Description: doc.Summary,
				SourceFile:  filePath,
				SourceCode:  extractSourceRange(content, st.StartPos, st.EndPos),
				Visibility:  "public",
				Deprecated:  doc.Deprecated != "",
				Properties:  props,
			}
			return entity, nil
		}
	}

	// Check enums.
	for i := range analysis.Enums {
		en := &analysis.Enums[i]
		if en.Name == name {
			doc := s.docParser.Parse(en.DocComment)
			props := make([]source.Property, 0, len(en.Constants))
			for _, c := range en.Constants {
				props = append(props, source.Property{
					Name:        c.Name,
					Type:        "int",
					Description: c.Value,
					Visibility:  "public",
				})
			}
			entity := &source.Entity{
				Slug:        en.Name,
				Name:        en.Name,
				Kind:        source.KindEnum,
				Description: doc.Summary,
				SourceFile:  filePath,
				SourceCode:  extractSourceRange(content, en.StartPos, en.EndPos),
				Visibility:  "public",
				Deprecated:  doc.Deprecated != "",
				Properties:  props,
			}
			return entity, nil
		}
	}

	// Check unions.
	for i := range analysis.Unions {
		un := &analysis.Unions[i]
		if un.Name == name {
			doc := s.docParser.Parse(un.DocComment)
			props := make([]source.Property, 0, len(un.Fields))
			for _, f := range un.Fields {
				props = append(props, source.Property{
					Name:       f.Name,
					Type:       f.Type,
					Visibility: "public",
				})
			}
			entity := &source.Entity{
				Slug:        un.Name,
				Name:        un.Name,
				Kind:        source.KindUnion,
				Description: doc.Summary,
				SourceFile:  filePath,
				SourceCode:  extractSourceRange(content, un.StartPos, un.EndPos),
				Visibility:  "public",
				Deprecated:  doc.Deprecated != "",
				Properties:  props,
			}
			return entity, nil
		}
	}

	// Check typedefs.
	for i := range analysis.Typedefs {
		td := &analysis.Typedefs[i]
		if td.Name == name {
			doc := s.docParser.Parse(td.DocComment)
			entity := &source.Entity{
				Slug:        td.Name,
				Name:        td.Name,
				Kind:        source.KindTypeAlias,
				Description: doc.Summary,
				SourceFile:  filePath,
				SourceCode:  extractSourceRange(content, td.StartPos, td.EndPos),
				Visibility:  "public",
				Deprecated:  doc.Deprecated != "",
			}
			return entity, nil
		}
	}

	// Check macros.
	for i := range macros {
		m := &macros[i]
		if m.Name == name {
			entity := &source.Entity{
				Slug:        m.Name,
				Name:        m.Name,
				Kind:        source.KindMacro,
				Description: m.DocComment,
				SourceFile:  filePath,
				SourceCode:  m.Body,
				Visibility:  "public",
			}
			return entity, nil
		}
	}

	return nil, nil
}

// functionToMethod converts a parsed function definition into a source.Method.
func (s *CSource) functionToMethod(fn *functionDef, content []byte) *source.Method {
	doc := s.docParser.Parse(fn.DocComment)
	params := make([]source.Parameter, 0, len(fn.Params))
	for _, p := range fn.Params {
		desc := ""
		for _, dp := range doc.Params {
			if dp.Name == p.Name {
				desc = dp.Description
				break
			}
		}
		params = append(params, source.Parameter{
			Name:        p.Name,
			Type:        p.Type,
			Required:    true,
			Description: desc,
		})
	}

	retType := fn.ReturnType
	retDesc := ""
	if doc.Returns != nil {
		if doc.Returns.Type != "" {
			retType = doc.Returns.Type
		}
		retDesc = doc.Returns.Description
	}

	return &source.Method{
		Slug:       fn.Name,
		Name:       fn.Name,
		Signature:  fn.Signature,
		Description: doc.Summary,
		Parameters: params,
		ReturnType: retType,
		ReturnDesc: retDesc,
		SourceCode: extractSourceRange(content, fn.StartPos, fn.EndPos),
		Since:      doc.Since,
		Deprecated: doc.Deprecated != "",
	}
}

// parseStructMethod parses a struct "method" — in C, this is represented
// as a function pointer field within a struct.
func (s *CSource) parseStructMethod(content []byte, structName, fieldName string) (*source.Method, error) {
	analysis := parseFileC(content)
	for _, st := range analysis.Structs {
		if st.Name == structName {
			for _, f := range st.Fields {
				if f.Name == fieldName {
					return &source.Method{
						Slug:      fieldName,
						Name:      fieldName,
						Signature: f.Type + " " + f.Name,
						Description: "",
					}, nil
				}
			}
			return nil, fmt.Errorf("field %q not found in struct %q", fieldName, structName)
		}
	}
	return nil, fmt.Errorf("struct %q not found", structName)
}

// splitFragment splits an entityID at the '#' character.
func splitFragment(entityID string) (path, fragment string) {
	if idx := strings.LastIndex(entityID, "#"); idx >= 0 {
		return entityID[:idx], entityID[idx+1:]
	}
	return entityID, ""
}

// extractSourceRange safely extracts a byte range from content.
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
