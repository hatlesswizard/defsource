//go:build sqlite_fts5 || fts5

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/store"
)

// escapeLike escapes LIKE pattern metacharacters so they match literally.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return s
}

// SQLiteStore implements store.Store backed by SQLite with FTS5.
type SQLiteStore struct {
	db *sql.DB
	mu sync.Mutex // serialize writes for concurrent crawler workers
}

// New opens (or creates) a SQLite database at dbPath and runs migrations.
func New(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_fk=1")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	// Verify the database is writable and not corrupt
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS _health_check (v INTEGER); DROP TABLE _health_check;")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("database is read-only or corrupt: %w", err)
	}
	if _, err := db.Exec(migrationV1); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations v1: %w", err)
	}
	if _, err := db.Exec(migrationV2); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations v2: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) UpsertLibrary(ctx context.Context, id string, meta source.LibraryMeta) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO libraries (id, name, description, source_url, version, trust_score, crawled_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			source_url = excluded.source_url,
			version = excluded.version,
			trust_score = excluded.trust_score,
			crawled_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP`,
		id, meta.Name, meta.Description, meta.SourceURL, meta.Version, meta.TrustScore)
	return err
}

func (s *SQLiteStore) GetLibrary(ctx context.Context, id string) (*store.LibraryRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, source_url, version, trust_score, snippet_count,
		       COALESCE(crawled_at, ''), created_at, updated_at
		FROM libraries WHERE id = ?`, id)
	var rec store.LibraryRecord
	var crawledAt string
	err := row.Scan(&rec.ID, &rec.Name, &rec.Description, &rec.SourceURL,
		&rec.Version, &rec.TrustScore, &rec.SnippetCount,
		&crawledAt, &rec.CreatedAt, &rec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if crawledAt != "" {
		rec.CrawledAt, _ = time.Parse("2006-01-02 15:04:05", crawledAt)
	}
	return &rec, nil
}

func (s *SQLiteStore) SearchLibraries(ctx context.Context, query string) ([]store.LibraryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, source_url, version, trust_score, snippet_count,
		       COALESCE(crawled_at, ''), created_at, updated_at
		FROM libraries
		WHERE name LIKE ? ESCAPE '\' OR description LIKE ? ESCAPE '\' OR id LIKE ? ESCAPE '\'
		ORDER BY trust_score DESC`,
		"%"+escapeLike(query)+"%", "%"+escapeLike(query)+"%", "%"+escapeLike(query)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLibraries(rows)
}

func (s *SQLiteStore) ListLibraries(ctx context.Context) ([]store.LibraryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, source_url, version, trust_score, snippet_count,
		       COALESCE(crawled_at, ''), created_at, updated_at
		FROM libraries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLibraries(rows)
}

func scanLibraries(rows *sql.Rows) ([]store.LibraryRecord, error) {
	var results []store.LibraryRecord
	for rows.Next() {
		var rec store.LibraryRecord
		var crawledAt string
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Description, &rec.SourceURL,
			&rec.Version, &rec.TrustScore, &rec.SnippetCount,
			&crawledAt, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		if crawledAt != "" {
			rec.CrawledAt, _ = time.Parse("2006-01-02 15:04:05", crawledAt)
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) UpsertEntity(ctx context.Context, libraryID string, entity *source.Entity) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO entities (library_id, slug, name, kind, description, source_file, source_code, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(library_id, slug) DO UPDATE SET
			name = excluded.name,
			kind = excluded.kind,
			description = excluded.description,
			source_file = excluded.source_file,
			source_code = excluded.source_code,
			url = excluded.url,
			updated_at = CURRENT_TIMESTAMP`,
		libraryID, entity.Slug, entity.Name, entity.Kind, entity.Description,
		entity.SourceFile, entity.SourceCode, entity.URL)
	if err != nil {
		return 0, err
	}

	// Always SELECT to reliably get the entity ID (LastInsertId is unreliable for upserts)
	var entityID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM entities WHERE library_id = ? AND slug = ?",
		libraryID, entity.Slug).Scan(&entityID)
	if err != nil {
		return 0, err
	}

	// Replace properties
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM properties WHERE entity_id = ?`, entityID); err != nil {
		return 0, err
	}
	for _, p := range entity.Properties {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO properties (entity_id, name, type, description, visibility, since)
			VALUES (?, ?, ?, ?, ?, ?)`,
			entityID, p.Name, p.Type, p.Description, p.Visibility, p.Since); err != nil {
			return 0, err
		}
	}

	return entityID, tx.Commit()
}

