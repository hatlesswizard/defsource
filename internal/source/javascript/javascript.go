// Package javascript provides a documentation source adapter that reads
// JavaScript source code from a local directory (downloaded from GitHub).
// It parses raw JS source via tree-sitter to extract classes, functions,
// module exports, and JSDoc without making any network calls during parsing.
//
// The adapter is configurable for any JavaScript library (Node.js, React,
// Express, Lodash, etc.) via LibraryConfig.
package javascript

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/hatlesswizard/defsource/internal/docparser"
	"github.com/hatlesswizard/defsource/internal/docparser/jsdoc"
	"github.com/hatlesswizard/defsource/internal/source"
)

// LibraryConfig describes a JavaScript library to index.
type LibraryConfig struct {
	// ID is the canonical library ID (e.g., "/javascript/express").
	ID string

	// Name is the human-readable library name (e.g., "Express.js").
	Name string

	// Description is a short summary of what the library provides.
	Description string

	// SourceURL is the GitHub repository URL.
	SourceURL string

	// Version is the library version being indexed.
	Version string

	// SourceDirs are relative paths within the repo to scan for JS source.
	// If empty, the root directory is scanned.
	SourceDirs []string

	// BlobRef is the git ref for source-code links (e.g., "v4.18.2").
	// Defaults to "main" if empty.
	BlobRef string
}

// Source is a documentation source adapter that reads JavaScript source code
// from a local directory. It implements the source.Source interface.
type Source struct {
	repoPath  string
	config    LibraryConfig
	index     *codebaseIndex
	jsdocParser *jsdoc.Parser
}

var _ source.Source = (*Source)(nil)

// Option customises a Source at construction time.
type Option func(*Source)

// WithConfig sets the library configuration.
func WithConfig(cfg LibraryConfig) Option {
	return func(s *Source) { s.config = cfg }
}

// New constructs a new JavaScript Source pointing at a local directory.
// The index is initialised to an empty (non-nil) state so that DetectWrapper
// and ResolveWrapperURL are safe to call before DiscoverEntities.
func New(repoPath string, opts ...Option) *Source {
	s := &Source{
		repoPath:    repoPath,
		index:       emptyIndex(),
		jsdocParser: jsdoc.New(),
		config: LibraryConfig{
			ID:      "/javascript/unknown",
			Name:    "JavaScript Library",
			BlobRef: "main",
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.config.BlobRef == "" {
		s.config.BlobRef = "main"
	}
	return s
}

// blobBase returns the GitHub blob URL prefix for source-code links.
func (s *Source) blobBase() string {
	url := strings.TrimSuffix(s.config.SourceURL, ".git")
	if url == "" {
		url = "https://github.com/unknown/unknown"
	}
	return url + "/blob/" + s.config.BlobRef + "/"
}

// ID returns the canonical library ID.
func (s *Source) ID() string { return s.config.ID }

// Meta returns metadata for the library record.
func (s *Source) Meta() source.LibraryMeta {
	return source.LibraryMeta{
		Name:        s.config.Name,
		Description: s.config.Description,
		SourceURL:   s.config.SourceURL,
		Version:     s.config.Version,
		Language:    "javascript",
		TrustScore:  0.90,
	}
}

// DiscoverEntities walks the local repository and returns a sorted list of
// entity identifiers. The fetch parameter is unused — all discovery reads
// files locally.
func (s *Source) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	dirs := s.config.SourceDirs
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	roots := make([]string, 0, len(dirs))
	for _, d := range dirs {
		roots = append(roots, filepath.Join(s.repoPath, d))
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

// ParseEntity parses a single entity from file content, identified by entityID.
// Returns the entity data plus a list of method identifiers to crawl next.
func (s *Source) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, nil, fmt.Errorf("failed to parse file for %s", entityID)
	}

	filePath := entityIDToPath(entityID)
	rel := relativePath(s.repoPath, filePath)

	// Try to find the entity in classes first.
	for _, cls := range analysis.Classes {
		if cls.Name == fragment {
			return s.buildClassEntity(cls, content, rel, entityID)
		}
	}

	// Then in functions.
	for _, fn := range analysis.Functions {
		if fn.Name == fragment {
			return s.buildFunctionEntity(fn, content, rel, entityID)
		}
	}

	// Then in module exports.
	for _, mod := range analysis.Modules {
		if mod.Name == fragment {
			return s.buildModuleEntity(mod, content, rel, entityID)
		}
	}

	// Then in constants.
	for _, c := range analysis.Constants {
		if c.Name == fragment {
			return s.buildConstantEntity(c, content, rel, entityID)
		}
	}

	return nil, nil, fmt.Errorf("entity %q not found in file", fragment)
}

// ParseMethod parses a single method from file content.
func (s *Source) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	_, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	// Fragment format: ClassName.methodName or ClassName.prototype.methodName
	className, methodName, ok := parseMethodFragment(fragment)
	if !ok {
		return nil, fmt.Errorf("invalid method fragment %q: expected ClassName.methodName", fragment)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, fmt.Errorf("failed to parse file for %s", methodID)
	}

	filePath := entityIDToPath(methodID)
	rel := relativePath(s.repoPath, filePath)

	// Look in classes.
	for _, cls := range analysis.Classes {
		if cls.Name == className {
			for _, m := range cls.Methods {
				if m.Name == methodName {
					return s.buildMethod(m, className, content, rel), nil
				}
			}
			return nil, fmt.Errorf("method %q not found in class %q", methodName, className)
		}
	}

	// Look in module exports for object methods.
	for _, mod := range analysis.Modules {
		if mod.Name == className {
			for _, m := range mod.Methods {
				if m.Name == methodName {
					return s.buildMethod(m, className, content, rel), nil
				}
			}
			return nil, fmt.Errorf("method %q not found in module %q", methodName, className)
		}
	}

	return nil, fmt.Errorf("class/module %q not found in %s", className, methodID)
}

