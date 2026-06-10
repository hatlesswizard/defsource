// Package source defines the Source interface and the data-transfer types that
// flow between a source adapter and the crawler. Each documentation source
// (PHP/WordPress, Go/stdlib, Python/Django, etc.) provides a concrete Source
// implementation; the crawler consumes the interface without knowing which
// language or framework is in use.
//
// Data types in this package (LibraryMeta, Entity, Property, Method, Parameter,
// Relation, DocSnippet) are intentionally plain DTOs — all exported fields, no
// methods — because they serve as data carriers between the parser, the store
// layer, and the public query client.
//
// # Canonical shared types
//
// Parameter and Relation are the canonical definitions of those concepts across
// the entire codebase. The public defsource package re-exports them as type
// aliases (type Parameter = source.Parameter) so callers using defsource.Parameter
// continue to work without any change.
//
// DocSnippet is the canonical type for a formatted documentation snippet returned
// by QueryDocs. The internal/search package accepts []DocSnippet directly for
// formatting, eliminating the former search.Snippet / search.SnippetParam
// parallel types and the manual field-copy loop that connected them.
package source

import "context"

// Source defines the contract for a documentation source adapter.
// Each documentation source (PHP/WordPress, Go/stdlib, Python/Django, etc.)
// implements this interface. The crawler uses it generically.
//
// # Terminology
//
//   - "entityID": A string that uniquely identifies an entity within its source.
//     For local-source parsers this is typically an absolute filesystem path with
//     an optional fragment: "/path/to/file.ext#EntityName". The fragment identifies
//     a specific entity within a multi-entity file.
//   - "content": The raw bytes of the file identified by the entityID (with fragment
//     stripped). The Fetcher reads the file path; the Source parses the content.
//
// # Lifecycle
//
//  1. ID and Meta may be called at any time after construction.
//  2. DiscoverEntities must be called before DetectWrapper, ResolveWrapperURL,
//     or ParseSourceCode.
//  3. ParseEntity and ParseMethod may be called concurrently after DiscoverEntities.
//  4. DetectWrapper, ResolveWrapperURL, and ParseSourceCode are called during
//     wrapper chain resolution, concurrently after DiscoverEntities.
//
// Implementations are responsible for thread safety on all methods that are
// called concurrently (steps 3 and 4 above).
type Source interface {
	// ID returns the canonical library ID (e.g., "/wordpress", "go/stdlib").
	ID() string

	// Meta returns metadata for the library record.
	Meta() LibraryMeta

	// DiscoverEntities returns identifiers for all top-level entities to crawl.
	// Each identifier is an absolute file path optionally suffixed with a fragment
	// (#EntityName) that uniquely identifies one entity in a multi-entity file.
	// The fetch function is provided by the crawler for sources that need to
	// read remote resources during discovery (most local sources ignore it).
	DiscoverEntities(ctx context.Context, fetch FetchFunc) ([]string, error)

	// ParseEntity parses a single entity from file content, identified by entityID.
	// Returns the entity data plus a list of method identifiers to crawl next.
	// For multi-entity files, the implementation uses the fragment in entityID to
	// locate the specific entity within the content bytes.
	ParseEntity(ctx context.Context, entityID string, content []byte) (*Entity, []string, error)

	// ParseMethod parses a single method/function from file content.
	// The methodID contains a fragment identifying the specific method.
	ParseMethod(ctx context.Context, methodID string, content []byte) (*Method, error)

	// DetectWrapper analyzes a method's source code and returns wrapper info.
	// Returns (isWrapper, targetName, targetKind) where targetKind is a
	// language-specific delegation category (e.g., "function", "self_method",
	// "static_method" for PHP; "method", "function" for Go).
	// Returns (false, "", "") if the method is not a wrapper.
	DetectWrapper(method *Method) (bool, string, string)

	// ResolveWrapperURL constructs the identifier to fetch the wrapped target's
	// source. Returns an empty string if the target cannot be resolved.
	ResolveWrapperURL(targetName, targetKind, entitySlug string) string

	// ParseSourceCode extracts just the source code of a specific entity or
	// method from file content, using the fragment in the identifier.
	ParseSourceCode(entityID string, content []byte) (string, error)
}

// FetchFunc is a rate-limited fetch function provided by the crawler.
// For local sources it reads files from disk. Implementations must honour
// context cancellation.
type FetchFunc func(ctx context.Context, id string) ([]byte, error)

// LibraryMeta is a DTO that carries metadata for library registration.
// Returned by Source.Meta and passed directly to store.Store.UpsertLibrary.
type LibraryMeta struct {
	Name        string
	Description string
	SourceURL   string
	Version     string
	Language    string // "php", "go", "python", "javascript", "typescript", "java", "c", "cpp", "csharp", "ruby", "rust"
	TrustScore  float64
}

