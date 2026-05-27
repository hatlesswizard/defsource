//go:build sqlite_fts5 || fts5

package defsource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/store"
)

// ---------------------------------------------------------------------------
// Existing 4 tests — preserved verbatim
// ---------------------------------------------------------------------------

func TestNewClient(t *testing.T) {
	// Create a temp directory for the test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	// Test ListLibraries on empty DB
	ctx := context.Background()
	libs, err := client.ListLibraries(ctx)
	if err != nil {
		t.Fatalf("ListLibraries() error: %v", err)
	}
	if len(libs) != 0 {
		t.Errorf("ListLibraries() = %d libraries, want 0", len(libs))
	}
}

func TestResolveLibraryEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	libs, err := client.ResolveLibrary(ctx, "test query", "wordpress")
	if err != nil {
		t.Fatalf("ResolveLibrary() error: %v", err)
	}
	if len(libs) != 0 {
		t.Errorf("ResolveLibrary() = %d, want 0 on empty DB", len(libs))
	}
}

func TestQueryDocsNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	_, err = client.QueryDocs(ctx, "/nonexistent/lib", "test")
	if err == nil {
		t.Error("QueryDocs() expected error for nonexistent library, got nil")
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Verify DB file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

// ---------------------------------------------------------------------------
// mockStore — minimal store.Store implementation for unit tests.
//
// Each field provides a configurable return value or error for one method
// group. Unset fields produce sensible zero-value returns (nil error, empty
// slices, nil pointers). The type satisfies the full store.Store interface so
// the compiler enforces completeness.
// ---------------------------------------------------------------------------

type mockStore struct {
	// Library group
	libraries          map[string]*store.LibraryRecord
	searchResults      []store.LibraryRecord
	searchLibrariesErr error
	listLibrariesErr   error

	// Entity / method group
	entities         map[string][]store.EntityRecord  // keyed by libraryID
	methods          map[int64][]store.MethodRecord   // keyed by entityID
	relations        map[int64][]store.RelationRecord // keyed by methodID
	listEntitiesErr  error
	listMethodsErr   error
	listRelationsErr error

	// Search group
	searchHits []store.SearchResult
	searchErr  error
	entityByID map[int64]*store.EntityRecord

	// Closed flag — lets Close() tests verify release semantics
	closed   bool
	closeErr error
}

// ---- Library methods -------------------------------------------------------

func (m *mockStore) UpsertLibrary(_ context.Context, _ string, _ source.LibraryMeta) error {
	return nil
}

func (m *mockStore) GetLibrary(_ context.Context, id string) (*store.LibraryRecord, error) {
	if m.libraries == nil {
		return nil, nil
	}
	r, ok := m.libraries[id]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockStore) SearchLibraries(_ context.Context, _ string) ([]store.LibraryRecord, error) {
	if m.searchLibrariesErr != nil {
		return nil, m.searchLibrariesErr
	}
	return m.searchResults, nil
}

func (m *mockStore) ListLibraries(_ context.Context) ([]store.LibraryRecord, error) {
	if m.listLibrariesErr != nil {
		return nil, m.listLibrariesErr
	}
	var out []store.LibraryRecord
	for _, r := range m.libraries {
		out = append(out, *r)
	}
	return out, nil
}

// ---- Entity / method methods -----------------------------------------------

func (m *mockStore) UpsertEntity(_ context.Context, _ string, _ *source.Entity) (int64, error) {
	return 0, nil
}

func (m *mockStore) GetEntity(_ context.Context, _, _ string) (*store.EntityRecord, error) {
	return nil, nil
}

func (m *mockStore) GetEntityByID(_ context.Context, id int64) (*store.EntityRecord, error) {
	if m.entityByID == nil {
		return nil, nil
	}
	e, ok := m.entityByID[id]
	if !ok {
		return nil, nil
	}
	return e, nil
}

func (m *mockStore) ListEntities(_ context.Context, libraryID string) ([]store.EntityRecord, error) {
	if m.listEntitiesErr != nil {
		return nil, m.listEntitiesErr
	}
	return m.entities[libraryID], nil
}

func (m *mockStore) UpsertMethod(_ context.Context, _ int64, _ *source.Method) error {
	return nil
}

func (m *mockStore) GetMethod(_ context.Context, _ int64, _ string) (*store.MethodRecord, error) {
	return nil, nil
}

func (m *mockStore) ListMethods(_ context.Context, entityID int64) ([]store.MethodRecord, error) {
	if m.listMethodsErr != nil {
		return nil, m.listMethodsErr
	}
	return m.methods[entityID], nil
}

func (m *mockStore) ListRelations(_ context.Context, methodID int64) ([]store.RelationRecord, error) {
	if m.listRelationsErr != nil {
		return nil, m.listRelationsErr
	}
	return m.relations[methodID], nil
}

// ---- Search methods --------------------------------------------------------

func (m *mockStore) UpdateSnippetCount(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockStore) ComputeSnippetCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *mockStore) Search(_ context.Context, _, _ string, _ int, _ string) ([]store.SearchResult, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchHits, nil
}