func (s *SQLiteStore) GetEntity(ctx context.Context, libraryID, slug string) (*store.EntityRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, library_id, slug, name, kind, description, source_file, source_code, url,
		       created_at, updated_at
		FROM entities WHERE library_id = ? AND slug = ?`, libraryID, slug)
	var rec store.EntityRecord
	err := row.Scan(&rec.ID, &rec.LibraryID, &rec.Slug, &rec.Name, &rec.Kind,
		&rec.Description, &rec.SourceFile, &rec.SourceCode, &rec.URL,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *SQLiteStore) GetEntityByID(ctx context.Context, id int64) (*store.EntityRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, library_id, slug, name, kind, description, source_file, source_code, url,
		       created_at, updated_at
		FROM entities WHERE id = ?`, id)
	var rec store.EntityRecord
	err := row.Scan(&rec.ID, &rec.LibraryID, &rec.Slug, &rec.Name, &rec.Kind,
		&rec.Description, &rec.SourceFile, &rec.SourceCode, &rec.URL,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *SQLiteStore) ListEntities(ctx context.Context, libraryID string) ([]store.EntityRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, library_id, slug, name, kind, description, source_file, source_code, url,
		       created_at, updated_at
		FROM entities WHERE library_id = ? ORDER BY name`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []store.EntityRecord
	for rows.Next() {
		var rec store.EntityRecord
		if err := rows.Scan(&rec.ID, &rec.LibraryID, &rec.Slug, &rec.Name, &rec.Kind,
			&rec.Description, &rec.SourceFile, &rec.SourceCode, &rec.URL,
			&rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) UpsertMethod(ctx context.Context, entityID int64, method *source.Method) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM entities WHERE id = ?)", entityID).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("entity %d not found: FOREIGN KEY would fail", entityID)
	}

	params := method.Parameters
	if params == nil {
		params = []source.Parameter{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal parameters: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO methods (entity_id, slug, name, signature, description, parameters_json,
		                     return_type, return_desc, source_code, wrapped_source, wrapped_method,
		                     url, since)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_id, slug) DO UPDATE SET
			name = excluded.name,
			signature = excluded.signature,
			description = excluded.description,
			parameters_json = excluded.parameters_json,
			return_type = excluded.return_type,
			return_desc = excluded.return_desc,
			source_code = excluded.source_code,
			wrapped_source = excluded.wrapped_source,
			wrapped_method = excluded.wrapped_method,
			url = excluded.url,
			since = excluded.since,
			updated_at = CURRENT_TIMESTAMP`,
		entityID, method.Slug, method.Name, method.Signature, method.Description,
		string(paramsJSON), method.ReturnType, method.ReturnDesc, method.SourceCode,
		method.WrappedSource, method.WrappedMethod, method.URL, method.Since)
	if err != nil {
		return err
	}

	// Get the method ID
	methodID, err := res.LastInsertId()
	if err != nil || methodID == 0 {
		row := tx.QueryRowContext(ctx,
			`SELECT id FROM methods WHERE entity_id = ? AND slug = ?`,
			entityID, method.Slug)
		if err := row.Scan(&methodID); err != nil {
			return err
		}
	}

	// Replace relations
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM relations WHERE method_id = ?`, methodID); err != nil {
		return err
	}
	for _, r := range method.Relations {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO relations (method_id, kind, target_name, target_url, description)
			VALUES (?, ?, ?, ?, ?)`,
			methodID, r.Kind, r.TargetName, r.TargetURL, r.Description); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetMethod(ctx context.Context, entityID int64, slug string) (*store.MethodRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, entity_id, slug, name, signature, description, parameters_json,
		       return_type, return_desc, source_code, wrapped_source, wrapped_method,
		       url, since, created_at, updated_at
		FROM methods WHERE entity_id = ? AND slug = ?`, entityID, slug)
	var rec store.MethodRecord
	err := row.Scan(&rec.ID, &rec.EntityID, &rec.Slug, &rec.Name, &rec.Signature,
		&rec.Description, &rec.ParametersJSON, &rec.ReturnType, &rec.ReturnDesc,
		&rec.SourceCode, &rec.WrappedSource, &rec.WrappedMethod,
		&rec.URL, &rec.Since, &rec.CreatedAt, &rec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *SQLiteStore) ListMethods(ctx context.Context, entityID int64) ([]store.MethodRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entity_id, slug, name, signature, description, parameters_json,
		       return_type, return_desc, source_code, wrapped_source, wrapped_method,
		       url, since, created_at, updated_at
		FROM methods WHERE entity_id = ? ORDER BY name`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []store.MethodRecord
	for rows.Next() {
		var rec store.MethodRecord
		if err := rows.Scan(&rec.ID, &rec.EntityID, &rec.Slug, &rec.Name, &rec.Signature,
			&rec.Description, &rec.ParametersJSON, &rec.ReturnType, &rec.ReturnDesc,
			&rec.SourceCode, &rec.WrappedSource, &rec.WrappedMethod,
			&rec.URL, &rec.Since, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) ListRelations(ctx context.Context, methodID int64) ([]store.RelationRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, method_id, kind, target_name, target_url, description
		FROM relations WHERE method_id = ? ORDER BY kind, target_name`, methodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []store.RelationRecord
	for rows.Next() {
		var rec store.RelationRecord
		if err := rows.Scan(&rec.ID, &rec.MethodID, &rec.Kind, &rec.TargetName,
			&rec.TargetURL, &rec.Description); err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) UpdateSnippetCount(ctx context.Context, libraryID string, count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		UPDATE libraries SET snippet_count = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, count, libraryID)
	return err
}

func (s *SQLiteStore) ComputeSnippetCount(ctx context.Context, libraryID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT (SELECT COUNT(*) FROM entities WHERE library_id = ?) +
		       (SELECT COUNT(*) FROM methods m JOIN entities e ON m.entity_id = e.id WHERE e.library_id = ?)
	`, libraryID, libraryID).Scan(&count)
	return count, err
}

func (s *SQLiteStore) CreateCrawlSession(ctx context.Context, libraryID string, totalURLs int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO crawl_sessions (library_id, total_urls)
		VALUES (?, ?)`, libraryID, totalURLs)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) UpdateCrawlSession(ctx context.Context, sessionID int64, status string, success, fail, skip int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var completedAt interface{}
	if status == "completed" || status == "interrupted" {
		completedAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE crawl_sessions
		SET status = ?, success_count = ?, fail_count = ?, skip_count = ?, completed_at = ?
		WHERE id = ?`, status, success, fail, skip, completedAt, sessionID)
	return err
}

