// Package source defines the Source interface and the data-transfer types that
// flow between a source adapter and the crawler. Each documentation framework
// (WordPress, Laravel, React, etc.) provides a concrete Source implementation;
// the crawler consumes the interface without knowing which framework is in use.
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
// Each documentation framework (WordPress, Laravel, React, etc.)
// implements this interface. The crawler uses it generically.
//
// # Lifecycle and temporal coupling
//
// The seven methods on Source must be called in a specific order. The crawler
// always follows this sequence:
//
//  1. ID and Meta may be called at any time after construction.
//  2. DiscoverEntities must be called before DetectWrapper, ResolveWrapperURL,
//     or ParseSourceCode. Implementations typically build an internal index of
//     the codebase during DiscoverEntities; calling wrapper-resolution methods
//     before this step produces empty or incorrect results (they return zero
//     values rather than errors — a known temporal coupling that is documented
//     here rather than restructured, per Wave-1 scope constraints).
//  3. ParseEntity and ParseMethod may be called concurrently by multiple
//     crawler goroutines after DiscoverEntities completes.
//  4. DetectWrapper, ResolveWrapperURL, and ParseSourceCode are called during
//     wrapper chain resolution, also concurrently after DiscoverEntities.
//
// Implementations are responsible for thread safety on all methods that are
// called concurrently (steps 3 and 4 above).
type Source interface {
	// ID returns the canonical library ID (e.g., "/wordpress/classes").
	ID() string

	// Meta returns metadata for the library record.
	Meta() LibraryMeta

	// DiscoverEntities returns URLs of all top-level entities to crawl.
	// The fetch function is provided by the crawler and is rate-limited.
	// DiscoverEntities must be called before DetectWrapper, ResolveWrapperURL,
	// or ParseSourceCode; those methods depend on the internal index built here.
	DiscoverEntities(ctx context.Context, fetch FetchFunc) ([]string, error)

	// ParseEntity parses a single entity page and returns its data
	// plus a list of method URLs to crawl next.
	ParseEntity(ctx context.Context, url string, body []byte) (*Entity, []string, error)

	// ParseMethod parses a single method/function detail page.
	ParseMethod(ctx context.Context, url string, body []byte) (*Method, error)

	// DetectWrapper analyzes a method's source code and returns wrapper info.
	// Returns (isWrapper, targetName, targetKind) where targetKind is
	// "function", "self_method", or "static_method".
	// Returns (false, "", "") if the method is not a wrapper, or if
	// DiscoverEntities has not yet been called.
	DetectWrapper(method *Method) (bool, string, string)

	// ResolveWrapperURL constructs the URL to fetch the wrapped method's page.
	// Returns an empty string if the target cannot be resolved or if
	// DiscoverEntities has not yet been called.
	ResolveWrapperURL(targetName, targetKind, entitySlug string) string

	// ParseSourceCode extracts just the source code from a page body.
	// Called during wrapper chain resolution to obtain the source of a
	// delegated-to function or method.
	ParseSourceCode(url string, body []byte) (string, error)
}

// FetchFunc is a rate-limited HTTP fetch function provided by the crawler.
// Implementations must honour context cancellation.
type FetchFunc func(ctx context.Context, url string) ([]byte, error)

// LibraryMeta is a DTO that carries metadata for library registration.
// Returned by Source.Meta and passed directly to store.Store.UpsertLibrary.
type LibraryMeta struct {
	Name        string
	Description string
	SourceURL   string
	Version     string
	TrustScore  float64
}

// Entity is a DTO that represents a top-level documentation entity (class,
// interface, trait, or stand-alone function) as returned by ParseEntity.
// Kind is one of "class", "interface", "trait", or "function".
type Entity struct {
	Slug        string
	Name        string
	Kind        string // "class", "interface", "trait", "function"
	Description string
	SourceFile  string
	SourceCode  string
	URL         string
	Properties  []Property
}