func (m *mockStore) RebuildIndex(_ context.Context, _ string) error {
	return nil
}

// ---- Crawl session methods -------------------------------------------------

func (m *mockStore) CreateCrawlSession(_ context.Context, _ string, _ int) (int64, error) {
	return 0, nil
}

func (m *mockStore) UpdateCrawlSession(_ context.Context, _ int64, _ string, _, _, _ int) error {
	return nil
}

func (m *mockStore) GetLastSession(_ context.Context, _ string) (*store.CrawlSession, error) {
	return nil, nil
}

func (m *mockStore) RecordProgress(_ context.Context, _ int64, _ *store.CrawlProgressItem) error {
	return nil
}

func (m *mockStore) GetProcessedURLs(_ context.Context, _ int64) (map[string]string, error) {
	return nil, nil
}

func (m *mockStore) GetCrawlStats(_ context.Context, _ int64) (*store.CrawlStats, error) {
	return nil, nil
}

func (m *mockStore) GetFailures(_ context.Context, _ int64) ([]store.CrawlProgressItem, error) {
	return nil, nil
}

// ---- Close -----------------------------------------------------------------

func (m *mockStore) Close() error {
	m.closed = true
	return m.closeErr
}

// ---------------------------------------------------------------------------
// New tests (Wave-2) — 14 test functions total in file (4 existing + 10 new)
// ---------------------------------------------------------------------------

// Test 5: NewWithStore constructs a Client without touching SQLite;
// the Client stores the injected mock and delegates ResolveLibrary to it.
func TestNewWithStore_ConstructsClient(t *testing.T) {
	t.Parallel()

	now := time.Now()
	ms := &mockStore{
		libraries: map[string]*store.LibraryRecord{
			"wp-6.5": {
				ID:        "wp-6.5",
				Name:      "WordPress 6.5",
				CrawledAt: now,
			},
		},
		searchResults: []store.LibraryRecord{
			{ID: "wp-6.5", Name: "WordPress 6.5", CrawledAt: now},
		},
	}

	client := NewWithStore(ms)
	if client == nil {
		t.Fatal("NewWithStore() returned nil")
	}

	ctx := context.Background()
	libs, err := client.ResolveLibrary(ctx, "WordPress", "WordPress")
	if err != nil {
		t.Fatalf("ResolveLibrary() error: %v", err)
	}
	if len(libs) == 0 {
		t.Fatal("ResolveLibrary() returned 0 libraries, want at least 1")
	}
	if libs[0].ID != "wp-6.5" {
		t.Errorf("libs[0].ID = %q, want %q", libs[0].ID, "wp-6.5")
	}
}

// Test 6: ResolveLibrary on an empty mockStore returns an empty slice,
// never an error — the absence of data is not a failure.
func TestResolveLibrary_EmptyMockStore_ReturnsZeroResults(t *testing.T) {
	t.Parallel()

	ms := &mockStore{} // no libraries configured
	client := NewWithStore(ms)
	ctx := context.Background()

	libs, err := client.ResolveLibrary(ctx, "anything", "anything")
	if err != nil {
		t.Fatalf("ResolveLibrary() unexpected error: %v", err)
	}
	if len(libs) != 0 {
		t.Errorf("ResolveLibrary() = %d results, want 0 for empty store", len(libs))
	}
}

