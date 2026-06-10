// Package csharp provides a documentation source adapter that reads C# source
// code from a local clone of a .NET/ASP.NET/EF Core GitHub repository. It
// parses raw .cs source via tree-sitter to extract classes, interfaces, structs,
// records, enums, delegates, methods, properties, and XML doc comments without
// making any network calls during parsing.
package csharp

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hatlesswizard/defsource/internal/source"
)

// Config holds settings for a C# source adapter. It is configurable for any
// C# library (.NET runtime, ASP.NET Core, EF Core, etc.).
type Config struct {
	// LibraryID is the canonical library ID (e.g., "csharp/aspnetcore").
	LibraryID string

	// Name is the human-readable library name.
	Name string

	// Description is a brief description of the library.
	Description string

	// SourceURL is the canonical GitHub URL of the repository.
	SourceURL string

	// SourceDirs lists relative directories within the repo to scan.
	// If empty, the entire repo is scanned (excluding test/obj/bin dirs).
	SourceDirs []string
}

// CSharpSource is a documentation source adapter that reads C# source code
// from a local repository clone. It parses raw .cs source using tree-sitter
// to extract all public type declarations and their members.
type CSharpSource struct {
	repoPath string
	ref      string
	config   Config
	index    *codebaseIndex
}

var _ source.Source = (*CSharpSource)(nil)

// Option customises a CSharpSource at construction time.
type Option func(*CSharpSource)

// WithRef sets the git ref (tag or branch) for source-code links.
func WithRef(ref string) Option {
	return func(s *CSharpSource) { s.ref = ref }
}

