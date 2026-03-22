package defsource

import "time"

// Library represents an indexed documentation source.
type Library struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SourceURL    string    `json:"source_url"`
	Version      string    `json:"version"`
	TrustScore   float64   `json:"trust_score"`
	SnippetCount int       `json:"snippet_count"`
	CrawledAt    time.Time `json:"crawled_at"`
}

// DocResult is the response from QueryDocs.
type DocResult struct {
	Library  string       `json:"library"`
	Query    string       `json:"query"`
	Snippets []DocSnippet `json:"snippets"`
	Text     string       `json:"text"`
}

// DocSnippet is a single documentation entry (class or method).
type DocSnippet struct {
	EntityName    string      `json:"entity_name"`
	MethodName    string      `json:"method_name,omitempty"`
	Signature     string      `json:"signature,omitempty"`
	Description   string      `json:"description"`
	Parameters    []Parameter `json:"parameters,omitempty"`
	ReturnType    string      `json:"return_type,omitempty"`
	ReturnDesc    string      `json:"return_desc,omitempty"`
	SourceCode    string      `json:"source_code"`
	WrappedSource string      `json:"wrapped_source,omitempty"`
	WrappedMethod string      `json:"wrapped_method,omitempty"`
	URL           string      `json:"url"`
	Relevance     float64     `json:"relevance"`
	Relations     []Relation  `json:"relations,omitempty"`
}

// Parameter describes a function/method parameter.
type Parameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// Relation describes a relationship between methods (uses/used_by).
type Relation struct {
	Kind        string `json:"kind"`
	TargetName  string `json:"target_name"`
	TargetURL   string `json:"target_url,omitempty"`
	Description string `json:"description,omitempty"`
}

// EntityInfo represents summary information about an entity.
type EntityInfo struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	MethodCount int    `json:"method_count"`
	URL         string `json:"url"`
}