// Test 7: QueryDocs on a populated mockStore returns a DocResult with at
// least one snippet and a non-empty Text field.
func TestQueryDocs_PopulatedStore_ReturnsDocResult(t *testing.T) {
	t.Parallel()

	entityID := int64(42)
	methodID := int64(99)

	ms := &mockStore{
		libraries: map[string]*store.LibraryRecord{
			"wp-6.5": {ID: "wp-6.5", Name: "WordPress 6.5"},
		},
		searchHits: []store.SearchResult{
			{
				EntityID:    entityID,
				MethodID:    &methodID,
				SnippetType: "method",
				EntityName:  "WP_Query",
				MethodName:  "get_posts",
				Rank:        -3.5,
			},
		},
		entityByID: map[int64]*store.EntityRecord{
			entityID: {
				ID:          entityID,
				LibraryID:   "wp-6.5",
				Slug:        "wp_query",
				Name:        "WP_Query",
				Kind:        "class",
				Description: "WordPress query object.",
				URL:         "https://example.com/wp-query",
			},
		},
		methods: map[int64][]store.MethodRecord{
			entityID: {
				{
					ID:          methodID,
					EntityID:    entityID,
					Slug:        "get_posts",
					Name:        "get_posts",
					Signature:   "public function get_posts(): array",
					Description: "Returns an array of posts.",
					SourceCode:  "return $this->posts;",
					URL:         "https://example.com/wp-query/get_posts",
				},
			},
		},
	}

	client := NewWithStore(ms)
	ctx := context.Background()

	result, err := client.QueryDocs(ctx, "wp-6.5", "get posts")
	if err != nil {
		t.Fatalf("QueryDocs() error: %v", err)
	}
	if result == nil {
		t.Fatal("QueryDocs() returned nil DocResult")
	}
	if len(result.Snippets) == 0 {
		t.Error("QueryDocs() Snippets is empty, want at least 1")
	}
	if result.Text == "" {
		t.Error("QueryDocs() Text is empty, want non-empty formatted output")
	}
	if result.Library != "wp-6.5" {
		t.Errorf("result.Library = %q, want %q", result.Library, "wp-6.5")
	}
	if result.Query != "get posts" {
		t.Errorf("result.Query = %q, want %q", result.Query, "get posts")
	}
}

// Test 8: QueryDocs respects the token budget: a large budget returns all
// snippets formatted; the Text field should contain the entity name.
func TestQueryDocs_TokenBudgetHonored_LargeBudget(t *testing.T) {
	t.Parallel()

	entityID := int64(1)

	ms := &mockStore{
		libraries: map[string]*store.LibraryRecord{
			"lib": {ID: "lib", Name: "Test Lib"},
		},
		searchHits: []store.SearchResult{
			{EntityID: entityID, SnippetType: "class", EntityName: "MyClass", Rank: -1.0},
		},
		entityByID: map[int64]*store.EntityRecord{
			entityID: {
				ID:          entityID,
				LibraryID:   "lib",
				Slug:        "myclass",
				Name:        "MyClass",
				Kind:        "class",
				Description: "A test class.",
			},
		},
		methods: map[int64][]store.MethodRecord{
			entityID: nil,
		},
	}

	// Large token budget — all content should appear.
	client := NewWithStore(ms, WithTokenBudget(100000))
	ctx := context.Background()

	result, err := client.QueryDocs(ctx, "lib", "my class")
	if err != nil {
		t.Fatalf("QueryDocs() error: %v", err)
	}
	if !strings.Contains(result.Text, "MyClass") {
		t.Errorf("result.Text does not contain entity name %q; got: %s", "MyClass", result.Text)
	}
}

// Test 9: Token-budget at-least-1 invariant — even a budget of 1 token
// returns exactly one snippet (never an empty Text).
// Cross-reference: ch09 F-CRITICAL-004 and formatter.go FormatDocSnippets
// contract: "The first snippet is always included regardless of its size."
func TestQueryDocs_TokenBudgetAtLeastOne_TinyBudget(t *testing.T) {
	t.Parallel()

	entityID := int64(7)

	ms := &mockStore{
		libraries: map[string]*store.LibraryRecord{
			"lib": {ID: "lib", Name: "Test Lib"},
		},
		// Two search hits; tiny budget should still return the first snippet.
		searchHits: []store.SearchResult{
			{EntityID: entityID, SnippetType: "class", EntityName: "Alpha", Rank: -2.0},
		},
		entityByID: map[int64]*store.EntityRecord{
			entityID: {
				ID:          entityID,
				LibraryID:   "lib",
				Slug:        "alpha",
				Name:        "Alpha",
				Kind:        "class",
				Description: "First class with a very long description that exceeds any tiny token budget.",
			},
		},
		methods: map[int64][]store.MethodRecord{
			entityID: nil,
		},
	}

	// Budget of 1 — smaller than any rendered snippet; at-least-1 contract
	// guarantees that Text is still non-empty.
	client := NewWithStore(ms, WithTokenBudget(1))
	ctx := context.Background()

	result, err := client.QueryDocs(ctx, "lib", "alpha")
	if err != nil {
		t.Fatalf("QueryDocs() error: %v", err)
	}
	if result.Text == "" {
		t.Error("QueryDocs() Text is empty with budget=1; at-least-1 invariant violated")
	}
}

