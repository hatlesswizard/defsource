// Package php provides a documentation source adapter that reads
// WordPress PHP source code from a local clone of the WordPress/WordPress
// GitHub repository. It parses raw PHP source via tree-sitter to extract
// classes, functions, methods, properties, and PHPDoc without making any
// network calls during parsing.
package php

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/source"
)

// defaultBlobRef is the git ref used to build source-code links when the
// source was constructed without an explicit version (e.g. fixture tests).
const defaultBlobRef = "master"

// PHPSource is a documentation source adapter that reads WordPress
// PHP source code from a local clone of the WordPress/WordPress GitHub
// repository. Unlike the HTML-scraping wordpress adapter, this one parses
// raw PHP source, which allows direct access to source code, PHPDoc, and
// wrapper detection without making any network calls during parsing.
type PHPSource struct {
	repoPath string
	ref      string // git tag/branch used for source-code links; "" => defaultBlobRef
	index    *codebaseIndex
	config   *PHPConfig // nil means WordPress defaults
}

// PHPConfig holds configuration for a generic PHP library source adapter.
// When nil/unset, the adapter defaults to WordPress behaviour.
type PHPConfig struct {
	// LibraryID is the canonical library ID (e.g., "php/laravel").
	LibraryID string

	// Name is the human-readable library name.
	Name string

	// Description is a short description of the library.
	Description string

	// SourceURL is the upstream repository URL.
	SourceURL string

	// SourceDirs restricts discovery to these subdirectories. If empty,
	// the entire repo is walked for PHP files.
	SourceDirs []string
}

// WithConfig sets a PHPConfig for non-WordPress PHP libraries.
func WithConfig(cfg PHPConfig) Option {
	return func(s *PHPSource) { s.config = &cfg }
}

var _ source.Source = (*PHPSource)(nil)

// Option customises a PHPSource at construction time.
type Option func(*PHPSource)

// WithRef sets the git ref (tag or branch) that source-code links point at,
// e.g. WithRef("6.5.3") produces links under .../blob/6.5.3/. When unset,
// links fall back to defaultBlobRef.
func WithRef(ref string) Option {
	return func(s *PHPSource) { s.ref = ref }
}

