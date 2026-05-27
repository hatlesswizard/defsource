//go:build sqlite_fts5 || fts5

package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/hatlesswizard/defsource/internal/store"
)

// RebuildIndex acquires the global write mutex and rebuilds the entire FTS5
// search index for the given library inside a single transaction. All existing
// index rows for the library are deleted, then every entity and method is
// re-indexed. Because this holds s.mu for the full duration, it can be a
// lengthy operation on large libraries; callers should not invoke it on the
// worker hot path.
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
		rowid, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("get entity rowid after search_index insert: %w", err)
		}
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
		rowid, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("get method rowid after search_index insert: %w", err)
		}
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

// Search executes a full-text search against the FTS5 search_index for the
// given library. query is sanitized through sanitizeFTSQuery before being
// passed to the MATCH expression; an empty or all-special-character query
// returns (nil, nil). limit <= 0 defaults to 20. mode "any" joins terms with
// OR; any other value (including "all" and "") uses AND (FTS5 implicit
// default). Results are ordered by BM25 rank ascending (most relevant first).
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

// sanitizeFTSQuery is the sole defense against FTS5 injection. It applies an
// 8-rule strings.Replacer to user-supplied query text before that text is
// interpolated into a MATCH expression. The 8-rule contract:
//
//  1. `"` → `""` — doubles double-quotes so the word-quoting below does not
//     produce unbalanced quotes that break the FTS5 query parser.
//  2. `*` → ` ` — strips the prefix-match operator; unrestricted prefix
//     wildcards allow full-index scans and must not reach the parser.
//  3. `(` → ` ` — removes grouping operators that allow query restructuring.
//  4. `)` → ` ` — same reason as rule 3.
//  5. `:` → ` ` — removes the column-filter operator (e.g. `name:foo`) so
//     callers cannot target arbitrary FTS5 columns.
//  6. `^` → ` ` — strips the start-of-phrase anchor operator.
//  7. `{` → ` ` — removes table-valued-function start delimiters.
//  8. `}` → ` ` — removes table-valued-function end delimiters.
//
// After replacement, each whitespace-separated word is individually double-
// quoted. Quoting ensures FTS5 keywords (AND, OR, NOT, NEAR) are treated as
// literal search terms, not query operators. Leading `-` (NOT prefix) is
// stripped before quoting. Words that reduce to empty after stripping are
// dropped. If no words survive, the empty string is returned and the caller
// must not issue a MATCH query.
//
// mode "any" joins surviving words with " OR "; all other values use " "
// (AND, the FTS5 implicit default).
//
// SECURITY INVARIANT: Every character that SQLite FTS5 treats as a query
// operator must either appear in the 8-rule replacer or be neutralized by
// word-level double-quoting. If a new FTS5 operator is introduced by a future
// SQLite version, this function must be updated and TestSanitizeFTSQuery must
// be extended to cover it.
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

// RebuildIndexAsync launches a single background goroutine to rebuild the FTS5
// search index for libraryID. If a rebuild is already in flight for any library,
// this call is a no-op and returns immediately — the caller is notified by the
// returned bool (true = goroutine launched, false = already in flight, skipped).
//
// This is the preferred variant for the crawler worker hot-path (CRIT-08 fix).
// The actual rebuild still acquires s.mu and is therefore serialised with all
// other writes, but workers no longer block synchronously waiting for the rebuild
// to finish. The search index is eventually consistent during a crawl; it becomes
// fully consistent once RebuildIndex is called at the end of the crawl.
//
// The error from the background rebuild (if any) is silently discarded. If the
// caller needs guaranteed delivery, use RebuildIndex (the synchronous variant).
func (s *SQLiteStore) RebuildIndexAsync(ctx context.Context, libraryID string) bool {
	if !s.rebuildInFlight.CompareAndSwap(0, 1) {
		// Another rebuild goroutine is already running — skip to avoid piling up.
		return false
	}
	go func() {
		defer s.rebuildInFlight.Store(0)
		// Use a background context so the rebuild is not cancelled if the
		// triggering worker's context is cancelled before the goroutine runs.
		_ = s.RebuildIndex(context.Background(), libraryID)
	}()
	return true
}
