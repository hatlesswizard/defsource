// Package store defines the persistence interface and data-transfer types for
// documentation storage. The Store interface is the single boundary between the
// crawler / query layers and any concrete storage backend. The only current
// implementation is internal/store/sqlite.SQLiteStore, which is backed by a
// SQLite database with FTS5 full-text search.
package store

import (
	"context"
	"errors"

	"github.com/hatlesswizard/defsource/internal/source"
)

// ErrNotFound is returned by Get* methods when the requested entity does not
// exist in the store. Callers should use errors.Is(err, store.ErrNotFound) to
// distinguish a missing record from a real database error.
var ErrNotFound = errors.New("store: not found")

// ErrFKConstraint is returned when an upsert violates a foreign-key constraint,
// for example when UpsertMethod is called with an entityID that does not exist.
var ErrFKConstraint = errors.New("store: foreign key constraint")

// LibraryStore handles library registration and lookup. Consumers that only
// need library-level operations (e.g., the HTTP search endpoint) may declare
// their dependency as LibraryStore rather than the full Store interface.
type LibraryStore interface {
	// UpsertLibrary inserts or updates a library record identified by id, using
	// the fields from meta. If the library already exists its metadata is refreshed.
	UpsertLibrary(ctx context.Context, id string, meta source.LibraryMeta) error

	// GetLibrary returns the library record for the given id, or (nil, ErrNotFound)
	// when no such library exists.
	GetLibrary(ctx context.Context, id string) (*LibraryRecord, error)

	// SearchLibraries performs a case-insensitive substring search on library
	// names and returns matching records ranked by relevance.
	SearchLibraries(ctx context.Context, query string) ([]LibraryRecord, error)

	// ListLibraries returns all library records in the store.
	ListLibraries(ctx context.Context) ([]LibraryRecord, error)
}

// EntityStore handles entity and method persistence. This is the primary write
// interface for the crawler worker pool.
type EntityStore interface {
	// UpsertEntity inserts or replaces an entity (class, function, etc.) within
	// the given library. Properties are replaced atomically inside a transaction.
	// Returns the row ID of the upserted entity.
	UpsertEntity(ctx context.Context, libraryID string, entity *source.Entity) (int64, error)

	// GetEntity returns the entity with the given slug inside the specified
	// library, or (nil, ErrNotFound) when no such entity exists.
	GetEntity(ctx context.Context, libraryID, slug string) (*EntityRecord, error)

	// GetEntityByID returns the entity with the given row ID, or (nil, ErrNotFound)
	// when no such entity exists.
	GetEntityByID(ctx context.Context, id int64) (*EntityRecord, error)

	// ListEntities returns all entities belonging to the given library.
	ListEntities(ctx context.Context, libraryID string) ([]EntityRecord, error)

	// UpdateSnippetCount sets the pre-computed snippet count on the library record.
	// The authoritative final count is written once after a full crawl completes.
	UpdateSnippetCount(ctx context.Context, libraryID string, count int) error

	// ComputeSnippetCount calculates the total number of searchable snippets for
	// the library by aggregating entity and method counts in the database.
	// Returns the count without modifying the stored snippet_count field.
	ComputeSnippetCount(ctx context.Context, libraryID string) (int, error)
}

// MethodStore handles method and relation persistence.
type MethodStore interface {
	// UpsertMethod inserts or replaces a method record under the given entity.
	// Relations are replaced atomically inside a transaction.
	UpsertMethod(ctx context.Context, entityID int64, method *source.Method) error

	// GetMethod returns the method with the given slug under the specified entity,
	// or (nil, ErrNotFound) when no such method exists.
	GetMethod(ctx context.Context, entityID int64, slug string) (*MethodRecord, error)

	// ListMethods returns all methods belonging to the given entity.
	ListMethods(ctx context.Context, entityID int64) ([]MethodRecord, error)

	// ListRelations returns all cross-reference relations for the given method.
	ListRelations(ctx context.Context, methodID int64) ([]RelationRecord, error)
}

// SearchStore handles full-text search index management and query execution.
type SearchStore interface {
	// Search executes a BM25-ranked FTS5 query against the search index for the
	// given library. limit caps returned results. mode is "all" (AND semantics,
	// default) or "any" (OR semantics). Returns nil, not an empty slice, when
	// the query is empty or produces no hits.
	Search(ctx context.Context, libraryID, query string, limit int, mode string) ([]SearchResult, error)

	// RebuildIndex rebuilds the FTS5 search index for the given library from
	// scratch. This is a potentially long-running operation that acquires the
	// write lock for the duration of the full transaction.
	RebuildIndex(ctx context.Context, libraryID string) error
}

// SessionStore handles crawl session audit records. Consumers that only perform
// resume/retry operations may declare their dependency as SessionStore.
type SessionStore interface {
	// CreateCrawlSession opens a new crawl session for the library and returns
	// its session ID. totalURLs is the count of entity URLs discovered.
	CreateCrawlSession(ctx context.Context, libraryID string, totalURLs int) (int64, error)

	// UpdateCrawlSession updates the status and counters for an existing session.
	// status should be one of "running", "completed", or "interrupted".
	UpdateCrawlSession(ctx context.Context, sessionID int64, status string, success, fail, skip int) error

	// GetLastSession returns the most recent crawl session for the library, or
	// (nil, ErrNotFound) when no session exists.
	GetLastSession(ctx context.Context, libraryID string) (*CrawlSession, error)

	// RecordProgress records a single URL's crawl outcome in the session log.
	RecordProgress(ctx context.Context, sessionID int64, item *CrawlProgressItem) error

	// GetProcessedURLs returns a map of URL → status for all URLs recorded in
	// the given session. Used to skip already-processed URLs on resume.
	GetProcessedURLs(ctx context.Context, sessionID int64) (map[string]string, error)

	// GetCrawlStats returns aggregate success/failure/skip counts for a session.
	GetCrawlStats(ctx context.Context, sessionID int64) (*CrawlStats, error)

	// GetFailures returns all failed crawl progress items for a session. Used by
	// the --retry-failed crawl mode.
	GetFailures(ctx context.Context, sessionID int64) ([]CrawlProgressItem, error)
}

// Store is the unified persistence interface for documentation data. It composes
// all focused sub-interfaces so that a single injection point serves both the
// crawler and the query client. Consumers that need only a subset of operations
// should declare their dependency on the appropriate sub-interface:
//
//   - LibraryStore  — library registration and lookup
//   - EntityStore   — entity and method persistence
//   - MethodStore   — method and relation persistence
//   - SearchStore   — FTS5 index management and query execution
//   - SessionStore  — crawl session audit records
//
// Implementations are responsible for their own thread safety. The SQLiteStore
// implementation serialises all writes behind a sync.Mutex while allowing
// concurrent reads via SQLite WAL mode; callers may invoke any method from
// multiple goroutines simultaneously.
type Store interface {
	LibraryStore
	EntityStore
	MethodStore
	SearchStore
	SessionStore

	// Close releases any resources held by the store (e.g., the database
	// connection pool). Must be called when the store is no longer needed.
	Close() error
}
