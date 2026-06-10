// Package java provides a documentation source adapter that reads Java source
// code from a local clone of a GitHub repository. It parses raw Java source via
// tree-sitter to extract classes, interfaces, enums, records, annotations,
// methods, fields, and JavaDoc without making any network calls during parsing.
package java

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/hatlesswizard/defsource/internal/source"
)

// Config holds the configuration for a Java source adapter. Each Java library
// (Spring, Hibernate, JUnit, etc.) uses its own Config.
type Config struct {
	// Owner is the GitHub repository owner (e.g., "spring-projects").
	Owner string
	// Repo is the GitHub repository name (e.g., "spring-framework").
	Repo string
	// LibraryID is the canonical library ID (e.g., "java/spring-framework").
	LibraryID string
	// Name is the display name (e.g., "Spring Framework").
	Name string
	// Description is a short description of the library.
	Description string
	// SourceURL is the GitHub URL of the project.
	SourceURL string
	// SourceRoots are the root directories to walk for Java source files,
	// relative to the repository root. If empty, defaults to ["src/main/java"].
	SourceRoots []string
	// ExcludePatterns are glob patterns (relative to repo root) to exclude
	// from discovery. Common: ["**/test/**", "**/internal/**"].
	ExcludePatterns []string
	// TagFilter filters valid tags during version resolution.
	TagFilter func(string) bool
}

// Source is a documentation source adapter for Java libraries.
type Source struct {
	repoPath string
	ref      string
	config   Config
	index    *codebaseIndex
}

var _ source.Source = (*Source)(nil)

// Option customises a Source at construction time.
type Option func(*Source)

// WithRef sets the git ref (tag or branch) for source-code links.
func WithRef(ref string) Option {
	return func(s *Source) { s.ref = ref }
}

// New constructs a new Java Source pointing at a local clone.
func New(repoPath string, cfg Config, opts ...Option) *Source {
	s := &Source{
		repoPath: repoPath,
		config:   cfg,
		index:    emptyIndex(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ID returns the canonical library ID.
func (s *Source) ID() string { return s.config.LibraryID }

// Meta returns metadata for the library record.
func (s *Source) Meta() source.LibraryMeta {
	return source.LibraryMeta{
		Name:        s.config.Name,
		Description: s.config.Description,
		SourceURL:   s.config.SourceURL,
		Version:     s.ref,
		Language:    "java",
		TrustScore:  0.90,
	}
}

// DiscoverEntities walks the local repository and returns a sorted list of
// entity identifiers. Each identifier is of the form filepath#ClassName
// (or filepath#Outer.Inner for inner classes).
func (s *Source) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	idx, err := buildCodebaseIndex(s.repoPath, s.sourceRoots())
	if err != nil {
		return nil, fmt.Errorf("buildCodebaseIndex: %w", err)
	}
	s.index = idx
	ids := idx.buildEntityList()
	log.Printf("Discovered %d Java entities from %s", len(ids), s.repoPath)
	return ids, nil
}

// ParseEntity parses a single entity from file content, identified by entityID.
func (s *Source) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	filePath, fragment := splitFragment(entityID)
	if fragment == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Find the target entity by name (supports Outer.Inner notation)
	typeDef := analysis.findType(fragment)
	if typeDef == nil {
		return nil, nil, fmt.Errorf("type %q not found in %s", fragment, filePath)
	}

	rel := relativePath(s.repoPath, filePath)
	url := s.blobURL(rel, content, typeDef.StartPos)

	var description string
	var deprecated bool
	if typeDef.Doc != nil {
		description = typeDef.Doc.Description
		deprecated = typeDef.Doc.Deprecated != ""
	}

	entity := &source.Entity{
		Slug:        slugFromFragment(fragment),
		Name:        typeDef.Name,
		Kind:        typeDef.Kind,
		Description: description,
		SourceFile:  rel,
		SourceCode:  string(content[typeDef.StartPos:typeDef.EndPos]),
		URL:         url,
		Visibility:  typeDef.Visibility,
		Deprecated:  deprecated,
		Properties:  convertFields(typeDef.Fields),
	}

	// Build method URLs for the crawler to follow.
	methodURLs := make([]string, 0, len(typeDef.Methods))
	for _, m := range typeDef.Methods {
		methodID := entityID + "::" + m.Name + m.paramSignature()
		methodURLs = append(methodURLs, methodID)
	}

	return entity, methodURLs, nil
}

// ParseMethod parses a single method/function from file content.
// The methodID has the form: filepath#ClassName::methodName(paramTypes)
func (s *Source) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	filePath, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	// Split fragment into className and method spec.
	classAndMethod := strings.SplitN(fragment, "::", 2)
	if len(classAndMethod) != 2 {
		return nil, fmt.Errorf("invalid method fragment %q: expected ClassName::method(params)", fragment)
	}
	className := classAndMethod[0]
	methodSpec := classAndMethod[1]

	// Extract just the method name (before parentheses).
	methodName := methodSpec
	paramSig := ""
	if idx := strings.Index(methodSpec, "("); idx >= 0 {
		methodName = methodSpec[:idx]
		paramSig = methodSpec[idx:]
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}

	typeDef := analysis.findType(className)
	if typeDef == nil {
		return nil, fmt.Errorf("type %q not found in %s", className, filePath)
	}

	// Find the method, handling overloads via parameter signature.
	var method *javaMethod
	for i := range typeDef.Methods {
		m := &typeDef.Methods[i]
		if m.Name == methodName {
			if paramSig == "" || m.paramSignature() == paramSig {
				method = m
				break
			}
		}
	}
	if method == nil {
		return nil, fmt.Errorf("method %q not found in type %q", methodSpec, className)
	}

	rel := relativePath(s.repoPath, filePath)
	url := s.blobURL(rel, content, method.StartPos)

	parameters := make([]source.Parameter, 0, len(method.Params))
	for _, p := range method.Params {
		parameters = append(parameters, source.Parameter{
			Name:        p.Name,
			Type:        p.Type,
			Required:    !p.HasDefault,
			Description: p.Description,
		})
	}

	// Merge doc params if JavaDoc provides descriptions.
	if method.Doc != nil {
		for i := range parameters {
			for _, dp := range method.Doc.Params {
				if dp.Name == parameters[i].Name && parameters[i].Description == "" {
					parameters[i].Description = dp.Description
					break
				}
			}
		}
	}

	var returnType, returnDesc string
	if method.ReturnType != "" && method.ReturnType != "void" {
		returnType = method.ReturnType
	}
	if method.Doc != nil && method.Doc.Returns != nil {
		returnDesc = method.Doc.Returns.Description
		if returnType == "" {
			returnType = method.Doc.Returns.Type
		}
	}

	relations := buildMethodRelations(method)

	signature := buildMethodSignature(className, method)

	return &source.Method{
		Slug:        strings.ToLower(method.Name),
		Name:        method.Name,
		Signature:   signature,
		Description: methodDescription(method),
		Parameters:  parameters,
		ReturnType:  returnType,
		ReturnDesc:  returnDesc,
		SourceCode:  string(content[method.StartPos:method.EndPos]),
		URL:         url,
		Since:       methodSince(method),
		Deprecated:  methodDeprecated(method),
		Relations:   relations,
	}, nil
}

