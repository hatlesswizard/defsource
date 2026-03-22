package store

import "time"

// LibraryRecord represents a library row from the database.
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

// EntityRecord represents an entity row from the database.
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

// MethodRecord represents a method row from the database.
type MethodRecord struct {
	ID            int64     `json:"id"`
	EntityID      int64     `json:"entity_id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Signature     string    `json:"signature"`
	Description   string    `json:"description"`
	ParametersJSON string   `json:"parameters_json"`
	ReturnType    string    `json:"return_type"`
	ReturnDesc    string    `json:"return_desc"`
	SourceCode    string    `json:"source_code"`
	WrappedSource string    `json:"wrapped_source"`
	WrappedMethod string    `json:"wrapped_method"`
	URL           string    `json:"url"`
	Since         string    `json:"since"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RelationRecord represents a relation row from the database.
type RelationRecord struct {
	ID          int64  `json:"id"`
	MethodID    int64  `json:"method_id"`
	Kind        string `json:"kind"`
	TargetName  string `json:"target_name"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description"`
}

// CrawlSession represents a crawl session row from the database.
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

// CrawlProgressItem represents a single crawl progress entry.
type CrawlProgressItem struct {
	URL          string
	ItemType     string
	Status       string
	ErrorType    string
	ErrorMessage string
	ParentEntity string
}

// CrawlStats holds aggregate statistics for a crawl session.
type CrawlStats struct {
	Total          int
	Success        int
	Failed         int
	Skipped        int
	FailuresByType map[string]int
}

// SearchResult represents a single FTS5 search hit.
type SearchResult struct {
	EntityID    int64   `json:"entity_id"`
	MethodID    *int64  `json:"method_id,omitempty"`
	SnippetType string  `json:"snippet_type"` // "class" or "method"
	EntityName  string  `json:"entity_name"`
	MethodName  string  `json:"method_name,omitempty"`
	Rank        float64 `json:"rank"`
}