// New constructs a new CSharpSource pointing at a local clone of a C# repo.
func New(repoPath string, cfg Config, opts ...Option) *CSharpSource {
	s := &CSharpSource{
		repoPath: repoPath,
		config:   cfg,
		index:    emptyIndex(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ID returns the canonical library ID (e.g., "csharp/aspnetcore").
func (s *CSharpSource) ID() string { return s.config.LibraryID }

// Meta returns metadata for the library record.
func (s *CSharpSource) Meta() source.LibraryMeta {
	return source.LibraryMeta{
		Name:        s.config.Name,
		Description: s.config.Description,
		SourceURL:   s.config.SourceURL,
		Version:     s.ref,
		Language:    "csharp",
		TrustScore:  0.90,
	}
}

// blobBase returns the GitHub blob URL prefix for source-code links.
func (s *CSharpSource) blobBase() string {
	ref := s.ref
	if ref == "" {
		ref = "main"
	}
	return strings.TrimRight(s.config.SourceURL, "/") + "/blob/" + ref + "/"
}

// DiscoverEntities walks the local repository and returns identifiers for all
// public type declarations. The fetch parameter is unused for local sources.
func (s *CSharpSource) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	idx, err := buildCodebaseIndex(s.repoPath, s.config.SourceDirs)
	if err != nil {
		return nil, fmt.Errorf("buildCodebaseIndex: %w", err)
	}
	s.index = idx
	ids := idx.buildEntityList()
	log.Printf("Discovered %d C# entities from %s", len(ids), s.repoPath)
	return ids, nil
}

// ParseEntity parses a single entity from file content, identified by entityID.
// The entityID has the form "filepath#Namespace.TypeName".
func (s *CSharpSource) ParseEntity(ctx context.Context, entityID string, content []byte) (*source.Entity, []string, error) {
	filePath, fragment := splitFragment(entityID)
	if fragment == "" {
		return nil, nil, fmt.Errorf("no fragment in entityID: %s", entityID)
	}

	analysis := parseFile(content)
	if analysis == nil {
		return nil, nil, fmt.Errorf("failed to parse %s", filePath)
	}

	// Find the target type by qualified name
	typeDef := analysis.findType(fragment)
	if typeDef == nil {
		return nil, nil, fmt.Errorf("type %q not found in %s", fragment, filePath)
	}

	// Build entity
	entity := &source.Entity{
		Slug:       slugFromFragment(fragment),
		Name:       typeDef.Name,
		Kind:       typeDef.Kind,
		SourceFile: relativePath(s.repoPath, filePath),
		SourceCode: string(content[typeDef.StartPos:typeDef.EndPos]),
		URL:        s.buildURL(filePath, typeDef.StartPos, content),
		Visibility: typeDef.Visibility,
		Deprecated: typeDef.Deprecated,
	}

	// Parse XML doc for description
	if typeDef.DocComment != "" {
		doc := parseXMLDoc(typeDef.DocComment)
		entity.Description = doc.Summary
	}

	// Build properties list
	for _, prop := range typeDef.Properties {
		p := source.Property{
			Name:       prop.Name,
			Type:       prop.Type,
			Visibility: prop.Visibility,
		}
		if prop.DocComment != "" {
			doc := parseXMLDoc(prop.DocComment)
			p.Description = doc.Summary
		}
		entity.Properties = append(entity.Properties, p)
	}

	// Build method identifiers for crawling
	var methodIDs []string
	for _, m := range typeDef.Methods {
		if m.Visibility != "public" && m.Visibility != "protected" {
			continue
		}
		methodID := filePath + "#" + fragment + "." + m.Name
		methodIDs = append(methodIDs, methodID)
	}

	return entity, methodIDs, nil
}

// ParseMethod parses a single method from file content. The methodID has the
// form "filepath#Namespace.TypeName.MethodName".
func (s *CSharpSource) ParseMethod(ctx context.Context, methodID string, content []byte) (*source.Method, error) {
	filePath, fragment := splitFragment(methodID)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in methodID: %s", methodID)
	}

	// Split fragment into type-qualified-name and method name
	lastDot := strings.LastIndex(fragment, ".")
	if lastDot < 0 {
		return nil, fmt.Errorf("invalid method fragment %q: expected Namespace.Type.Method", fragment)
	}
	typeQualified := fragment[:lastDot]
	methodName := fragment[lastDot+1:]

	analysis := parseFile(content)
	if analysis == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}

	typeDef := analysis.findType(typeQualified)
	if typeDef == nil {
		return nil, fmt.Errorf("type %q not found in %s", typeQualified, filePath)
	}

	var method *methodDef
	for i := range typeDef.Methods {
		if typeDef.Methods[i].Name == methodName {
			method = &typeDef.Methods[i]
			break
		}
	}
	if method == nil {
		return nil, fmt.Errorf("method %q not found in type %q", methodName, typeQualified)
	}

	// Parse XML doc
	var description string
	var parameters []source.Parameter
	var returnType, returnDesc string
	var since string
	var deprecated bool
	var relations []source.Relation

	if method.DocComment != "" {
		doc := parseXMLDoc(method.DocComment)
		description = doc.Summary

		for _, p := range doc.Params {
			parameters = append(parameters, source.Parameter{
				Name:        p.Name,
				Type:        findParamType(method.Params, p.Name),
				Required:    !isOptionalParam(method.Params, p.Name),
				Description: p.Description,
			})
		}

		if doc.Returns != nil {
			returnDesc = doc.Returns.Description
		}

		for _, see := range doc.See {
			relations = append(relations, source.Relation{
				Kind:       "uses",
				TargetName: see,
			})
		}

		since = doc.Since
		deprecated = doc.Deprecated != ""
	}

	// If no doc params, build from signature
	if len(parameters) == 0 {
		for _, p := range method.Params {
			parameters = append(parameters, source.Parameter{
				Name:     p.Name,
				Type:     p.Type,
				Required: !p.HasDefault,
			})
		}
	}

	if returnType == "" {
		returnType = method.ReturnType
	}

	rel := relativePath(s.repoPath, filePath)
	url := s.blobBase() + rel
	if ln := lineNumber(content, method.StartPos); ln > 0 {
		url += fmt.Sprintf("#L%d", ln)
	}

	return &source.Method{
		Slug:        strings.ToLower(methodName),
		Name:        methodName,
		Signature:   method.Signature,
		Description: description,
		Parameters:  parameters,
		ReturnType:  returnType,
		ReturnDesc:  returnDesc,
		SourceCode:  string(content[method.StartPos:method.EndPos]),
		URL:         url,
		Since:       since,
		Deprecated:  deprecated,
		Relations:   relations,
	}, nil
}