// DetectWrapper analyzes a method's source code for delegation patterns.
func (s *Source) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil || method.SourceCode == "" {
		return false, "", ""
	}
	return detectWrapper([]byte(method.SourceCode), s.index)
}

// ResolveWrapperURL constructs the identifier to fetch the wrapped target's source.
func (s *Source) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "method":
		// targetName is "ClassName::methodName"
		parts := strings.SplitN(targetName, "::", 2)
		if len(parts) != 2 {
			return ""
		}
		className := parts[0]
		if path, ok := s.index.FileForClass(className); ok {
			return path + "#" + className + "::" + parts[1]
		}
	case "self_method":
		// Delegate to a method on the same class.
		if path, ok := s.index.FileForClass(entitySlug); ok {
			return path + "#" + entitySlug + "::" + targetName
		}
	case "static_method":
		// targetName is "ClassName.methodName"
		parts := strings.SplitN(targetName, ".", 2)
		if len(parts) != 2 {
			return ""
		}
		if path, ok := s.index.FileForClass(parts[0]); ok {
			return path + "#" + parts[0] + "::" + parts[1]
		}
	}
	return ""
}

// ParseSourceCode extracts the source code for a specific entity or method.
func (s *Source) ParseSourceCode(entityID string, content []byte) (string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	analysis := parseFile(content)
	if analysis == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	// Check if this is a method reference (contains ::).
	if className, methodSpec, ok := strings.Cut(fragment, "::"); ok {
		typeDef := analysis.findType(className)
		if typeDef == nil {
			return "", fmt.Errorf("type %q not found", className)
		}
		methodName := methodSpec
		paramSig := ""
		if idx := strings.Index(methodSpec, "("); idx >= 0 {
			methodName = methodSpec[:idx]
			paramSig = methodSpec[idx:]
		}
		for _, m := range typeDef.Methods {
			if m.Name == methodName && (paramSig == "" || m.paramSignature() == paramSig) {
				return string(content[m.StartPos:m.EndPos]), nil
			}
		}
		return "", fmt.Errorf("method %q not found in type %q", methodSpec, className)
	}

	// Otherwise it is a type reference.
	typeDef := analysis.findType(fragment)
	if typeDef == nil {
		return "", fmt.Errorf("type %q not found", fragment)
	}
	return string(content[typeDef.StartPos:typeDef.EndPos]), nil
}