// New constructs a new PHPSource pointing at a local clone of the
// WordPress/WordPress repository. The index is initialised to an empty
// (non-nil) state so that DetectWrapper and ResolveWrapperURL are safe to
// call before DiscoverEntities has been run.
func New(repoPath string, opts ...Option) *PHPSource {
	s := &PHPSource{
		repoPath: repoPath,
		index:    emptyIndex(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// blobBase returns the GitHub blob URL prefix for source-code links, pinned
// to the source's ref (or defaultBlobRef when no ref was supplied).
func (s *PHPSource) blobBase() string {
	ref := s.ref
	if ref == "" {
		ref = defaultBlobRef
	}
	baseURL := "https://github.com/WordPress/WordPress"
	if s.config != nil && s.config.SourceURL != "" {
		baseURL = strings.TrimSuffix(s.config.SourceURL, "/")
	}
	return baseURL + "/blob/" + ref + "/"
}

// ID returns the canonical library ID.
func (s *PHPSource) ID() string {
	if s.config != nil {
		return s.config.LibraryID
	}
	return "/wordpress"
}

// Meta returns metadata for the library record.
func (s *PHPSource) Meta() source.LibraryMeta {
	if s.config != nil {
		return source.LibraryMeta{
			Name:        s.config.Name,
			Description: s.config.Description,
			SourceURL:   s.config.SourceURL,
			Version:     s.ref,
			Language:    "php",
			TrustScore:  0.90,
		}
	}
	return source.LibraryMeta{
		Name:        "WordPress Reference",
		Description: "Complete reference for WordPress PHP classes and functions",
		SourceURL:   "https://github.com/WordPress/WordPress",
		Version:     detectVersion(s.repoPath),
		TrustScore:  0.95,
	}
}

// DiscoverEntities walks the local repository and returns a sorted list
// of entity identifiers. The fetch parameter is unused — all discovery
// reads files locally.
func (s *PHPSource) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	var dirs []string
	if s.config != nil {
		dirs = s.config.SourceDirs
	}
	idx, err := buildCodebaseIndex(s.repoPath, dirs)
	if err != nil {
		return nil, fmt.Errorf("buildCodebaseIndex: %w", err)
	}
	s.index = idx
	ids := idx.buildEntityList()
	log.Printf("Discovered %d entities from %s", len(ids), s.repoPath)
	return ids, nil
}

// ParseEntity dispatches to either function-entity or class-entity parsing
// based on whether the URL contains a fragment.
func (s *PHPSource) ParseEntity(ctx context.Context, url string, body []byte) (*source.Entity, []string, error) {
	if strings.Contains(url, "#") {
		return s.parseFunctionEntity(body, url)
	}
	return s.parseClassEntity(body, url)
}

// ParseMethod parses a class method given a URL fragment of the form
// `path/to/file.php#ClassName::methodName`.
func (s *PHPSource) ParseMethod(ctx context.Context, url string, body []byte) (*source.Method, error) {
	fileURL, fragment := splitFragment(url)
	if fragment == "" {
		return nil, fmt.Errorf("no fragment in url: %s", url)
	}

	parts := strings.SplitN(fragment, "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid method fragment %q: expected ClassName::methodName", fragment)
	}
	className, methodName := parts[0], parts[1]

	analysis := parseFile(body)
	if analysis == nil {
		return nil, fmt.Errorf("class %q not found in %s", className, fileURL)
	}

	var cls *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].Name == className {
			cls = &analysis.Classes[i]
			break
		}
	}
	if cls == nil {
		return nil, fmt.Errorf("class %q not found in %s", className, fileURL)
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

	doc := parsePhpDoc(method.DocComment)

	rel := relativePath(s.repoPath, fileURL)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(body, method.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	parameters := make([]source.Parameter, 0, len(doc.Params))
	for _, p := range doc.Params {
		name := strings.TrimPrefix(p.Name, "$")
		parameters = append(parameters, source.Parameter{
			Name:        name,
			Type:        p.Type,
			Required:    !p.Optional,
			Description: p.Desc,
		})
	}

	signature := buildMethodSignature(className, method, doc)

	relations := make([]source.Relation, 0, len(doc.See)+len(doc.Uses))
	for _, see := range doc.See {
		relations = append(relations, source.Relation{
			Kind:       "uses",
			TargetName: see,
		})
	}
	for _, uses := range doc.Uses {
		relations = append(relations, source.Relation{
			Kind:       "uses",
			TargetName: uses,
		})
	}

	return &source.Method{
		Slug:        strings.ToLower(method.Name),
		Name:        method.Name,
		Signature:   signature,
		Description: doc.Description,
		Parameters:  parameters,
		ReturnType:  doc.Return.Type,
		ReturnDesc:  doc.Return.Desc,
		SourceCode:  string(body[method.StartPos:method.EndPos]),
		URL:         githubURL,
		Since:       doc.Since,
		Relations:   relations,
	}, nil
}

// DetectWrapper analyzes a method's source code for wrapper patterns.
// It is safe to call before DiscoverEntities — it will return (false,"","")
// when the index has no data (i.e. no builtins have been detected yet).
func (s *PHPSource) DetectWrapper(method *source.Method) (bool, string, string) {
	if method == nil {
		return false, "", ""
	}
	return detectWrapperAST([]byte(method.SourceCode), s.index.builtinFunctions)
}

// ResolveWrapperURL constructs a URL identifier for a wrapped target so
// the crawler can fetch its source. Returns an empty string when no file
// in the index hosts the requested target. It is safe to call before
// DiscoverEntities — an empty index simply finds no matches.
func (s *PHPSource) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "function":
		if path, ok := s.index.FileForFunction(targetName); ok {
			return path + "#" + targetName
		}
	case "self_method":
		if path, canonical, ok := s.index.LookupClass(entitySlug); ok {
			return path + "#" + canonical + "::" + targetName
		}
	case "static_method":
		className, methodName, found := strings.Cut(targetName, "::")
		if !found {
			return ""
		}
		if path, ok := s.index.FileForClass(className); ok {
			return path + "#" + className + "::" + methodName
		}
	}
	return ""
}

