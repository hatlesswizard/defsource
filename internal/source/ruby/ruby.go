// Package ruby provides a documentation source adapter that reads Ruby
// source code from a local checkout of a Ruby framework repository
// (Rails, RSpec, Sidekiq, etc.). It parses raw Ruby source via tree-sitter
// to extract classes, modules, methods, and YARD documentation without
// making any network calls during parsing.
package ruby

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser/yard"
	"github.com/hatlesswizard/defsource/internal/source"
)

// RubySource is a documentation source adapter that reads Ruby source
// code from a local repository checkout. It is configurable for any Ruby
// framework (Rails, RSpec, Sidekiq, etc.).
type RubySource struct {
	repoPath    string
	libraryID   string // e.g., "/ruby/rails"
	meta        source.LibraryMeta
	ref         string // git tag/branch for source-code links
	sourceRoots []string
	index       *codebaseIndex
	docParser   *yard.Parser
}

var _ source.Source = (*RubySource)(nil)

// Option customises a RubySource at construction time.
type Option func(*RubySource)

// WithRef sets the git ref (tag or branch) for source-code links,
// e.g. WithRef("v7.1.2") produces links under .../blob/v7.1.2/.
func WithRef(ref string) Option {
	return func(s *RubySource) { s.ref = ref }
}

// Config holds the framework-specific configuration for a RubySource.
type Config struct {
	// LibraryID is the canonical identifier (e.g., "/ruby/rails").
	LibraryID string

	// Name is the display name for the library (e.g., "Rails Reference").
	Name string

	// Description is the library description.
	Description string

	// SourceURL is the GitHub URL for the framework (e.g., "https://github.com/rails/rails").
	SourceURL string

	// Version is the version string for the library.
	Version string

	// TrustScore is the relevance trust score (0-1).
	TrustScore float64

	// SourceRoots are the directory paths relative to the repo root that
	// contain the framework's source code (e.g., ["lib"] for gems,
	// ["activerecord/lib"] for Rails components).
	SourceRoots []string
}