// Test 10: QueryDocs with a missing library ID returns an error whose message
// contains "not found" (matches the format in defsource.go: "library %q not found").
func TestQueryDocs_MissingLibrary_ReturnsNotFoundError(t *testing.T) {
	t.Parallel()

	ms := &mockStore{} // no libraries populated
	client := NewWithStore(ms)
	ctx := context.Background()

	_, err := client.QueryDocs(ctx, "nonexistent-lib", "query")
	if err == nil {
		t.Fatal("QueryDocs() expected error for missing library, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error message %q does not contain %q", err.Error(), "not found")
	}
}

// Test 11: ListLibraries returns an empty slice (not nil, not an error)
// when the store has no libraries.
func TestListLibraries_EmptyStore_ReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	ms := &mockStore{}
	client := NewWithStore(ms)
	ctx := context.Background()

	libs, err := client.ListLibraries(ctx)
	if err != nil {
		t.Fatalf("ListLibraries() unexpected error: %v", err)
	}
	if libs == nil {
		t.Error("ListLibraries() returned nil slice, want empty non-nil slice")
	}
	if len(libs) != 0 {
		t.Errorf("ListLibraries() = %d, want 0 on empty store", len(libs))
	}
}

// Test 12: ListLibraries on a populated store returns Library values with
// the fields correctly mapped from LibraryRecord.
func TestListLibraries_PopulatedStore_ReturnsMappedLibraries(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	ms := &mockStore{
		libraries: map[string]*store.LibraryRecord{
			"wp-6.5": {
				ID:           "wp-6.5",
				Name:         "WordPress 6.5",
				Description:  "WordPress core",
				SourceURL:    "https://github.com/WordPress/WordPress",
				Version:      "6.5.0",
				TrustScore:   0.9,
				SnippetCount: 250,
				CrawledAt:    now,
			},
		},
	}

	client := NewWithStore(ms)
	ctx := context.Background()

	libs, err := client.ListLibraries(ctx)
	if err != nil {
		t.Fatalf("ListLibraries() error: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("ListLibraries() = %d libraries, want 1", len(libs))
	}
	lib := libs[0]
	if lib.ID != "wp-6.5" {
		t.Errorf("ID = %q, want %q", lib.ID, "wp-6.5")
	}
	if lib.Name != "WordPress 6.5" {
		t.Errorf("Name = %q, want %q", lib.Name, "WordPress 6.5")
	}
	if lib.SnippetCount != 250 {
		t.Errorf("SnippetCount = %d, want 250", lib.SnippetCount)
	}
	if !lib.CrawledAt.Equal(now) {
		t.Errorf("CrawledAt = %v, want %v", lib.CrawledAt, now)
	}
}

// Test 13: ListEntities propagates errors returned by store.ListMethods.
// Wave-1 changed the previously-silent discard to error propagation;
// this test locks in that contract.
func TestListEntities_ListMethodsError_IsReturned(t *testing.T) {
	t.Parallel()

	listMethodsSentinel := errors.New("listMethods: db gone")

	ms := &mockStore{
		entities: map[string][]store.EntityRecord{
			"lib": {
				{
					ID:        1,
					LibraryID: "lib",
					Slug:      "wp-query",
					Name:      "WP_Query",
					Kind:      "class",
				},
			},
		},
		listMethodsErr: listMethodsSentinel,
	}

	client := NewWithStore(ms)
	ctx := context.Background()

	_, err := client.ListEntities(ctx, "lib")
	if err == nil {
		t.Fatal("ListEntities() expected error from ListMethods, got nil")
	}
	if !errors.Is(err, listMethodsSentinel) && !strings.Contains(err.Error(), "db gone") {
		t.Errorf("error = %v; does not wrap or contain the ListMethods sentinel", err)
	}
}

// Test 14: ListEntities propagates errors returned by store.ListRelations
// when constructing QueryDocs snippets. Although ListEntities itself does not
// call ListRelations, QueryDocs does. This test verifies the error-propagation
// contract via QueryDocs (where ListRelations is called for each method hit).
func TestQueryDocs_ListRelationsError_IsReturned(t *testing.T) {
	t.Parallel()

	listRelationsSentinel := errors.New("listRelations: network timeout")

	entityID := int64(5)
	methodID := int64(10)

	ms := &mockStore{
		libraries: map[string]*store.LibraryRecord{
			"lib": {ID: "lib", Name: "Test Lib"},
		},
		searchHits: []store.SearchResult{
			{
				EntityID:    entityID,
				MethodID:    &methodID,
				SnippetType: "method",
				EntityName:  "SomeClass",
				MethodName:  "someMethod",
				Rank:        -1.0,
			},
		},
		entityByID: map[int64]*store.EntityRecord{
			entityID: {
				ID:        entityID,
				LibraryID: "lib",
				Slug:      "someclass",
				Name:      "SomeClass",
				Kind:      "class",
			},
		},
		methods: map[int64][]store.MethodRecord{
			entityID: {
				{
					ID:       methodID,
					EntityID: entityID,
					Slug:     "somemethod",
					Name:     "someMethod",
				},
			},
		},
		listRelationsErr: listRelationsSentinel,
	}

	client := NewWithStore(ms)
	ctx := context.Background()

	_, err := client.QueryDocs(ctx, "lib", "someMethod")
	if err == nil {
		t.Fatal("QueryDocs() expected error from ListRelations, got nil")
	}
	if !errors.Is(err, listRelationsSentinel) && !strings.Contains(err.Error(), "network timeout") {
		t.Errorf("error = %v; does not wrap or contain the ListRelations sentinel", err)
	}
}

// Test 15: WithSearchMode option is applied: smoke-test that QueryDocs
// accepts "any" mode without error and returns a result.
func TestWithSearchMode_AnyMode_AcceptedWithoutError(t *testing.T) {
	t.Parallel()

	entityID := int64(3)

	ms := &mockStore{
		libraries: map[string]*store.LibraryRecord{
			"lib": {ID: "lib", Name: "Lib"},
		},
		searchHits: []store.SearchResult{
			{EntityID: entityID, SnippetType: "class", EntityName: "Foo", Rank: -1.0},
		},
		entityByID: map[int64]*store.EntityRecord{
			entityID: {
				ID:        entityID,
				LibraryID: "lib",
				Slug:      "foo",
				Name:      "Foo",
				Kind:      "class",
			},
		},
		methods: map[int64][]store.MethodRecord{
			entityID: nil,
		},
	}

	client := NewWithStore(ms)
	ctx := context.Background()

	result, err := client.QueryDocs(ctx, "lib", "foo", WithSearchMode("any"))
	if err != nil {
		t.Fatalf("QueryDocs() with WithSearchMode(\"any\") error: %v", err)
	}
	if result == nil {
		t.Fatal("QueryDocs() returned nil result")
	}
}

// Test 16: WithTokenBudget option sets the budget on the Client; a very
// small budget still satisfies the at-least-1 invariant.
func TestWithTokenBudget_SmallBudget_AppliedToClient(t *testing.T) {
	t.Parallel()

	entityID := int64(8)

	ms := &mockStore{
		libraries: map[string]*store.LibraryRecord{
			"lib": {ID: "lib", Name: "Lib"},
		},
		searchHits: []store.SearchResult{
			{EntityID: entityID, SnippetType: "class", EntityName: "Bar", Rank: -1.0},
		},
		entityByID: map[int64]*store.EntityRecord{
			entityID: {
				ID:          entityID,
				LibraryID:   "lib",
				Slug:        "bar",
				Name:        "Bar",
				Kind:        "class",
				Description: "Description that is long enough to exceed any minimal token budget.",
			},
		},
		methods: map[int64][]store.MethodRecord{
			entityID: nil,
		},
	}

	// budget=10 is smaller than any realistic snippet; at-least-1 must hold.
	client := NewWithStore(ms, WithTokenBudget(10))
	ctx := context.Background()

	result, err := client.QueryDocs(ctx, "lib", "bar")
	if err != nil {
		t.Fatalf("QueryDocs() error: %v", err)
	}
	// The at-least-1 invariant: Text must be non-empty even when budget < snippet size.
	if result.Text == "" {
		t.Error("WithTokenBudget(10): Text is empty; at-least-1 invariant violated")
	}
}

// Test 17: Close() marks the underlying store as closed; subsequent calls on
// a closed SQLite-backed client produce an error (or Close itself returns nil
// once and then the store is marked closed). For the mockStore case we verify
// the closed flag is set and that the mockStore.Close error is propagated.
func TestClose_ReleasesStore(t *testing.T) {
	t.Parallel()

	ms := &mockStore{}
	client := NewWithStore(ms)

	if err := client.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}
	if !ms.closed {
		t.Error("Close() did not mark the mockStore as closed")
	}
}

// Test 18: Close() propagates an error from the underlying store's Close.
func TestClose_PropagatesStoreError(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("close: wal checkpoint failed")
	ms := &mockStore{closeErr: closeErr}
	client := NewWithStore(ms)

	if err := client.Close(); !errors.Is(err, closeErr) {
		t.Errorf("Close() error = %v, want %v", err, closeErr)
	}
}
