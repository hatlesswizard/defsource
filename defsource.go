// Package defsource provides a Go client for querying WordPress PHP documentation
// indexed from source code. Use New to open (or create) a SQLite database backed
// by FTS5, then call QueryDocs, ResolveLibrary, ListLibraries, or ListEntities.
// Use NewWithStore to inject any store.Store implementation — useful for testing.
package defsource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hatlesswizard/defsource/internal/search"
	"github.com/hatlesswizard/defsource/internal/store"
	"github.com/hatlesswizard/defsource/internal/store/sqlite"
)

// Client is the main entry point for querying WordPress PHP documentation.
// Construct one with New (SQLite-backed) or NewWithStore (custom store).
type Client struct {
	store       store.Store
	tokenBudget int
}

// queryConfig holds per-call options for QueryDocs.
type queryConfig struct {
	mode string // "all" or "any"
}

// QueryOption configures a single QueryDocs call.
type QueryOption func(*queryConfig)

// WithSearchMode sets the FTS5 search mode: "all" (AND semantics, default) or "any"
// (OR semantics). Use "any" when partial matches are acceptable.
func WithSearchMode(mode string) QueryOption {
	return func(c *queryConfig) { c.mode = mode }
}

// Option configures a Client at construction time.
type Option func(*Client)

// WithTokenBudget sets the maximum approximate token count used when formatting
// QueryDocs text responses. The default is 8000 tokens.
func WithTokenBudget(budget int) Option {
	return func(c *Client) {
		c.tokenBudget = budget
	}
}

// New creates a Client backed by the SQLite database at dbPath.
// The database is created if it does not exist; FTS5 migrations are applied
// automatically. Requires CGO_ENABLED=1 and the sqlite_fts5 build tag.
func New(dbPath string, opts ...Option) (*Client, error) {
	s, err := sqlite.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return NewWithStore(s, opts...), nil
}

