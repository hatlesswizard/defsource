// Package store defines the persistence interface and data-transfer types used
// by all storage backends.
//
// All types in this file are intentionally plain data-transfer objects (DTOs):
// they have only exported fields and no methods. Their purpose is to carry data
// across the boundary between the storage layer (SQLite, or any future backend)
// and consumers (the crawler and the public query client). Do not add behaviour
// to these types; logic belongs in the Store implementation or the caller.
package store

import "time"

// LibraryRecord is a DTO that maps one-to-one to a row in the libraries table.
// All fields are exposed; callers read them directly. The record includes
// CreatedAt and UpdatedAt timestamps that are not surfaced in the public
// defsource.Library type — they are internal housekeeping fields.
type LibraryRecord struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SourceURL    string    `json:"source_url"`
	Version      string    `json:"version"`
	TrustScore   float64   `json:"trust_score"`
	SnippetCount int       `json:"snippet_count"`
	CrawledAt    time.Time `json:"crawled_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// EntityRecord is a DTO that maps one-to-one to a row in the entities table.
// An entity is a top-level PHP construct — class, interface, trait, or
// stand-alone function. Slug is derived from the entity's URL path, not from
// the display name, to guarantee uniqueness across the library.
type EntityRecord struct {
	ID          int64     `json:"id"`
	LibraryID   string    `json:"library_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Description string    `json:"description"`
	SourceFile  string    `json:"source_file"`
	SourceCode  string    `json:"source_code"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MethodRecord is a DTO that maps one-to-one to a row in the methods table.
// ParametersJSON stores the method's parameter list as a JSON array; callers
// are responsible for unmarshalling it (see defsource.parseParameters). This
// is the raw storage representation — the JSON encoding detail is part of the
// contract for consumers of this type.
//
// WrappedSource and WrappedMethod are non-empty only for wrapper functions
// whose implementation delegates to another function or method; they carry
// the resolved source code and identifier of the ultimate target.
type MethodRecord struct {
	ID             int64     `json:"id"`
	EntityID       int64     `json:"entity_id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	Signature      string    `json:"signature"`
	Description    string    `json:"description"`
	ParametersJSON string    `json:"parameters_json"`
	ReturnType     string    `json:"return_type"`
	ReturnDesc     string    `json:"return_desc"`
	SourceCode     string    `json:"source_code"`
	WrappedSource  string    `json:"wrapped_source"`
	WrappedMethod  string    `json:"wrapped_method"`
	URL            string    `json:"url"`
	Since          string    `json:"since"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RelationRecord is a DTO that maps one-to-one to a row in the relations table.
// A relation is a PHPDoc cross-reference (@see / @uses) from a method to
// another function or method. Kind is "uses" for both @see and @uses tags.
type RelationRecord struct {
	ID          int64  `json:"id"`
	MethodID    int64  `json:"method_id"`
	Kind        string `json:"kind"`
	TargetName  string `json:"target_name"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description"`
}

// CrawlSession is a DTO that maps one-to-one to a row in the crawl_sessions
// table. Status is one of "running", "completed", or "interrupted".
// CompletedAt is nil while the session is still running.
type CrawlSession struct {
	ID           int64
	LibraryID    string
	Status       string
	TotalURLs    int
	SuccessCount int
	FailCount    int
	SkipCount    int
	StartedAt    time.Time
	CompletedAt  *time.Time
}

// CrawlProgressItem is a DTO that maps one-to-one to a row in the
// crawl_progress table. Each row records the outcome of crawling one URL.
// ErrorType and ErrorMessage are empty when Status is "success".
// ParentEntity is the entity URL that owns this method URL, or empty for
// entity-level items.
type CrawlProgressItem struct {
	URL          string
	ItemType     string
	Status       string
	ErrorType    string
	ErrorMessage string
	ParentEntity string
}

// CrawlStats is a DTO holding aggregate counts computed from the
// crawl_progress table for a single session. FailuresByType maps error
// type strings (e.g., "http_404", "parse_error") to their occurrence counts.
type CrawlStats struct {
	Total          int
	Success        int
	Failed         int
	Skipped        int
	FailuresByType map[string]int
}

// SearchResult is a DTO that represents a single BM25-ranked FTS5 search hit.
// SnippetType is "class" when MethodID is nil (entity-level hit) and "method"
// when MethodID is non-nil (method-level hit). Rank holds the raw BM25 score;
// lower (more negative) values indicate a better match.
type SearchResult struct {
	EntityID    int64   `json:"entity_id"`
	MethodID    *int64  `json:"method_id,omitempty"`
	SnippetType string  `json:"snippet_type"` // "class" or "method"
	EntityName  string  `json:"entity_name"`
	MethodName  string  `json:"method_name,omitempty"`
	Rank        float64 `json:"rank"`
}