// Property is a DTO that represents a class or interface property as parsed
// from PHPDoc. Visibility is one of "public", "protected", or "private".
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
	Relations     []Relation
}

// Parameter is a DTO that describes a single function or method parameter.
// Required is false when the PHPDoc description begins with "Optional".
//
// This is the canonical definition; defsource.Parameter is a type alias for
// this type (type Parameter = source.Parameter). Callers using either name
// are working with identical types — no conversion is needed.
type Parameter struct {
	// Name is the PHP parameter name including the leading dollar sign (e.g., "$post_id").
	Name string `json:"name"`

	// Type is the PHP type annotation (e.g., "int", "WP_Post|null").
	Type string `json:"type"`

	// Required is true when the parameter has no default value and is not
	// described as optional in the PHPDoc.
	Required bool `json:"required"`

	// Description is the PHPDoc text for this parameter.
	Description string `json:"description"`
}

// Relation is a DTO that represents a PHPDoc cross-reference (@see or @uses)
// from a method to another function or method. Kind is "uses" for both tag
// types. TargetName uses WordPress method-reference notation, e.g.
// "WP_Query::get_posts()".
//
// This is the canonical definition; defsource.Relation is a type alias for
// this type (type Relation = source.Relation).
type Relation struct {
	// Kind is the relationship type, either "uses" (this method calls the target)
	// or "used_by" (this method is called by the target).
	Kind string `json:"kind"`

	// TargetName is the PHP name of the related class or method.
	TargetName string `json:"target_name"`

	// TargetURL is the canonical documentation URL of the related entity,
	// if available.
	TargetURL string `json:"target_url,omitempty"`

	// Description is any additional PHPDoc text associated with the relation.
	Description string `json:"description,omitempty"`
}

// DocSnippet is the canonical DTO for a single documentation entry returned by
// QueryDocs. It represents either a class-level or method-level snippet and
// carries all fields needed for Markdown formatting by the search package.
//
// This is the canonical definition; defsource.DocSnippet is a type alias for
// this type (type DocSnippet = source.DocSnippet). The JSON struct tags are
// defined here so both names produce identical serialisation output.
//
// Relevance and Relations are populated by the query client after retrieval;
// they are not used by the formatter and may be zero.
type DocSnippet struct {
	// EntityName is the name of the PHP class or function (e.g., "WP_Query").
	EntityName string `json:"entity_name"`

	// MethodName is the name of the method within the entity, if this snippet
	// is method-level. Empty for class-level snippets.
	MethodName string `json:"method_name,omitempty"`

	// Signature is the full PHP method signature including parameters and return
	// type, if available.
	Signature string `json:"signature,omitempty"`

	// Description is the PHPDoc summary for this entity or method.
	Description string `json:"description"`

	// Parameters lists the documented parameters for this method.
	Parameters []Parameter `json:"parameters,omitempty"`

	// ReturnType is the PHP type of the method's return value.
	ReturnType string `json:"return_type,omitempty"`

	// ReturnDesc is the PHPDoc description of the return value.
	ReturnDesc string `json:"return_desc,omitempty"`

	// SourceCode is the PHP source of the entity or method body.
	SourceCode string `json:"source_code"`

	// WrappedSource is the source code of the underlying function that this
	// method delegates to, when wrapper detection resolves a chain.
	WrappedSource string `json:"wrapped_source,omitempty"`

	// WrappedMethod is the name of the underlying delegated-to method.
	WrappedMethod string `json:"wrapped_method,omitempty"`

	// URL is the canonical documentation URL for this snippet.
	URL string `json:"url"`

	// Relevance is the BM25 rank score. Lower (more negative) values indicate
	// higher relevance.
	Relevance float64 `json:"relevance"`

	// Relations lists cross-references to related methods (e.g., @uses, @see).
	Relations []Relation `json:"relations,omitempty"`
}