// NewWithStore creates a Client backed by the provided store.Store implementation.
// This constructor satisfies the Dependency Inversion Principle: callers can inject
// any store implementation — including in-memory fakes for unit tests — without
// depending on the SQLite package.
func NewWithStore(s store.Store, opts ...Option) *Client {
	c := &Client{
		store:       s,
		tokenBudget: 8000,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Close releases the underlying store resources. It must be called when the Client
// is no longer needed to avoid leaking database connections.
func (c *Client) Close() error {
	return c.store.Close()
}

// ResolveLibrary searches for libraries whose name matches libraryName, then
// re-ranks the results against query for relevance. At most 5 libraries are
// returned. SnippetCount is computed on demand when the store has not cached it.
func (c *Client) ResolveLibrary(ctx context.Context, query, libraryName string) ([]Library, error) {
	records, err := c.store.SearchLibraries(ctx, libraryName)
	if err != nil {
		return nil, fmt.Errorf("search libraries: %w", err)
	}

	ranked := search.RankLibraries(query, records)

	if len(ranked) > 5 {
		ranked = ranked[:5]
	}

	libs := make([]Library, len(ranked))
	for i, r := range ranked {
		libs[i] = libraryFromRecord(r)
		c.populateSnippetCount(ctx, &libs[i])
	}
	return libs, nil
}

// QueryDocs retrieves documentation snippets for the library identified by
// libraryID that are relevant to query. opts may include WithSearchMode to
// control FTS5 AND/OR semantics. Returns a DocResult containing raw snippets
// and a pre-formatted text string budget-limited by the client's token budget.
func (c *Client) QueryDocs(ctx context.Context, libraryID, query string, opts ...QueryOption) (*DocResult, error) {
	cfg := queryConfig{mode: "all"}
	for _, opt := range opts {
		opt(&cfg)
	}

	lib, err := c.store.GetLibrary(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("get library: %w", err)
	}
	if lib == nil {
		return nil, fmt.Errorf("library %q: %w", libraryID, store.ErrNotFound)
	}

	searchResults, err := c.store.Search(ctx, libraryID, query, 20, cfg.mode)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	entityIDs := make(map[int64]struct{})
	for _, sr := range searchResults {
		entityIDs[sr.EntityID] = struct{}{}
	}

	entityByID := make(map[int64]store.EntityRecord, len(entityIDs))
	for eid := range entityIDs {
		entity, err := c.store.GetEntityByID(ctx, eid)
		if err != nil {
			return nil, fmt.Errorf("get entity %d: %w", eid, err)
		}
		if entity != nil {
			entityByID[eid] = *entity
		}
	}

	methodByID := make(map[int64]store.MethodRecord)
	for eid := range entityIDs {
		methods, err := c.store.ListMethods(ctx, eid)
		if err != nil {
			return nil, fmt.Errorf("list methods for entity %d: %w", eid, err)
		}
		for _, m := range methods {
			methodByID[m.ID] = m
		}
	}

	snippets := make([]DocSnippet, 0)
	for _, sr := range searchResults {
		entity, ok := entityByID[sr.EntityID]
		if !ok {
			continue
		}

		snippet := DocSnippet{
			EntityName:  entity.Name,
			Description: entity.Description,
			SourceCode:  entity.SourceCode,
			URL:         entity.URL,
			Relevance:   sr.Rank,
		}

		if sr.MethodID != nil {
			if m, ok := methodByID[*sr.MethodID]; ok {
				snippet.MethodName = m.Name
				snippet.Signature = m.Signature
				snippet.Description = m.Description
				snippet.SourceCode = m.SourceCode
				snippet.WrappedSource = m.WrappedSource
				snippet.WrappedMethod = m.WrappedMethod
				snippet.ReturnType = m.ReturnType
				snippet.ReturnDesc = m.ReturnDesc
				snippet.URL = m.URL
				snippet.Parameters = parseParameters(m.ParametersJSON)

				rels, err := c.store.ListRelations(ctx, m.ID)
				if err != nil {
					return nil, fmt.Errorf("list relations for method %d: %w", m.ID, err)
				}
				for _, r := range rels {
					snippet.Relations = append(snippet.Relations, Relation{
						Kind:        r.Kind,
						TargetName:  r.TargetName,
						TargetURL:   r.TargetURL,
						Description: r.Description,
					})
				}
			}
		}

		snippets = append(snippets, snippet)
	}

	// Parameter, Relation, and DocSnippet are type aliases for the canonical
	// source.Parameter, source.Relation, and source.DocSnippet types (declared
	// in types.go).  FormatDocSnippets now accepts []source.DocSnippet directly,
	// so snippets (which is []DocSnippet = []source.DocSnippet) can be passed
	// through without any field-by-field translation.
	text := search.FormatDocSnippets(snippets, c.tokenBudget)

	return &DocResult{
		Library:  libraryID,
		Query:    query,
		Snippets: snippets,
		Text:     text,
	}, nil
}

// ListLibraries returns all indexed libraries. SnippetCount is computed on
// demand for libraries whose stored count is zero.
func (c *Client) ListLibraries(ctx context.Context) ([]Library, error) {
	records, err := c.store.ListLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	libs := make([]Library, len(records))
	for i, r := range records {
		libs[i] = libraryFromRecord(r)
		c.populateSnippetCount(ctx, &libs[i])
	}
	return libs, nil
}

// ListEntities returns summary information about every entity (class or function)
// belonging to the library identified by libraryID.
func (c *Client) ListEntities(ctx context.Context, libraryID string) ([]EntityInfo, error) {
	records, err := c.store.ListEntities(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	result := make([]EntityInfo, len(records))
	for i, r := range records {
		methods, err := c.store.ListMethods(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("list methods for entity %d: %w", r.ID, err)
		}
		result[i] = EntityInfo{
			Name:        r.Name,
			Slug:        r.Slug,
			Kind:        r.Kind,
			Description: r.Description,
			MethodCount: len(methods),
			URL:         r.URL,
		}
	}
	return result, nil
}

// libraryFromRecord converts a store.LibraryRecord to the public Library type.
// Both ResolveLibrary and ListLibraries share this mapping; centralising it here
// ensures any field addition is applied in both call sites automatically.
func libraryFromRecord(r store.LibraryRecord) Library {
	return Library{
		ID:           r.ID,
		Name:         r.Name,
		Description:  r.Description,
		SourceURL:    r.SourceURL,
		Version:      r.Version,
		TrustScore:   r.TrustScore,
		SnippetCount: r.SnippetCount,
		CrawledAt:    r.CrawledAt,
	}
}

// populateSnippetCount fills lib.SnippetCount by counting entities and methods
// when the stored value is zero. It is a best-effort helper: if the store
// returns an error the count is left at zero rather than propagating a failure
// into a listing call.
func (c *Client) populateSnippetCount(ctx context.Context, lib *Library) {
	if lib.SnippetCount > 0 {
		return
	}
	entities, err := c.store.ListEntities(ctx, lib.ID)
	if err != nil || len(entities) == 0 {
		return
	}
	count := len(entities)
	for _, e := range entities {
		methods, err := c.store.ListMethods(ctx, e.ID)
		if err != nil {
			return
		}
		count += len(methods)
	}
	lib.SnippetCount = count
}

// parseParameters deserialises a JSON-encoded parameter list stored in the
// database. It returns nil on an empty value or a JSON parse error; callers
// should treat nil as "no parameters".
func parseParameters(jsonStr string) []Parameter {
	if jsonStr == "" || jsonStr == "[]" {
		return nil
	}
	var params []Parameter
	if err := json.Unmarshal([]byte(jsonStr), &params); err != nil {
		return nil
	}
	return params
}