// sourceRoots returns the configured source roots or the default.
func (s *Source) sourceRoots() []string {
	if len(s.config.SourceRoots) > 0 {
		return s.config.SourceRoots
	}
	return []string{"src/main/java"}
}

// blobURL builds a GitHub blob URL for a relative path and optional line number.
func (s *Source) blobURL(rel string, content []byte, pos int) string {
	ref := s.ref
	if ref == "" {
		ref = "main"
	}
	url := fmt.Sprintf("%s/blob/%s/%s", s.config.SourceURL, ref, rel)
	if ln := lineNumber(content, pos); ln > 0 {
		url += fmt.Sprintf("#L%d", ln)
	}
	return url
}

// splitFragment splits an entityID at "#".
func splitFragment(id string) (base, fragment string) {
	base, fragment, _ = strings.Cut(id, "#")
	return base, fragment
}

// slugFromFragment converts a type name fragment into a URL slug.
func slugFromFragment(fragment string) string {
	return strings.ToLower(strings.ReplaceAll(fragment, ".", "-"))
}

// relativePath returns absPath relative to repoPath.
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
	if pos < 0 || pos > len(content) {
		return 0
	}
	count := 1
	for i := 0; i < pos; i++ {
		if content[i] == '\n' {
			count++
		}
	}
	return count
}

// convertFields converts javaField slice to source.Property slice.
func convertFields(fields []javaField) []source.Property {
	if len(fields) == 0 {
		return nil
	}
	props := make([]source.Property, 0, len(fields))
	for _, f := range fields {
		desc := ""
		if f.Doc != nil {
			desc = f.Doc.Description
		}
		props = append(props, source.Property{
			Name:        f.Name,
			Type:        f.Type,
			Description: desc,
			Visibility:  f.Visibility,
		})
	}
	return props
}

// buildMethodRelations creates relation entries for a method.
func buildMethodRelations(m *javaMethod) []source.Relation {
	var relations []source.Relation
	if m.Doc != nil {
		for _, see := range m.Doc.See {
			relations = append(relations, source.Relation{
				Kind:       "uses",
				TargetName: see,
			})
		}
		for _, t := range m.Doc.Throws {
			relations = append(relations, source.Relation{
				Kind:        "throws",
				TargetName:  t.Type,
				Description: t.Description,
			})
		}
	}
	for _, thrown := range m.Throws {
		// Avoid duplicates from JavaDoc.
		found := false
		for _, r := range relations {
			if r.Kind == "throws" && r.TargetName == thrown {
				found = true
				break
			}
		}
		if !found {
			relations = append(relations, source.Relation{
				Kind:       "throws",
				TargetName: thrown,
			})
		}
	}
	return relations
}

// buildMethodSignature constructs a Java method signature string.
func buildMethodSignature(className string, m *javaMethod) string {
	var b strings.Builder
	if m.Visibility != "" && m.Visibility != "public" {
		b.WriteString(m.Visibility)
		b.WriteString(" ")
	}
	if m.Static {
		b.WriteString("static ")
	}
	if m.TypeParams != "" {
		b.WriteString(m.TypeParams)
		b.WriteString(" ")
	}
	if m.ReturnType != "" {
		b.WriteString(m.ReturnType)
		b.WriteString(" ")
	}
	b.WriteString(className)
	b.WriteString("::")
	b.WriteString(m.Name)
	b.WriteString("(")
	for i, p := range m.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Type)
		b.WriteString(" ")
		b.WriteString(p.Name)
	}
	b.WriteString(")")
	if len(m.Throws) > 0 {
		b.WriteString(" throws ")
		b.WriteString(strings.Join(m.Throws, ", "))
	}
	return b.String()
}

// methodDescription extracts the description from a method's doc.
func methodDescription(m *javaMethod) string {
	if m.Doc != nil {
		return m.Doc.Description
	}
	return ""
}

// methodSince extracts the @since tag from a method's doc.
func methodSince(m *javaMethod) string {
	if m.Doc != nil {
		return m.Doc.Since
	}
	return ""
}

// methodDeprecated checks if the method is deprecated.
func methodDeprecated(m *javaMethod) bool {
	if m.Deprecated {
		return true
	}
	if m.Doc != nil && m.Doc.Deprecated != "" {
		return true
	}
	return false
}