func (s *SQLiteStore) GetLastSession(ctx context.Context, libraryID string) (*store.CrawlSession, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, library_id, status, total_urls, success_count, fail_count, skip_count,
		       started_at, completed_at
		FROM crawl_sessions
		WHERE library_id = ?
		ORDER BY id DESC LIMIT 1`, libraryID)

	var rec store.CrawlSession
	var completedAt sql.NullTime
	err := row.Scan(&rec.ID, &rec.LibraryID, &rec.Status, &rec.TotalURLs,
		&rec.SuccessCount, &rec.FailCount, &rec.SkipCount,
		&rec.StartedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		rec.CompletedAt = &completedAt.Time
	}
	return &rec, nil
}

func (s *SQLiteStore) RecordProgress(ctx context.Context, sessionID int64, item *store.CrawlProgressItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO crawl_progress (session_id, url, item_type, status, error_type, error_message, parent_entity, processed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(session_id, url) DO UPDATE SET
			status = excluded.status,
			error_type = excluded.error_type,
			error_message = excluded.error_message,
			processed_at = CURRENT_TIMESTAMP`,
		sessionID, item.URL, item.ItemType, item.Status, item.ErrorType, item.ErrorMessage, item.ParentEntity)
	return err
}

func (s *SQLiteStore) GetProcessedURLs(ctx context.Context, sessionID int64) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT url, status FROM crawl_progress
		WHERE session_id = ? AND status = 'success'`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var url, status string
		if err := rows.Scan(&url, &status); err != nil {
			return nil, err
		}
		result[url] = status
	}
	return result, rows.Err()
}

func (s *SQLiteStore) GetCrawlStats(ctx context.Context, sessionID int64) (*store.CrawlStats, error) {
	stats := &store.CrawlStats{
		FailuresByType: make(map[string]int),
	}

	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN status = 'skipped' THEN 1 ELSE 0 END)
		FROM crawl_progress WHERE session_id = ?`, sessionID).
		Scan(&stats.Total, &stats.Success, &stats.Failed, &stats.Skipped)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(error_type, 'unknown'), COUNT(*)
		FROM crawl_progress
		WHERE session_id = ? AND status = 'failed'
		GROUP BY error_type`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var errType string
		var count int
		if err := rows.Scan(&errType, &count); err != nil {
			return nil, err
		}
		stats.FailuresByType[errType] = count
	}
	return stats, rows.Err()
}

func (s *SQLiteStore) GetFailures(ctx context.Context, sessionID int64) ([]store.CrawlProgressItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT url, item_type, status, COALESCE(error_type, ''), COALESCE(error_message, ''), COALESCE(parent_entity, '')
		FROM crawl_progress
		WHERE session_id = ? AND status = 'failed'`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []store.CrawlProgressItem
	for rows.Next() {
		var item store.CrawlProgressItem
		if err := rows.Scan(&item.URL, &item.ItemType, &item.Status,
			&item.ErrorType, &item.ErrorMessage, &item.ParentEntity); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}
