//go:build sqlite_fts5 || fts5

package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/hatlesswizard/defsource/internal/store"
)

func (s *SQLiteStore) RebuildIndex(ctx context.Context, libraryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing index entries for this library
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM search_index WHERE rowid IN (
			SELECT sim.rowid FROM search_index_map sim
			JOIN entities e ON sim.entity_id = e.id
			WHERE e.library_id = ?
		)`, libraryID); err != nil {
		return fmt.Errorf("delete search index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM search_index_map WHERE entity_id IN (
			SELECT id FROM entities WHERE library_id = ?
		)`, libraryID); err != nil {
		return fmt.Errorf("delete search index map: %w", err)
	}

	// Index entity-level entries
	rows, err := tx.QueryContext(ctx, `
		SELECT id, name, description FROM entities WHERE library_id = ?`, libraryID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var entityID int64
		var name, description string
		if err := rows.Scan(&entityID, &name, &description); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO search_index (library_id, entity_name, method_name, content)
			VALUES (?, ?, '', ?)`, libraryID, name, description)
		if err != nil {
			return err
		}
		rowid, _ := res.LastInsertId()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO search_index_map (rowid, entity_id, method_id, snippet_type)
			VALUES (?, ?, NULL, 'class')`, rowid, entityID); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	// Index method-level entries
	mrows, err := tx.QueryContext(ctx, `
		SELECT m.id, m.entity_id, e.name, m.name, m.description, m.signature
		FROM methods m
		JOIN entities e ON m.entity_id = e.id
		WHERE e.library_id = ?`, libraryID)
	if err != nil {
		return err
	}
	defer mrows.Close()

	for mrows.Next() {
		var methodID, entityID int64
		var entityName, methodName, description, signature string
		if err := mrows.Scan(&methodID, &entityID, &entityName, &methodName, &description, &signature); err != nil {
			return err
		}
		content := description
		if signature != "" {
			content = signature + " " + description
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO search_index (library_id, entity_name, method_name, content)
			VALUES (?, ?, ?, ?)`, libraryID, entityName, methodName, content)
		if err != nil {
			return err
		}
		rowid, _ := res.LastInsertId()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO search_index_map (rowid, entity_id, method_id, snippet_type)
			VALUES (?, ?, ?, 'method')`, rowid, entityID, methodID); err != nil {
			return err
		}
	}
	if err := mrows.Err(); err != nil {
		return fmt.Errorf("iterate methods: %w", err)
	}
	return tx.Commit()
}

func (s *SQLiteStore) Search(ctx context.Context, libraryID, query string, limit int, mode string) ([]store.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	ftsQuery := sanitizeFTSQuery(query, mode)
	if ftsQuery == "" {
		return nil, nil
	}

	// BM25 returns negative values; lower (more negative) = better match.
	// ORDER BY rank ASC returns best matches first.
	// Weights: library_id=1, entity_name=5, method_name=10, content=15
	rows, err := s.db.QueryContext(ctx, `
		SELECT sim.entity_id, sim.method_id, sim.snippet_type,
		       si.entity_name, si.method_name,
		       bm25(search_index, 1, 5, 10, 15) AS rank
		FROM search_index si
		JOIN search_index_map sim ON si.rowid = sim.rowid
		WHERE search_index MATCH ? AND si.library_id = ?
		ORDER BY rank
		LIMIT ?`, ftsQuery, libraryID, limit)
	if err != nil {
		return nil, fmt.Errorf("fts5 search: %w", err)
	}
	defer rows.Close()

	var results []store.SearchResult
	for rows.Next() {
		var r store.SearchResult
		var methodID *int64
		if err := rows.Scan(&r.EntityID, &methodID, &r.SnippetType,
			&r.EntityName, &r.MethodName, &r.Rank); err != nil {
			return nil, err
		}
		r.MethodID = methodID
		results = append(results, r)
	}
	return results, rows.Err()
}

// sanitizeFTSQuery escapes special FTS5 characters, quotes each word, and joins
// with AND (space) or OR depending on mode. mode "any" uses OR; everything else
// (including "" and "all") uses AND (implicit FTS5 default).
func sanitizeFTSQuery(query string, mode string) string {
	// FTS5 special characters that need escaping
	replacer := strings.NewReplacer(
		`"`, `""`,
		`*`, ` `,
		`(`, ` `,
		`)`, ` `,
		`:`, ` `,
		`^`, ` `,
		`{`, ` `,
		`}`, ` `,
	)

	cleaned := replacer.Replace(query)
	words := strings.Fields(cleaned)
	if len(words) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(words))
	for _, w := range words {
		// Strip leading '-' (FTS5 NOT prefix)
		w = strings.TrimLeft(w, "-")
		if w == "" {
			continue
		}
		// Quoting each word handles FTS5 keywords (AND, OR, NOT, NEAR) as literals
		quoted = append(quoted, `"`+w+`"`)
	}
	if len(quoted) == 0 {
		return ""
	}

	joinOp := " " // AND (FTS5 implicit default)
	if mode == "any" {
		joinOp = " OR "
	}
	return strings.Join(quoted, joinOp)
}