// DetectWrapper analyzes a method's source code for wrapper patterns.
func (s *Source) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil || method.SourceCode == "" {
		return false, "", ""
	}
	return detectWrapper([]byte(method.SourceCode), s.index.builtinFunctions)
}

// ResolveWrapperURL constructs the identifier to fetch the wrapped target's
// source. Returns an empty string if the target cannot be resolved.
func (s *Source) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "function":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
	case "method":
		// targetName is "ClassName.methodName"
		parts := strings.SplitN(targetName, ".", 2)
		if len(parts) != 2 {
			return ""
		}
		className := parts[0]
		if path, ok := s.index.FileForClass(className); ok {
			return path + "#" + targetName
		}
	case "module_method":
		parts := strings.SplitN(targetName, ".", 2)
		if len(parts) != 2 {
			return ""
		}
		moduleName := parts[0]
		if path, ok := s.index.FileForModule(moduleName); ok {
			return path + "#" + targetName
		}
	}
	return ""
}

// ParseSourceCode extracts just the source code of a specific entity or
// method from file content.
func (s *Source) ParseSourceCode(entityID string, content []byte) (string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	analysis := parseFile(content)
	if analysis == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	// Check if it's a method reference (ClassName.methodName).
	if className, methodName, ok := parseMethodFragment(fragment); ok {
		for _, cls := range analysis.Classes {
			if cls.Name == className {
				for _, m := range cls.Methods {
					if m.Name == methodName {
						return string(content[m.StartPos:m.EndPos]), nil
					}
				}
			}
		}
		for _, mod := range analysis.Modules {
			if mod.Name == className {
				for _, m := range mod.Methods {
					if m.Name == methodName {
						return string(content[m.StartPos:m.EndPos]), nil
					}
				}
			}
		}
		return "", fmt.Errorf("method %q not found in %q", methodName, className)
	}

	// Look for entity by name.
	for _, cls := range analysis.Classes {
		if cls.Name == fragment {
			return string(content[cls.StartPos:cls.EndPos]), nil
		}
	}
	for _, fn := range analysis.Functions {
		if fn.Name == fragment {
			return string(content[fn.StartPos:fn.EndPos]), nil
		}
	}
	for _, mod := range analysis.Modules {
		if mod.Name == fragment {
			return string(content[mod.StartPos:mod.EndPos]), nil
		}
	}
	for _, c := range analysis.Constants {
		if c.Name == fragment {
			return string(content[c.StartPos:c.EndPos]), nil
		}
	}

	return "", fmt.Errorf("entity %q not found", fragment)
}

// buildClassEntity creates a source.Entity from a classDef.
func (s *Source) buildClassEntity(cls classDef, content []byte, rel, entityID string) (*source.Entity, []string, error) {
	doc := s.parseDoc(cls.DocComment)

	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, cls.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	properties := make([]source.Property, 0, len(cls.Properties))
	for _, p := range cls.Properties {
		pdoc := s.parseDoc(p.DocComment)
		properties = append(properties, source.Property{
			Name:        p.Name,
			Type:        p.Type,
			Description: pdoc.Summary,
			Visibility:  p.Visibility,
			Since:       pdoc.Since,
		})
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(cls.Name),
		Name:        cls.Name,
		Kind:        source.KindClass,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  string(content[cls.StartPos:cls.EndPos]),
		URL:         githubURL,
		Properties:  properties,
	}

	filePath := entityIDToPath(entityID)
	methodURLs := make([]string, 0, len(cls.Methods))
	for _, m := range cls.Methods {
		methodURLs = append(methodURLs, filePath+"#"+cls.Name+"."+m.Name)
	}

	return entity, methodURLs, nil
}

