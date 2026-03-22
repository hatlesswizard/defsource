package defsource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hatlesswizard/defsource/internal/search"
	"github.com/hatlesswizard/defsource/internal/store"
	"github.com/hatlesswizard/defsource/internal/store/sqlite"
)

// Client is the main entry point for querying documentation.
type Client struct {
	store       store.Store
	tokenBudget int
}

// queryConfig holds options for QueryDocs.
type queryConfig struct {
	mode string // "all" or "any"
}

// QueryOption configures a QueryDocs call.
type QueryOption func(*queryConfig)

// WithSearchMode sets the FTS5 search mode: "all" (AND, default) or "any" (OR).
func WithSearchMode(mode string) QueryOption {
	return func(c *queryConfig) { c.mode = mode }
}

// Option configures the Client.
type Option func(*Client)

// WithTokenBudget sets the maximum approximate token count for query-docs responses.
func WithTokenBudget(budget int) Option {
	return func(c *Client) {
		c.tokenBudget = budget
	}
}

// New creates a new defSource client backed by a SQLite database at dbPath.
func New(dbPath string, opts ...Option) (*Client, error) {
	s, err := sqlite.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	c := &Client{
		store:       s,
		tokenBudget: 8000,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Close releases database resources.
func (c *Client) Close() error {
	return c.store.Close()
}

// ResolveLibrary searches for libraries matching the given name.
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
		libs[i] = Library{
			ID:           r.ID,
			Name:         r.Name,
			Description:  r.Description,
			SourceURL:    r.SourceURL,
			Version:      r.Version,
			TrustScore:   r.TrustScore,
			SnippetCount: r.SnippetCount,
			CrawledAt:    r.CrawledAt,
		}
		c.computeSnippetCount(ctx, &libs[i])
	}
	return libs, nil
}

// QueryDocs retrieves documentation for a specific library.
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
		return nil, fmt.Errorf("library %q not found", libraryID)
	}

	searchResults, err := c.store.Search(ctx, libraryID, query, 20, cfg.mode)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// Load only the entities referenced by search results.
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

	// For each entity that appears in results, load its methods and index by ID.
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

	// Build DocSnippets from search results.
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

				rels, _ := c.store.ListRelations(ctx, m.ID)
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

	// Convert to search.Snippet for formatting (avoids import cycle).
	fmtSnippets := make([]search.Snippet, len(snippets))
	for i, s := range snippets {
		params := make([]search.SnippetParam, len(s.Parameters))
		for j, p := range s.Parameters {
			params[j] = search.SnippetParam{
				Name:        p.Name,
				Type:        p.Type,
				Required:    p.Required,
				Description: p.Description,
			}
		}
		fmtSnippets[i] = search.Snippet{
			EntityName:    s.EntityName,
			MethodName:    s.MethodName,
			Signature:     s.Signature,
			Description:   s.Description,
			Parameters:    params,
			ReturnType:    s.ReturnType,
			ReturnDesc:    s.ReturnDesc,
			SourceCode:    s.SourceCode,
			WrappedSource: s.WrappedSource,
			WrappedMethod: s.WrappedMethod,
			URL:           s.URL,
		}
	}
	text := search.FormatDocSnippets(fmtSnippets, c.tokenBudget)

	return &DocResult{
		Library:  libraryID,
		Query:    query,
		Snippets: snippets,
		Text:     text,
	}, nil
}

// ListLibraries returns all indexed libraries.
func (c *Client) ListLibraries(ctx context.Context) ([]Library, error) {
	records, err := c.store.ListLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	libs := make([]Library, len(records))
	for i, r := range records {
		libs[i] = Library{
			ID:           r.ID,
			Name:         r.Name,
			Description:  r.Description,
			SourceURL:    r.SourceURL,
			Version:      r.Version,
			TrustScore:   r.TrustScore,
			SnippetCount: r.SnippetCount,
			CrawledAt:    r.CrawledAt,
		}
		c.computeSnippetCount(ctx, &libs[i])
	}
	return libs, nil
}

// ListEntities returns all entities for a library.
func (c *Client) ListEntities(ctx context.Context, libraryID string) ([]EntityInfo, error) {
	records, err := c.store.ListEntities(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	result := make([]EntityInfo, len(records))
	for i, r := range records {
		methods, _ := c.store.ListMethods(ctx, r.ID)
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

func (c *Client) computeSnippetCount(ctx context.Context, lib *Library) {
	if lib.SnippetCount > 0 {
		return
	}
	entities, err := c.store.ListEntities(ctx, lib.ID)
	if err != nil || len(entities) == 0 {
		return
	}
	count := len(entities)
	for _, e := range entities {
		methods, _ := c.store.ListMethods(ctx, e.ID)
		count += len(methods)
	}
	lib.SnippetCount = count
}

// Store returns the underlying store for use by the crawler.
func (c *Client) Store() store.Store {
	return c.store
}

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
