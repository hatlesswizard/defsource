package store

import (
	"context"

	"github.com/hatlesswizard/defsource/internal/source"
)

// Store defines the persistence interface for documentation data.
type Store interface {
	UpsertLibrary(ctx context.Context, id string, meta source.LibraryMeta) error
	GetLibrary(ctx context.Context, id string) (*LibraryRecord, error)
	SearchLibraries(ctx context.Context, query string) ([]LibraryRecord, error)
	ListLibraries(ctx context.Context) ([]LibraryRecord, error)

	UpsertEntity(ctx context.Context, libraryID string, entity *source.Entity) (int64, error)
	GetEntity(ctx context.Context, libraryID, slug string) (*EntityRecord, error)
	GetEntityByID(ctx context.Context, id int64) (*EntityRecord, error)
	ListEntities(ctx context.Context, libraryID string) ([]EntityRecord, error)

	UpsertMethod(ctx context.Context, entityID int64, method *source.Method) error
	GetMethod(ctx context.Context, entityID int64, slug string) (*MethodRecord, error)
	ListMethods(ctx context.Context, entityID int64) ([]MethodRecord, error)
	ListRelations(ctx context.Context, methodID int64) ([]RelationRecord, error)

	UpdateSnippetCount(ctx context.Context, libraryID string, count int) error
	ComputeSnippetCount(ctx context.Context, libraryID string) (int, error)
	Search(ctx context.Context, libraryID, query string, limit int, mode string) ([]SearchResult, error)
	RebuildIndex(ctx context.Context, libraryID string) error

	CreateCrawlSession(ctx context.Context, libraryID string, totalURLs int) (int64, error)
	UpdateCrawlSession(ctx context.Context, sessionID int64, status string, success, fail, skip int) error
	GetLastSession(ctx context.Context, libraryID string) (*CrawlSession, error)
	RecordProgress(ctx context.Context, sessionID int64, item *CrawlProgressItem) error
	GetProcessedURLs(ctx context.Context, sessionID int64) (map[string]string, error)
	GetCrawlStats(ctx context.Context, sessionID int64) (*CrawlStats, error)
	GetFailures(ctx context.Context, sessionID int64) ([]CrawlProgressItem, error)

	Close() error
}