// ParseSourceCode extracts the source code for a specific function or
// method from a PHP file, identified by the URL fragment.
func (s *PHPSource) ParseSourceCode(url string, body []byte) (string, error) {
	_, fragment := splitFragment(url)
	if fragment == "" {
		return string(body), nil
	}

	analysis := parseFile(body)
	if analysis == nil {
		return "", fmt.Errorf("failed to parse source")
	}

	if className, methodName, ok := strings.Cut(fragment, "::"); ok {
		var cls *classDef
		for i := range analysis.Classes {
			if analysis.Classes[i].Name == className {
				cls = &analysis.Classes[i]
				break
			}
		}
		if cls == nil {
			return "", fmt.Errorf("class %q not found", className)
		}

		for _, m := range cls.Methods {
			if m.Name == methodName {
				return string(body[m.StartPos:m.EndPos]), nil
			}
		}
		return "", fmt.Errorf("method %q not found in class %q", methodName, className)
	}

	for _, fn := range analysis.Functions {
		if fn.Name == fragment {
			return string(body[fn.StartPos:fn.EndPos]), nil
		}
	}
	return "", fmt.Errorf("function %q not found", fragment)
}

// parseClassEntity parses a PHP file that contains a class definition and
// returns the corresponding Entity along with a method URL for each class method.
func (s *PHPSource) parseClassEntity(body []byte, url string) (*source.Entity, []string, error) {
	analysis := parseFile(body)
	if analysis == nil || len(analysis.Classes) == 0 {
		return nil, nil, fmt.Errorf("no classes found in %s", url)
	}

	cls := analysis.Classes[0]
	doc := parsePhpDoc(cls.DocComment)

	rel := relativePath(s.repoPath, url)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(body, cls.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	properties := make([]source.Property, 0, len(cls.Properties))
	for _, p := range cls.Properties {
		pdoc := parsePhpDoc(p.DocComment)
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
		Kind:        "class",
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  string(body[cls.StartPos:cls.EndPos]),
		URL:         githubURL,
		Properties:  properties,
	}

	methodURLs := make([]string, 0, len(cls.Methods))
	for _, m := range cls.Methods {
		methodURLs = append(methodURLs, url+"#"+cls.Name+"::"+m.Name)
	}

	return entity, methodURLs, nil
}

// parseFunctionEntity parses a standalone PHP function from a file, located
// by the function name in the URL fragment, and returns the corresponding Entity.
func (s *PHPSource) parseFunctionEntity(body []byte, url string) (*source.Entity, []string, error) {
	fileURL, funcName := splitFragment(url)
	if funcName == "" {
		return nil, nil, fmt.Errorf("no fragment in url: %s", url)
	}

	analysis := parseFile(body)
	if analysis == nil {
		return nil, nil, fmt.Errorf("function %q not found in %s", funcName, fileURL)
	}

	var found *functionDef
	for i := range analysis.Functions {
		if analysis.Functions[i].Name == funcName {
			found = &analysis.Functions[i]
			break
		}
	}
	if found == nil {
		return nil, nil, fmt.Errorf("function %q not found in %s", funcName, fileURL)
	}

	doc := parsePhpDoc(found.DocComment)

	rel := relativePath(s.repoPath, fileURL)
	githubURL := s.blobBase() + rel
	if ln := lineNumber(body, found.StartPos); ln > 0 {
		githubURL += fmt.Sprintf("#L%d", ln)
	}

	entity := &source.Entity{
		Slug:        funcName,
		Name:        funcName,
		Kind:        "function",
		Description: doc.Description,
		SourceFile:  rel,
		SourceCode:  string(body[found.StartPos:found.EndPos]),
		URL:         githubURL,
		Properties:  nil,
	}

	return entity, nil, nil
}

// splitFragment returns the URL up to and excluding "#", and the fragment
// after "#". If the URL contains no "#", fragment is "".
func splitFragment(url string) (base, fragment string) {
	base, fragment, _ = strings.Cut(url, "#")
	return base, fragment
}

var reWPVersion = regexp.MustCompile(`\$wp_version\s*=\s*'([^']+)'`)

// detectVersion reads wp-includes/version.php and extracts the WordPress
// version string. Returns "unknown" if it cannot be determined.
func detectVersion(repoPath string) string {
	if repoPath == "" {
		return "unknown"
	}
	versionFile := filepath.Join(repoPath, "wp-includes", "version.php")
	content, err := os.ReadFile(versionFile)
	if err != nil {
		return "unknown"
	}
	if m := reWPVersion.FindSubmatch(content); m != nil {
		return string(m[1])
	}
	return "unknown"
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

// lineNumber returns the 1-indexed line number for the given byte offset
// within content.
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

// lookupClassFile performs a case-insensitive lookup against classIndex,
// since WordPress class names mix conventions (`wpdb` vs `WP_Query`).
// Deprecated: use (*codebaseIndex).LookupClass instead.
func lookupClassFile(classIndex map[string]string, name string) (string, bool) {
	if path, ok := classIndex[name]; ok {
		return path, true
	}
	lower := strings.ToLower(name)
	for k, v := range classIndex {
		if strings.ToLower(k) == lower {
			return v, true
		}
	}
	return "", false
}

// canonicalClassName returns the original cased class name from the index
// for a given (possibly lowercased) lookup key.
// Deprecated: use (*codebaseIndex).LookupClass instead.
func canonicalClassName(classIndex map[string]string, name string) string {
	if _, ok := classIndex[name]; ok {
		return name
	}
	lower := strings.ToLower(name)
	for k := range classIndex {
		if strings.ToLower(k) == lower {
			return k
		}
	}
	return name
}

// buildMethodSignature constructs a WordPress-style method signature using
// PHPDoc parameter types when available.
func buildMethodSignature(className string, method *methodDef, doc phpDoc) string {
	rawSig := strings.TrimPrefix(method.Signature, "function ")

	openIdx := strings.Index(rawSig, "(")
	closeIdx := strings.LastIndex(rawSig, ")")
	if openIdx < 0 || closeIdx < 0 || closeIdx <= openIdx {
		return className + "::" + rawSig
	}

	tail := rawSig[closeIdx+1:]
	paramsBlob := strings.TrimSpace(rawSig[openIdx+1 : closeIdx])

	var enriched []string
	if paramsBlob != "" {
		typeByName := map[string]string{}
		for _, p := range doc.Params {
			typeByName[p.Name] = p.Type
		}
		for _, raw := range splitTopLevel(paramsBlob, ',') {
			enriched = append(enriched, enrichParam(strings.TrimSpace(raw), typeByName))
		}
	}

	body := strings.Join(enriched, ", ")
	if body != "" {
		body = " " + body + " "
	}

	return className + "::" + method.Name + "(" + body + ")" + tail
}

// enrichParam returns a parameter declaration with a PHPDoc-supplied type
// prepended when the original declaration omits one.
func enrichParam(decl string, typeByName map[string]string) string {
	if decl == "" {
		return decl
	}
	dollar := strings.Index(decl, "$")
	if dollar < 0 {
		return decl
	}
	prefix := strings.TrimSpace(decl[:dollar])
	rest := decl[dollar:]
	nameEnd := len(rest)
	for i, r := range rest {
		if i == 0 {
			continue
		}
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			nameEnd = i
			break
		}
	}
	name := rest[:nameEnd]
	if prefix == "" {
		if t, ok := typeByName[name]; ok && t != "" {
			return t + " " + rest
		}
	}
	return decl
}

// splitTopLevel splits a string on sep, ignoring sep that appears inside
// matched brackets, parentheses, or quotes. Used for splitting parameter
// lists that may contain default-value expressions.
func splitTopLevel(s string, sep byte) []string {
	var result []string
	depth := 0
	last := 0
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\\' && i+1 < len(s) {
				i++ // skip escaped character
				continue
			}
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			if c == '\\' && i+1 < len(s) {
				i++ // skip escaped character
				continue
			}
			if c == '"' {
				inDouble = false
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == sep && depth == 0:
			result = append(result, s[last:i])
			last = i + 1
		}
	}
	result = append(result, s[last:])
	return result
}