// buildFunctionEntity creates a source.Entity from a funcDef.
func (s *Source) buildFunctionEntity(fn funcDef, content []byte, rel, entityID string) (*source.Entity, []string, error) {
	doc := s.parseDoc(fn.DocComment)

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
		SourceCode:  string(content[fn.StartPos:fn.EndPos]),
		URL:         githubURL,
	}

	return entity, nil, nil
}

// buildModuleEntity creates a source.Entity from a moduleDef.
func (s *Source) buildModuleEntity(mod moduleDef, content []byte, rel, entityID string) (*source.Entity, []string, error) {
	doc := s.parseDoc(mod.DocComment)

	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, mod.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(mod.Name),
		Name:        mod.Name,
		Kind:        source.KindModule,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  string(content[mod.StartPos:mod.EndPos]),
		URL:         githubURL,
	}

	filePath := entityIDToPath(entityID)
	methodURLs := make([]string, 0, len(mod.Methods))
	for _, m := range mod.Methods {
		methodURLs = append(methodURLs, filePath+"#"+mod.Name+"."+m.Name)
	}

	return entity, methodURLs, nil
}

// buildConstantEntity creates a source.Entity from a constDef.
func (s *Source) buildConstantEntity(c constDef, content []byte, rel, entityID string) (*source.Entity, []string, error) {
	doc := s.parseDoc(c.DocComment)

	githubURL := s.blobBase() + rel
	if ln := lineNumber(content, c.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        strings.ToLower(c.Name),
		Name:        c.Name,
		Kind:        source.KindConstant,
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  string(content[c.StartPos:c.EndPos]),
		URL:         githubURL,
	}

	return entity, nil, nil
}

// buildMethod creates a source.Method from a methodDef.
func (s *Source) buildMethod(m methodDef, ownerName string, content []byte, rel string) *source.Method {
	doc := s.parseDoc(m.DocComment)

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

	var returnType, returnDesc string
	if doc.Returns != nil {
		returnType = doc.Returns.Type
		returnDesc = doc.Returns.Description
	}

	relations := make([]source.Relation, 0, len(doc.See))
	for _, see := range doc.See {
		relations = append(relations, source.Relation{
			Kind:       "uses",
			TargetName: see,
		})
	}

	return &source.Method{
		Slug:        strings.ToLower(m.Name),
		Name:        m.Name,
		Signature:   buildMethodSignature(ownerName, m),
		Description: doc.Description,
		Parameters:  parameters,
		ReturnType:  returnType,
		ReturnDesc:  returnDesc,
		SourceCode:  string(content[m.StartPos:m.EndPos]),
		URL:         githubURL,
		Since:       doc.Since,
		Deprecated:  doc.Deprecated != "",
		Relations:   relations,
	}
}

// parseDoc parses a JSDoc comment into a structured DocComment.
func (s *Source) parseDoc(raw string) *docparser.DocComment {
	if raw == "" {
		return &docparser.DocComment{}
	}
	return s.jsdocParser.Parse(raw)
}

// splitFragment returns the path up to and excluding "#", and the fragment
// after "#". If the string contains no "#", fragment is "".
func splitFragment(s string) (base, fragment string) {
	base, fragment, _ = strings.Cut(s, "#")
	return base, fragment
}

// entityIDToPath extracts just the file path from an entity ID (strips fragment).
func entityIDToPath(entityID string) string {
	path, _, _ := strings.Cut(entityID, "#")
	return path
}

// parseMethodFragment parses a method fragment like "ClassName.methodName"
// or "ClassName.prototype.methodName" into (className, methodName, ok).
func parseMethodFragment(fragment string) (className, methodName string, ok bool) {
	// Handle "ClassName.prototype.methodName"
	if strings.Contains(fragment, ".prototype.") {
		parts := strings.SplitN(fragment, ".prototype.", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], true
		}
	}

	// Handle "ClassName.methodName"
	idx := strings.LastIndex(fragment, ".")
	if idx > 0 && idx < len(fragment)-1 {
		return fragment[:idx], fragment[idx+1:], true
	}

	return "", "", false
}

// relativePath returns absPath relative to repoPath, falling back to
// absPath itself if the relative path cannot be computed.
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

// buildMethodSignature constructs a method signature string.
func buildMethodSignature(ownerName string, m methodDef) string {
	prefix := ownerName + "."
	if m.Static {
		prefix += "(static) "
	}
	if m.IsGetter {
		return prefix + "get " + m.Name
	}
	if m.IsSetter {
		return prefix + "set " + m.Name + "(" + m.Signature + ")"
	}
	return prefix + m.Name + "(" + m.Signature + ")"
}