// New constructs a new RubySource for the given local repository path
// and framework configuration.
func New(repoPath string, cfg Config, opts ...Option) *RubySource {
	s := &RubySource{
		repoPath:    repoPath,
		libraryID:   cfg.LibraryID,
		sourceRoots: cfg.SourceRoots,
		meta: source.LibraryMeta{
			Name:        cfg.Name,
			Description: cfg.Description,
			SourceURL:   cfg.SourceURL,
			Version:     cfg.Version,
			Language:    "ruby",
			TrustScore:  cfg.TrustScore,
		},
		index:     emptyIndex(),
		docParser: yard.New(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// blobBase returns the GitHub blob URL prefix for source-code links.
func (s *RubySource) blobBase() string {
	ref := s.ref
	if ref == "" {
		ref = "main"
	}
	return strings.TrimSuffix(s.meta.SourceURL, "/") + "/blob/" + ref + "/"
}

// ID returns the canonical library ID.
func (s *RubySource) ID() string { return s.libraryID }

// Meta returns metadata for the library record.
func (s *RubySource) Meta() source.LibraryMeta { return s.meta }

// DiscoverEntities walks the local repository and returns a sorted list
// of entity identifiers in the form "filepath#Module::ClassName".
func (s *RubySource) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	roots := make([]string, 0, len(s.sourceRoots))
	if len(s.sourceRoots) > 0 {
		for _, r := range s.sourceRoots {
			roots = append(roots, filepath.Join(s.repoPath, r))
		}
	} else {
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
func (s *RubySource) inferRoots() []string {
	// Try standard gem layout: lib/
	libDir := filepath.Join(s.repoPath, "lib")
	return []string{libDir}
}

// ParseEntity parses a single entity from file content, identified by entityID.
// The entityID is of the form "filepath#Module::ClassName".
func (s *RubySource) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	filePath, fragment := splitFragment(entityID)
	if fragment == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Look for a class
	for i := range analysis.Classes {
		cls := &analysis.Classes[i]
		if cls.QualifiedName == fragment || cls.Name == fragment {
			return s.buildClassEntity(cls, content, filePath)
		}
	}

	// Look for a module
	for i := range analysis.Modules {
		mod := &analysis.Modules[i]
		if mod.QualifiedName == fragment || mod.Name == fragment {
			return s.buildModuleEntity(mod, content, filePath)
		}
	}

	// Look for a module-level function
	for i := range analysis.Functions {
		fn := &analysis.Functions[i]
		if fn.Name == fragment {
			return s.buildFunctionEntity(fn, content, filePath)
		}
	}

	return nil, nil, fmt.Errorf("entity %q not found in %s", fragment, filePath)
}

// buildClassEntity constructs an Entity and method list from a parsed class.
func (s *RubySource) buildClassEntity(cls *classDef, content []byte, filePath string) (*source.Entity, []string, error) {
	doc := s.docParser.Parse(cls.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + filepath.ToSlash(rel)
	if cls.StartLine > 0 {
		githubURL += fmt.Sprintf("#L%d", cls.StartLine)
	}

	properties := make([]source.Property, 0, len(cls.Attributes))
	for _, attr := range cls.Attributes {
		properties = append(properties, source.Property{
			Name:       attr.Name,
			Type:       attr.Type,
			Visibility: attr.Visibility,
		})
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(strings.ReplaceAll(cls.QualifiedName, "::", "-")),
		Name:        cls.QualifiedName,
		Kind:        source.KindClass,
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
		methodID := filePath + "#" + cls.QualifiedName + "#" + m.Name
		methodIDs = append(methodIDs, methodID)
	}
	for _, m := range cls.SingletonMethods {
		methodID := filePath + "#" + cls.QualifiedName + ".self." + m.Name
		methodIDs = append(methodIDs, methodID)
	}

	return entity, methodIDs, nil
}

// buildModuleEntity constructs an Entity and method list from a parsed module.
func (s *RubySource) buildModuleEntity(mod *moduleDef, content []byte, filePath string) (*source.Entity, []string, error) {
	doc := s.docParser.Parse(mod.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + filepath.ToSlash(rel)
	if mod.StartLine > 0 {
		githubURL += fmt.Sprintf("#L%d", mod.StartLine)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(strings.ReplaceAll(mod.QualifiedName, "::", "-")),
		Name:        mod.QualifiedName,
		Kind:        source.KindModule,
		Description: doc.Summary,
		SourceFile:  rel,
		SourceCode:  string(content[mod.StartPos:mod.EndPos]),
		URL:         githubURL,
		Visibility:  "public",
		Deprecated:  doc.Deprecated != "",
	}

	// Build method IDs
	var methodIDs []string
	for _, m := range mod.Methods {
		methodID := filePath + "#" + mod.QualifiedName + "#" + m.Name
		methodIDs = append(methodIDs, methodID)
	}
	for _, m := range mod.SingletonMethods {
		methodID := filePath + "#" + mod.QualifiedName + ".self." + m.Name
		methodIDs = append(methodIDs, methodID)
	}

	return entity, methodIDs, nil
}

// buildFunctionEntity constructs an Entity from a module-level function.
func (s *RubySource) buildFunctionEntity(fn *methodDef, content []byte, filePath string) (*source.Entity, []string, error) {
	doc := s.docParser.Parse(fn.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + filepath.ToSlash(rel)
	if fn.StartLine > 0 {
		githubURL += fmt.Sprintf("#L%d", fn.StartLine)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(fn.Name),
		Name:        fn.Name,
		Kind:        source.KindFunction,
		Description: doc.Summary,
		SourceFile:  rel,
		SourceCode:  string(content[fn.StartPos:fn.EndPos]),
		URL:         githubURL,
		Visibility:  fn.Visibility,
		Deprecated:  doc.Deprecated != "",
	}

	return entity, nil, nil
}

// ParseMethod parses a single method from file content.
// The methodID is of the form "filepath#Module::ClassName#methodName"
// or "filepath#Module::ClassName.self.methodName" for singleton methods.
func (s *RubySource) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	filePath, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	// Fragment is either "ClassName#methodName" or "ClassName.self.methodName"
	var entityName, methodName string
	var isSingleton bool

	if idx := strings.Index(fragment, ".self."); idx >= 0 {
		entityName = fragment[:idx]
		methodName = fragment[idx+6:] // len(".self.") == 6
		isSingleton = true
	} else if idx := strings.LastIndex(fragment, "#"); idx >= 0 {
		entityName = fragment[:idx]
		methodName = fragment[idx+1:]
	} else {
		return nil, fmt.Errorf("invalid method fragment %q", fragment)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Look for method in classes
	for i := range analysis.Classes {
		cls := &analysis.Classes[i]
		if cls.QualifiedName == entityName || cls.Name == entityName {
			return s.findAndBuildMethod(cls.Methods, cls.SingletonMethods, entityName, methodName, isSingleton, content, filePath)
		}
	}

	// Look for method in modules
	for i := range analysis.Modules {
		mod := &analysis.Modules[i]
		if mod.QualifiedName == entityName || mod.Name == entityName {
			return s.findAndBuildMethod(mod.Methods, mod.SingletonMethods, entityName, methodName, isSingleton, content, filePath)
		}
	}

	return nil, fmt.Errorf("entity %q not found in %s", entityName, filePath)
}

// findAndBuildMethod locates and builds a Method from the appropriate list.
func (s *RubySource) findAndBuildMethod(methods, singletonMethods []methodDef, entityName, methodName string, isSingleton bool, content []byte, filePath string) (*source.Method, error) {
	var method *methodDef

	if isSingleton {
		for i := range singletonMethods {
			if singletonMethods[i].Name == methodName {
				method = &singletonMethods[i]
				break
			}
		}
	} else {
		for i := range methods {
			if methods[i].Name == methodName {
				method = &methods[i]
				break
			}
		}
	}

	if method == nil {
		return nil, fmt.Errorf("method %q not found in %q", methodName, entityName)
	}

	doc := s.docParser.Parse(method.DocComment)

	rel := relativePath(s.repoPath, filePath)
	githubURL := s.blobBase() + filepath.ToSlash(rel)
	if method.StartLine > 0 {
		githubURL += fmt.Sprintf("#L%d", method.StartLine)
	}

	parameters := make([]source.Parameter, 0, len(method.Params))
	for _, p := range method.Params {
		param := source.Parameter{
			Name:     p.Name,
			Type:     p.Type,
			Required: p.Default == "" && !p.Splat && !p.DoubleSplat && !p.Block,
		}
		// Enrich from YARD if available
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

	signature := buildMethodSignature(entityName, method, isSingleton)

	var returnType, returnDesc string
	if doc.Returns != nil {
		returnType = doc.Returns.Type
		returnDesc = doc.Returns.Description
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
func (s *RubySource) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil || method.SourceCode == "" {
		return false, "", ""
	}
	return detectWrapper([]byte(method.SourceCode), s.index)
}

// ResolveWrapperURL constructs the identifier to fetch the wrapped target's source.
func (s *RubySource) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "method":
		// targetName is "ClassName#methodName"
		if className, _, ok := strings.Cut(targetName, "#"); ok {
			if path, ok := s.index.FileForClass(className); ok {
				return path + "#" + targetName
			}
			if path, ok := s.index.FileForModule(className); ok {
				return path + "#" + targetName
			}
		}
	case "function":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
	case "delegate":
		// targetName is "object.method" — look up via entity
		if path, canonical, ok := s.index.LookupClass(entitySlug); ok {
			return path + "#" + canonical + "#" + targetName
		}
	}
	return ""
}

// ParseSourceCode extracts just the source code of a specific entity or
// method from file content, using the fragment in the identifier.
func (s *RubySource) ParseSourceCode(entityID string, content []byte) (string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	analysis := parseFile(content)
	if analysis == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	// Try "EntityName#methodName" for instance methods
	if entityName, methodName, ok := strings.Cut(fragment, "#"); ok && methodName != "" {
		// Check classes
		for i := range analysis.Classes {
			cls := &analysis.Classes[i]
			if cls.QualifiedName == entityName || cls.Name == entityName {
				for _, m := range cls.Methods {
					if m.Name == methodName {
						return string(content[m.StartPos:m.EndPos]), nil
					}
				}
				for _, m := range cls.SingletonMethods {
					if m.Name == methodName {
						return string(content[m.StartPos:m.EndPos]), nil
					}
				}
				return "", fmt.Errorf("method %q not found in %q", methodName, entityName)
			}
		}
		// Check modules
		for i := range analysis.Modules {
			mod := &analysis.Modules[i]
			if mod.QualifiedName == entityName || mod.Name == entityName {
				for _, m := range mod.Methods {
					if m.Name == methodName {
						return string(content[m.StartPos:m.EndPos]), nil
					}
				}
				for _, m := range mod.SingletonMethods {
					if m.Name == methodName {
						return string(content[m.StartPos:m.EndPos]), nil
					}
				}
				return "", fmt.Errorf("method %q not found in %q", methodName, entityName)
			}
		}
	}

	// Try "EntityName.self.methodName" for singleton methods
	if idx := strings.Index(fragment, ".self."); idx >= 0 {
		entityName := fragment[:idx]
		methodName := fragment[idx+6:]
		for i := range analysis.Classes {
			cls := &analysis.Classes[i]
			if cls.QualifiedName == entityName || cls.Name == entityName {
				for _, m := range cls.SingletonMethods {
					if m.Name == methodName {
						return string(content[m.StartPos:m.EndPos]), nil
					}
				}
			}
		}
		for i := range analysis.Modules {
			mod := &analysis.Modules[i]
			if mod.QualifiedName == entityName || mod.Name == entityName {
				for _, m := range mod.SingletonMethods {
					if m.Name == methodName {
						return string(content[m.StartPos:m.EndPos]), nil
					}
				}
			}
		}
	}

	// Try as a class/module name
	for _, cls := range analysis.Classes {
		if cls.QualifiedName == fragment || cls.Name == fragment {
			return string(content[cls.StartPos:cls.EndPos]), nil
		}
	}
	for _, mod := range analysis.Modules {
		if mod.QualifiedName == fragment || mod.Name == fragment {
			return string(content[mod.StartPos:mod.EndPos]), nil
		}
	}

	// Try as a top-level function
	for _, fn := range analysis.Functions {
		if fn.Name == fragment {
			return string(content[fn.StartPos:fn.EndPos]), nil
		}
	}

	return "", fmt.Errorf("entity %q not found", fragment)
}

// buildMethodSignature constructs a Ruby method signature string.
func buildMethodSignature(entityName string, method *methodDef, isSingleton bool) string {
	var b strings.Builder

	b.WriteString("def ")
	if isSingleton {
		b.WriteString(entityName)
		b.WriteString(".")
	} else {
		b.WriteString(entityName)
		b.WriteString("#")
	}
	b.WriteString(method.Name)

	if len(method.Params) > 0 {
		b.WriteString("(")
		var parts []string
		for _, p := range method.Params {
			parts = append(parts, formatParam(p))
		}
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString(")")
	}

	return b.String()
}

// formatParam formats a single parameter for display in a signature.
func formatParam(p paramDef) string {
	var b strings.Builder
	if p.DoubleSplat {
		b.WriteString("**")
	} else if p.Splat {
		b.WriteString("*")
	} else if p.Block {
		b.WriteString("&")
	}
	b.WriteString(p.Name)
	if p.Default != "" {
		b.WriteString(" = ")
		b.WriteString(p.Default)
	}
	return b.String()
}

// splitFragment splits "path#fragment" into (path, fragment).
func splitFragment(id string) (string, string) {
	if idx := strings.Index(id, "#"); idx >= 0 {
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