// DetectWrapper analyzes a method's source code for delegation patterns.
func (s *CSharpSource) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil || method.SourceCode == "" {
		return false, "", ""
	}
	return detectWrapper(method.SourceCode, s.index)
}

// ResolveWrapperURL constructs the identifier to fetch a wrapped target's source.
func (s *CSharpSource) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "method":
		// targetName is "TypeName.MethodName"
		parts := strings.SplitN(targetName, ".", 2)
		if len(parts) != 2 {
			return ""
		}
		typeName := parts[0]
		if path, ok := s.index.FileForType(typeName); ok {
			ns := s.index.NamespaceForType(typeName)
			qualified := ns + "." + typeName
			return path + "#" + qualified + "." + parts[1]
		}
	case "function":
		// Static method call: targetName is "TypeName.MethodName"
		parts := strings.SplitN(targetName, ".", 2)
		if len(parts) != 2 {
			return ""
		}
		typeName := parts[0]
		if path, ok := s.index.FileForType(typeName); ok {
			ns := s.index.NamespaceForType(typeName)
			qualified := ns + "." + typeName
			return path + "#" + qualified + "." + parts[1]
		}
	case "self_method":
		// Same-type method: look up entitySlug type
		if path, ok := s.index.FileForType(entitySlug); ok {
			ns := s.index.NamespaceForType(entitySlug)
			qualified := ns + "." + entitySlug
			return path + "#" + qualified + "." + targetName
		}
	}
	return ""
}

// ParseSourceCode extracts just the source code of a specific entity or method.
func (s *CSharpSource) ParseSourceCode(entityID string, content []byte) (string, error) {
	_, fragment := splitFragment(entityID)
	if fragment == "" {
		return string(content), nil
	}

	analysis := parseFile(content)
	if analysis == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	// Check if it's a method (has three+ dot-separated segments after the type)
	lastDot := strings.LastIndex(fragment, ".")
	if lastDot < 0 {
		return "", fmt.Errorf("invalid fragment: %s", fragment)
	}

	// Try as a type first
	typeDef := analysis.findType(fragment)
	if typeDef != nil {
		return string(content[typeDef.StartPos:typeDef.EndPos]), nil
	}

	// Try as a method
	typeQualified := fragment[:lastDot]
	methodName := fragment[lastDot+1:]
	typeDef = analysis.findType(typeQualified)
	if typeDef == nil {
		return "", fmt.Errorf("type %q not found", typeQualified)
	}
	for _, m := range typeDef.Methods {
		if m.Name == methodName {
			return string(content[m.StartPos:m.EndPos]), nil
		}
	}
	return "", fmt.Errorf("method %q not found in type %q", methodName, typeQualified)
}

// --- helper functions ---

// splitFragment splits an entityID into file path and fragment.
func splitFragment(id string) (string, string) {
	if i := strings.LastIndex(id, "#"); i >= 0 {
		return id[:i], id[i+1:]
	}
	return id, ""
}

// slugFromFragment derives a URL-safe slug from a qualified name.
func slugFromFragment(fragment string) string {
	return strings.ToLower(strings.ReplaceAll(fragment, ".", "-"))
}

// relativePath computes the relative path from repoPath to filePath.
func relativePath(repoPath, filePath string) string {
	rel := strings.TrimPrefix(filePath, repoPath)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "\\")
	return strings.ReplaceAll(rel, "\\", "/")
}

// buildURL constructs a GitHub source URL with line number.
func (s *CSharpSource) buildURL(filePath string, startPos int, content []byte) string {
	rel := relativePath(s.repoPath, filePath)
	url := s.blobBase() + rel
	if ln := lineNumber(content, startPos); ln > 0 {
		url += fmt.Sprintf("#L%d", ln)
	}
	return url
}

// lineNumber returns the 1-based line number for a byte offset.
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

// findParamType looks up the type of a parameter by name in the method's param list.
func findParamType(params []paramDef, name string) string {
	for _, p := range params {
		if p.Name == name {
			return p.Type
		}
	}
	return ""
}

// isOptionalParam checks if a parameter has a default value.
func isOptionalParam(params []paramDef, name string) bool {
	for _, p := range params {
		if p.Name == name {
			return p.HasDefault
		}
	}
	return false
}
