package defsource

import (
	"time"

	"github.com/hatlesswizard/defsource/internal/source"
)

// Library represents an indexed documentation source (e.g., a WordPress version).
type Library struct {
	// ID is the unique library identifier used as the libraryID parameter in
	// QueryDocs and ListEntities (e.g., "wordpress-6.5").
	ID string `json:"id"`

	// Name is the human-readable display name (e.g., "WordPress 6.5").
	Name string `json:"name"`

	// Description is a short summary of the library.
	Description string `json:"description"`

	// SourceURL is the canonical URL from which the library was crawled.
	SourceURL string `json:"source_url"`

	// Version is the version string reported by the source (e.g., "6.5.0").
	Version string `json:"version"`

	// Language is the programming language of the library (e.g., "php", "python", "go").
	Language string `json:"language"`

	// TrustScore is a normalised relevance weight in the range [0, 1].
	TrustScore float64 `json:"trust_score"`

	// SnippetCount is the total number of indexed documentation snippets
	// (entities + methods). It may be 0 if not yet computed.
	SnippetCount int `json:"snippet_count"`

	// CrawledAt is the time the library was last fully crawled.
	CrawledAt time.Time `json:"crawled_at"`
}

// LanguageInfo holds summary information about a supported language.
type LanguageInfo struct {
	// Language is the language identifier (e.g., "python", "go", "rust").
	Language string `json:"language"`

	// FrameworkCount is the number of libraries/frameworks indexed for this language.
	FrameworkCount int `json:"framework_count"`
}

// DocResult is the response from QueryDocs.
type DocResult struct {
	// Library is the libraryID that was queried.
	Library string `json:"library"`

	// Query is the search query string that produced these results.
	Query string `json:"query"`

	// Snippets contains the individual documentation entries ranked by relevance.
	Snippets []DocSnippet `json:"snippets"`

	// Text is a pre-formatted Markdown representation of Snippets, truncated
	// to the client's token budget.
	Text string `json:"text"`
}

// DocSnippet is re-exported from internal/source for the public API.
// It is the canonical type for a single documentation entry returned by
// QueryDocs; callers may use either defsource.DocSnippet or source.DocSnippet —
// they are identical types.
type DocSnippet = source.DocSnippet

// Parameter is re-exported from internal/source for the public API.
// Callers using defsource.Parameter continue to work without change —
// the type alias means defsource.Parameter and source.Parameter are identical.
type Parameter = source.Parameter

// Relation is re-exported from internal/source for the public API.
// Callers using defsource.Relation continue to work without change —
// the type alias means defsource.Relation and source.Relation are identical.
type Relation = source.Relation

// EntityInfo represents summary information about an entity returned by
// ListEntities. It does not include full source code or method details.
type EntityInfo struct {
	// Name is the display name of the entity (e.g., "WP_Query").
	Name string `json:"name"`

	// Slug is the URL-derived unique identifier for the entity within its library.
	Slug string `json:"slug"`

	// Kind is the entity type: "class" or "function".
	Kind string `json:"kind"`

	// Description is the PHPDoc summary for the entity.
	Description string `json:"description"`

	// MethodCount is the number of indexed methods belonging to this entity.
	MethodCount int `json:"method_count"`

	// URL is the canonical documentation URL for this entity.
	URL string `json:"url"`
}
