package source

import "context"

// Source defines the contract for a documentation source adapter.
// Each documentation framework (WordPress, Laravel, React, etc.)
// implements this interface. The crawler uses it generically.
type Source interface {
	// ID returns the canonical library ID (e.g., "/wordpress/classes").
	ID() string

	// Meta returns metadata for the library record.
	Meta() LibraryMeta

	// DiscoverEntities returns URLs of all top-level entities to crawl.
	DiscoverEntities(ctx context.Context, fetch FetchFunc) ([]string, error)

	// ParseEntity parses a single entity page and returns its data
	// plus a list of method URLs to crawl next.
	ParseEntity(ctx context.Context, url string, body []byte) (*Entity, []string, error)

	// ParseMethod parses a single method/function detail page.
	ParseMethod(ctx context.Context, url string, body []byte) (*Method, error)

	// DetectWrapper analyzes a method's source code and returns wrapper info.
	// Returns (isWrapper, targetName, targetKind) where targetKind is
	// "function", "self_method", or "static_method".
	DetectWrapper(method *Method) (bool, string, string)

	// ResolveWrapperURL constructs the URL to fetch the wrapped method's page.
	ResolveWrapperURL(targetName, targetKind, entitySlug string) string

	// ParseSourceCode extracts just the source code from a page body.
	ParseSourceCode(body []byte) (string, error)
}

// FetchFunc is a rate-limited HTTP fetch function provided by the crawler.
type FetchFunc func(ctx context.Context, url string) ([]byte, error)

// LibraryMeta holds metadata for library registration.
type LibraryMeta struct {
	Name        string
	Description string
	SourceURL   string
	Version     string
	TrustScore  float64
}

// Entity represents a top-level documentation entity (class, module, etc.)
type Entity struct {
	Slug        string
	Name        string
	Kind        string // "class", "interface", "trait", "module"
	Description string
	SourceFile  string
	SourceCode  string
	URL         string
	Properties  []Property
}

// Property represents a class/entity property.
type Property struct {
	Name        string
	Type        string
	Description string
	Visibility  string
	Since       string
}

// Method represents a method/function with full details.
type Method struct {
	Slug          string
	Name          string
	Signature     string
	Description   string
	Parameters    []Parameter
	ReturnType    string
	ReturnDesc    string
	SourceCode    string
	WrappedSource string
	WrappedMethod string
	URL           string
	Since         string
	Relations     []Relation
}

// Parameter describes a function/method parameter.
type Parameter struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// Relation is a cross-reference to another method/function.
type Relation struct {
	Kind        string // "uses", "used_by"
	TargetName  string // "WP_Query::get_posts()"
	TargetURL   string
	Description string
}