// Entity is a DTO that represents a top-level documentation entity (class,
// interface, trait, struct, enum, module, or stand-alone function) as returned
// by ParseEntity. Kind should be one of the Kind* constants from kinds.go.
type Entity struct {
	Slug        string
	Name        string
	Kind        string // Use KindClass, KindFunction, etc.
	Description string
	SourceFile  string
	SourceCode  string
	URL         string
	Visibility  string // "public", "protected", "private", "internal", "package"; empty defaults to "public"
	Deprecated  bool
	Properties  []Property
}

// Property is a DTO that represents a class/struct field or property.
// Visibility is one of "public", "protected", "private", or empty (defaults to "public").
type Property struct {
	Name        string
	Type        string
	Description string
	Visibility  string
	Since       string
}

// Method is a DTO that represents a method or function with full details as
// returned by ParseMethod. WrappedSource and WrappedMethod are populated by
// the crawler's wrapper-chain resolution step after ParseMethod returns; they
// are empty in the value returned by ParseMethod itself.
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
	Deprecated    bool
	Relations     []Relation
}

// Parameter is a DTO that describes a single function or method parameter.
//
// This is the canonical definition; defsource.Parameter is a type alias for
// this type (type Parameter = source.Parameter). Callers using either name
// are working with identical types — no conversion is needed.
type Parameter struct {
	// Name is the parameter name (e.g., "$post_id" in PHP, "ctx" in Go).
	Name string `json:"name"`

	// Type is the type annotation as it appears in source
	// (e.g., "int", "WP_Post|null", "Vec<u8>", "List[int]").
	Type string `json:"type"`

	// Required is true when the parameter has no default value.
	Required bool `json:"required"`

	// Description is the doc comment text for this parameter.
	Description string `json:"description"`
}

// Relation is a DTO that represents a cross-reference from a method to another
// entity. Language-neutral vocabulary for Kind: "calls", "called_by",
// "implements", "extends", "throws", "uses", "used_by", "overrides".
//
// This is the canonical definition; defsource.Relation is a type alias for
// this type (type Relation = source.Relation).
type Relation struct {
	// Kind is the relationship type (e.g., "uses", "used_by", "calls",
	// "implements", "extends", "overrides").
	Kind string `json:"kind"`

	// TargetName is the qualified name of the related entity
	// (e.g., "WP_Query::get_posts()", "net/http.Server").
	TargetName string `json:"target_name"`

	// TargetURL is the canonical documentation URL of the related entity,
	// if available.
	TargetURL string `json:"target_url,omitempty"`

	// Description is any additional text associated with the relation.
	Description string `json:"description,omitempty"`
}

// DocSnippet is the canonical DTO for a single documentation entry returned by
// QueryDocs. It represents either an entity-level or method-level snippet and
// carries all fields needed for Markdown formatting by the search package.
//
// This is the canonical definition; defsource.DocSnippet is a type alias for
// this type (type DocSnippet = source.DocSnippet). The JSON struct tags are
// defined here so both names produce identical serialisation output.
//
// Relevance and Relations are populated by the query client after retrieval;
// they are not used by the formatter and may be zero.
type DocSnippet struct {
	// EntityName is the name of the entity (e.g., "WP_Query", "http.Server").
	EntityName string `json:"entity_name"`

	// MethodName is the name of the method, empty for entity-level snippets.
	MethodName string `json:"method_name,omitempty"`

	// Signature is the full method/function signature.
	Signature string `json:"signature,omitempty"`

	// Description is the documentation summary.
	Description string `json:"description"`

	// Parameters lists the documented parameters.
	Parameters []Parameter `json:"parameters,omitempty"`

	// ReturnType is the return type annotation.
	ReturnType string `json:"return_type,omitempty"`

	// ReturnDesc is the description of the return value.
	ReturnDesc string `json:"return_desc,omitempty"`

	// SourceCode is the source of the entity or method body.
	SourceCode string `json:"source_code"`

	// WrappedSource is the source code of the underlying delegated-to function.
	WrappedSource string `json:"wrapped_source,omitempty"`

	// WrappedMethod is the name of the underlying delegated-to method.
	WrappedMethod string `json:"wrapped_method,omitempty"`

	// URL is the canonical source/documentation URL.
	URL string `json:"url"`

	// Relevance is the BM25 rank score. Lower (more negative) values indicate
	// higher relevance.
	Relevance float64 `json:"relevance"`

	// Relations lists cross-references to related methods.
	Relations []Relation `json:"relations,omitempty"`
}
