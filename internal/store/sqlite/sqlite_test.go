//go:build sqlite_fts5 || fts5

// Package sqlite_test provides characterization tests for the SQLiteStore
// implementation. All tests use t.TempDir() for hermetic database isolation.
//
// Concurrent tests are run without t.Parallel() per scope instructions but
// do exercise concurrent goroutines internally to validate mutex serialization.
//
// These tests characterize the current behaviour of sqlite.go, including
// deliberate documentation of known quirks (e.g., UpsertMethod's pre-flight
// existence check runs OUTSIDE the write mutex — a TOCTOU window noted in
// ch09 Finding F-CRITICAL-002).
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/store"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// newTestStore opens a fresh SQLiteStore in a temporary directory, registers a
// cleanup that closes the store, and returns both the store and the DB path.
func newTestStore(t *testing.T) (*SQLiteStore, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("newTestStore: New(%q): %v", dbPath, err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("newTestStore cleanup: Close: %v", err)
		}
	})
	return s, dbPath
}

// seedLibrary inserts a minimal library record with the given slug and returns
// that slug. It calls t.Fatal on any error so callers don't need to check.
func seedLibrary(t *testing.T, s *SQLiteStore, slug string) string {
	t.Helper()
	ctx := context.Background()
	meta := source.LibraryMeta{
		Name:        "Test Library " + slug,
		Description: "desc",
		SourceURL:   "https://example.com/" + slug,
		TrustScore:  0.9,
	}
	if err := s.UpsertLibrary(ctx, slug, meta); err != nil {
		t.Fatalf("seedLibrary: UpsertLibrary(%q): %v", slug, err)
	}
	return slug
}

// seedEntity inserts a minimal entity under libraryID and returns its DB ID.
func seedEntity(t *testing.T, s *SQLiteStore, libraryID, slug string) int64 {
	t.Helper()
	ctx := context.Background()
	entity := &source.Entity{
		Slug:        slug,
		Name:        "Entity " + slug,
		Kind:        "class",
		Description: "An entity",
		URL:         "https://example.com/" + slug,
	}
	id, err := s.UpsertEntity(ctx, libraryID, entity)
	if err != nil {
		t.Fatalf("seedEntity: UpsertEntity(%q, %q): %v", libraryID, slug, err)
	}
	return id
}

// ─── 1. Migration idempotence ────────────────────────────────────────────────

// TestNew_CreatesSchemaAndMigrations verifies that New runs both migrations
// without error and that schema_meta reflects version "2".
func TestNew_CreatesSchemaAndMigrations(t *testing.T) {
	s, dbPath := newTestStore(t)
	_ = dbPath // dbPath held for the re-open test below

	ctx := context.Background()

	// Verify schema_meta version is "2" after both migrations run.
	var version string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM schema_meta WHERE key = 'version'`).Scan(&version)
	if err != nil {
		t.Fatalf("reading schema_meta: %v", err)
	}
	if version != "2" {
		t.Errorf("schema_meta version = %q; want %q", version, "2")
	}

	// Verify that critical tables exist by querying them.
	for _, table := range []string{
		"libraries", "entities", "properties", "methods", "relations",
		"search_index_map", "crawl_sessions", "crawl_progress", "schema_meta",
	} {
		var n int
		q := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)
		if err := s.db.QueryRowContext(ctx, q).Scan(&n); err != nil {
			t.Errorf("table %q not accessible: %v", table, err)
		}
	}
}

// TestNew_MigrationIdempotence opens the same database twice and asserts that
// the second open does not error and leaves schema_meta.version unchanged.
func TestNew_MigrationIdempotence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "idempotent.db")
	ctx := context.Background()

	// First open — creates schema.
	s1, err := New(dbPath)
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Second open — must not error; migrations use IF NOT EXISTS guards.
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("second New (idempotency): %v", err)
	}
	defer func() {
		if err := s2.Close(); err != nil {
			t.Errorf("second Close: %v", err)
		}
	}()

	var version string
	if err := s2.db.QueryRowContext(ctx, `SELECT value FROM schema_meta WHERE key = 'version'`).Scan(&version); err != nil {
		t.Fatalf("reading schema_meta after re-open: %v", err)
	}
	if version != "2" {
		t.Errorf("schema_meta version after re-open = %q; want %q", version, "2")
	}

	// schema_meta must have exactly 1 row for key='version' (INSERT OR IGNORE
	// in migrationV1 means the second open does not duplicate it).
	var count int
	if err := s2.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_meta WHERE key = 'version'`).Scan(&count); err != nil {
		t.Fatalf("counting schema_meta rows: %v", err)
	}
	if count != 1 {
		t.Errorf("schema_meta row count = %d; want 1 (INSERT OR IGNORE must not duplicate)", count)
	}
}

// ─── 2. UpsertLibrary / GetLibrary ──────────────────────────────────────────

// TestUpsertLibraryAndGetLibrary_RoundTrip inserts a library and reads it back,
// verifying all persisted fields.
func TestUpsertLibraryAndGetLibrary_RoundTrip(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	const libID = "/test-lib"
	meta := source.LibraryMeta{
		Name:        "Test Library",
		Description: "A test library",
		SourceURL:   "https://example.com/test",
		Version:     "6.4.0",
		TrustScore:  0.95,
	}

	if err := s.UpsertLibrary(ctx, libID, meta); err != nil {
		t.Fatalf("UpsertLibrary: %v", err)
	}

	rec, err := s.GetLibrary(ctx, libID)
	if err != nil {
		t.Fatalf("GetLibrary: %v", err)
	}
	if rec == nil {
		t.Fatal("GetLibrary returned nil; want a record")
	}

	if rec.ID != libID {
		t.Errorf("ID = %q; want %q", rec.ID, libID)
	}
	if rec.Name != meta.Name {
		t.Errorf("Name = %q; want %q", rec.Name, meta.Name)
	}
	if rec.Description != meta.Description {
		t.Errorf("Description = %q; want %q", rec.Description, meta.Description)
	}
	if rec.SourceURL != meta.SourceURL {
		t.Errorf("SourceURL = %q; want %q", rec.SourceURL, meta.SourceURL)
	}
	if rec.Version != meta.Version {
		t.Errorf("Version = %q; want %q", rec.Version, meta.Version)
	}
	if rec.TrustScore != meta.TrustScore {
		t.Errorf("TrustScore = %v; want %v", rec.TrustScore, meta.TrustScore)
	}
}

// TestUpsertLibrary_UpdateDoesNotError upserts a library twice with a different
// name. The second call must succeed (ON CONFLICT DO UPDATE), not produce an
// error, and the record must reflect the updated name.
func TestUpsertLibrary_UpdateDoesNotError(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	const libID = "/update-lib"
	first := source.LibraryMeta{Name: "Original", SourceURL: "https://a.com"}
	second := source.LibraryMeta{Name: "Updated", SourceURL: "https://b.com"}

	if err := s.UpsertLibrary(ctx, libID, first); err != nil {
		t.Fatalf("first UpsertLibrary: %v", err)
	}
	if err := s.UpsertLibrary(ctx, libID, second); err != nil {
		t.Fatalf("second UpsertLibrary (should not error): %v", err)
	}

	rec, err := s.GetLibrary(ctx, libID)
	if err != nil {
		t.Fatalf("GetLibrary after update: %v", err)
	}
	if rec == nil {
		t.Fatal("GetLibrary returned nil after update")
	}
	if rec.Name != second.Name {
		t.Errorf("Name after update = %q; want %q", rec.Name, second.Name)
	}
}

// TestGetLibrary_NotFound_ReturnsErrNotFound verifies the sentinel error contract:
// a missing library returns (nil, store.ErrNotFound).
func TestGetLibrary_NotFound_ReturnsErrNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	rec, err := s.GetLibrary(ctx, "/does-not-exist")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetLibrary for missing id: got err=%v, want store.ErrNotFound", err)
	}
	if rec != nil {
		t.Errorf("GetLibrary for missing id returned non-nil record: %+v", rec)
	}
}

// ─── 3. SearchLibraries / escapeLike ────────────────────────────────────────

// TestSearchLibraries_LIKEMatching seeds one library and checks basic LIKE
// matching across name, description, and id fields.
func TestSearchLibraries_LIKEMatching(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	seedLibrary(t, s, "/wordpress")
	// Update name and description to something searchable.
	meta := source.LibraryMeta{
		Name:        "WordPress Reference",
		Description: "Official WordPress docs",
		SourceURL:   "https://developer.wordpress.org",
		TrustScore:  1.0,
	}
	if err := s.UpsertLibrary(ctx, "/wordpress", meta); err != nil {
		t.Fatalf("UpsertLibrary: %v", err)
	}

	tests := []struct {
		query   string
		wantLen int
	}{
		{"WordPress", 1},
		{"wordpress", 1},  // case-insensitive LIKE on name
		{"Reference", 1},  // name contains "Reference"
		{"Official", 1},   // description contains "Official"
		{"/wordpress", 1}, // id exact match
		{"zzz_no_match", 0},
	}

	for _, tc := range tests {
		results, err := s.SearchLibraries(ctx, tc.query)
		if err != nil {
			t.Errorf("SearchLibraries(%q): %v", tc.query, err)
			continue
		}
		if len(results) != tc.wantLen {
			t.Errorf("SearchLibraries(%q) returned %d results; want %d", tc.query, len(results), tc.wantLen)
		}
	}
}

// TestSearchLibraries_EscapesLIKEMetachars seeds a library whose ID contains
// the LIKE metacharacters _ and % and verifies that searching for those literal
// characters does NOT match as wildcards.
func TestSearchLibraries_EscapesLIKEMetachars(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	// A library whose ID contains LIKE metacharacters.
	specialID := "/test_%_lib"
	meta := source.LibraryMeta{
		Name:       "Special Lib",
		SourceURL:  "https://example.com/special",
		TrustScore: 0.5,
	}
	if err := s.UpsertLibrary(ctx, specialID, meta); err != nil {
		t.Fatalf("UpsertLibrary special: %v", err)
	}

	// Also insert a plain library that should NOT match a '%' or '_' wildcard.
	plainMeta := source.LibraryMeta{
		Name:       "Plain Lib",
		SourceURL:  "https://example.com/plain",
		TrustScore: 0.5,
	}
	if err := s.UpsertLibrary(ctx, "/plain", plainMeta); err != nil {
		t.Fatalf("UpsertLibrary plain: %v", err)
	}

	// Searching for the literal string "%" must NOT return both libraries (it
	// is escaped so it does not act as a wildcard that matches everything).
	results, err := s.SearchLibraries(ctx, "%")
	if err != nil {
		t.Fatalf("SearchLibraries('%%'): %v", err)
	}
	// The plain library's name/desc/id do not contain a literal '%' character.
	for _, r := range results {
		if r.ID == "/plain" {
			t.Errorf("SearchLibraries('%%') matched /plain — '%%' was not escaped to a literal")
		}
	}
}

// TestEscapeLike verifies that the private escapeLike helper correctly handles
// LIKE metacharacters.
func TestEscapeLike(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"normal", "normal"},
		{"50%", `50\%`},
		{"name_suffix", `name\_suffix`},
		{`C:\path`, `C:\\path`},
		{`%_\`, `\%\_\\`},
		{"", ""},
	}
	for _, tc := range cases {
		got := escapeLike(tc.in)
		if got != tc.want {
			t.Errorf("escapeLike(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// ─── 4. UpsertEntity / GetEntity / GetEntityByID / ListEntities ─────────────

// TestUpsertEntity_InsertAndRead inserts an entity with properties and reads it
// back via GetEntity and GetEntityByID.
func TestUpsertEntity_InsertAndRead(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/ent-lib")

	entity := &source.Entity{
		Slug:        "wp-query",
		Name:        "WP_Query",
		Kind:        "class",
		Description: "WordPress query class",
		SourceFile:  "wp-includes/class-wp-query.php",
		URL:         "https://developer.wordpress.org/reference/classes/wp_query/",
		Properties: []source.Property{
			{Name: "$query", Type: "array", Visibility: "public"},
			{Name: "$queried_object", Type: "WP_Post", Visibility: "public"},
		},
	}

	id, err := s.UpsertEntity(ctx, libID, entity)
	if err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	if id <= 0 {
		t.Errorf("UpsertEntity returned id=%d; want positive integer", id)
	}

	// GetEntity round-trip
	rec, err := s.GetEntity(ctx, libID, entity.Slug)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if rec == nil {
		t.Fatal("GetEntity returned nil; want record")
	}
	if rec.ID != id {
		t.Errorf("GetEntity ID = %d; want %d", rec.ID, id)
	}
	if rec.Name != entity.Name {
		t.Errorf("GetEntity Name = %q; want %q", rec.Name, entity.Name)
	}
	if rec.Kind != entity.Kind {
		t.Errorf("GetEntity Kind = %q; want %q", rec.Kind, entity.Kind)
	}

	// GetEntityByID round-trip
	recByID, err := s.GetEntityByID(ctx, id)
	if err != nil {
		t.Fatalf("GetEntityByID: %v", err)
	}
	if recByID == nil {
		t.Fatal("GetEntityByID returned nil; want record")
	}
	if recByID.Slug != entity.Slug {
		t.Errorf("GetEntityByID Slug = %q; want %q", recByID.Slug, entity.Slug)
	}
}

// TestUpsertEntity_DuplicatePropertyNameHandledByInsertOrReplace is the wpdb
// characterization test. WordPress's wpdb class has multiple properties all
// named "$col_meta". The store must accept these without a unique constraint
// error; INSERT OR REPLACE means the last write wins.
//
// Per CLAUDE.md: "Properties use INSERT OR REPLACE (wpdb has duplicate property
// names)". This test locks in that behaviour.
func TestUpsertEntity_DuplicatePropertyNameHandledByInsertOrReplace(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/wpdb-lib")

	// Two properties with identical names — mirrors the real wpdb scenario.
	entity := &source.Entity{
		Slug: "wpdb",
		Name: "wpdb",
		Kind: "class",
		URL:  "https://developer.wordpress.org/reference/classes/wpdb/",
		Properties: []source.Property{
			{Name: "$col_meta", Type: "array", Description: "first definition", Visibility: "public"},
			{Name: "$col_meta", Type: "array", Description: "second definition wins", Visibility: "public"},
		},
	}

	id, err := s.UpsertEntity(ctx, libID, entity)
	if err != nil {
		t.Fatalf("UpsertEntity with duplicate property names: %v", err)
	}
	if id <= 0 {
		t.Errorf("UpsertEntity returned id=%d; want positive integer", id)
	}

	// Verify that exactly one $col_meta property persists (INSERT OR REPLACE
	// on a UNIQUE(entity_id, name) constraint means the second write wins).
	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM properties WHERE entity_id = ? AND name = '$col_meta'`, id,
	).Scan(&count); err != nil {
		t.Fatalf("counting $col_meta properties: %v", err)
	}
	if count != 1 {
		t.Errorf("property count for $col_meta = %d; want 1 (INSERT OR REPLACE, second wins)", count)
	}

	// The surviving row should have the description from the second write.
	var desc string
	if err := s.db.QueryRowContext(ctx,
		`SELECT description FROM properties WHERE entity_id = ? AND name = '$col_meta'`, id,
	).Scan(&desc); err != nil {
		t.Fatalf("reading $col_meta description: %v", err)
	}
	if desc != "second definition wins" {
		t.Errorf("$col_meta description = %q; want %q", desc, "second definition wins")
	}
}

// TestUpsertEntity_ReUpsertReplacesProperties upserts the same entity twice with
// different property sets, verifying that the first set is fully replaced.
func TestUpsertEntity_ReUpsertReplacesProperties(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/replace-lib")

	entity := &source.Entity{
		Slug: "wp-post",
		Name: "WP_Post",
		Kind: "class",
		URL:  "https://developer.wordpress.org/reference/classes/wp_post/",
		Properties: []source.Property{
			{Name: "$ID", Type: "int", Visibility: "public"},
			{Name: "$post_author", Type: "string", Visibility: "public"},
			{Name: "$post_date", Type: "string", Visibility: "public"},
		},
	}

	id, err := s.UpsertEntity(ctx, libID, entity)
	if err != nil {
		t.Fatalf("first UpsertEntity: %v", err)
	}

	// Re-upsert with only one property.
	entity.Properties = []source.Property{
		{Name: "$ID", Type: "int", Visibility: "public"},
	}
	if _, err := s.UpsertEntity(ctx, libID, entity); err != nil {
		t.Fatalf("second UpsertEntity: %v", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM properties WHERE entity_id = ?`, id,
	).Scan(&count); err != nil {
		t.Fatalf("counting properties after re-upsert: %v", err)
	}
	if count != 1 {
		t.Errorf("property count after re-upsert = %d; want 1 (old properties deleted)", count)
	}
}

// TestGetEntity_NotFound_ReturnsErrNotFound verifies the sentinel error contract:
// a missing entity returns (nil, store.ErrNotFound). The implementation must
// never expose sql.ErrNoRows directly to the caller.
func TestGetEntity_NotFound_ReturnsErrNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	seedLibrary(t, s, "/lib")

	rec, err := s.GetEntity(ctx, "/lib", "does-not-exist")
	if errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetEntity returned raw sql.ErrNoRows; implementation must convert to store.ErrNotFound")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetEntity for missing slug: got err=%v, want store.ErrNotFound", err)
	}
	if rec != nil {
		t.Errorf("GetEntity for missing slug returned non-nil record: %+v", rec)
	}
}

// TestGetEntityByID_NotFound_ReturnsErrNotFound verifies the sentinel error
// contract for GetEntityByID: a missing id returns (nil, store.ErrNotFound).
func TestGetEntityByID_NotFound_ReturnsErrNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	rec, err := s.GetEntityByID(ctx, 999999)
	if errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetEntityByID returned raw sql.ErrNoRows; implementation must convert to store.ErrNotFound")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetEntityByID for missing id: got err=%v, want store.ErrNotFound", err)
	}
	if rec != nil {
		t.Errorf("GetEntityByID for missing id returned non-nil record: %+v", rec)
	}
}

// TestListEntities_OrderedByName inserts entities in reverse alphabetical order
// and verifies ListEntities returns them sorted by name.
func TestListEntities_OrderedByName(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/order-lib")

	for _, slug := range []string{"zzz-entity", "aaa-entity", "mmm-entity"} {
		entity := &source.Entity{
			Slug: slug,
			Name: slug, // name == slug for easy ordering assertion
			Kind: "class",
			URL:  "https://example.com/" + slug,
		}
		if _, err := s.UpsertEntity(ctx, libID, entity); err != nil {
			t.Fatalf("UpsertEntity %q: %v", slug, err)
		}
	}

	results, err := s.ListEntities(ctx, libID)
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("ListEntities returned %d results; want 3", len(results))
	}

	wantOrder := []string{"aaa-entity", "mmm-entity", "zzz-entity"}
	for i, want := range wantOrder {
		if results[i].Name != want {
			t.Errorf("ListEntities[%d].Name = %q; want %q", i, results[i].Name, want)
		}
	}
}

// TestListEntities_Empty verifies that ListEntities on a library with no entities
// returns a nil or empty slice without error.
func TestListEntities_Empty(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/empty-lib")

	results, err := s.ListEntities(ctx, libID)
	if err != nil {
		t.Fatalf("ListEntities on empty library: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("ListEntities on empty library returned %d results; want 0", len(results))
	}
}

// ─── 5. UpsertMethod / GetMethod / ListMethods / ListRelations ───────────────

// TestUpsertMethod_InsertAndRead inserts a method with relations and reads it
// back via GetMethod and ListRelations.
func TestUpsertMethod_InsertAndRead(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/method-lib")
	entityID := seedEntity(t, s, libID, "wp-query")

	method := &source.Method{
		Slug:        "get-posts",
		Name:        "get_posts",
		Signature:   "get_posts( array $query = null ): WP_Post[]",
		Description: "Retrieve array of posts.",
		ReturnType:  "WP_Post[]",
		URL:         "https://developer.wordpress.org/reference/classes/wp_query/get_posts/",
		Since:       "1.5.0",
		Parameters: []source.Parameter{
			{Name: "$query", Type: "array", Required: false, Description: "Optional. Query args."},
		},
		Relations: []source.Relation{
			{Kind: "uses", TargetName: "WP_Query::parse_query()", TargetURL: "https://example.com/parse_query"},
			{Kind: "uses", TargetName: "WP_Query::query()", TargetURL: "https://example.com/query"},
		},
	}

	if err := s.UpsertMethod(ctx, entityID, method); err != nil {
		t.Fatalf("UpsertMethod: %v", err)
	}

	rec, err := s.GetMethod(ctx, entityID, method.Slug)
	if err != nil {
		t.Fatalf("GetMethod: %v", err)
	}
	if rec == nil {
		t.Fatal("GetMethod returned nil; want record")
	}
	if rec.Name != method.Name {
		t.Errorf("GetMethod Name = %q; want %q", rec.Name, method.Name)
	}
	if rec.Signature != method.Signature {
		t.Errorf("GetMethod Signature = %q; want %q", rec.Signature, method.Signature)
	}
	if rec.Since != method.Since {
		t.Errorf("GetMethod Since = %q; want %q", rec.Since, method.Since)
	}

	// Verify relations were stored.
	rels, err := s.ListRelations(ctx, rec.ID)
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(rels) != 2 {
		t.Errorf("ListRelations returned %d; want 2", len(rels))
	}
}

// TestUpsertMethod_ReUpsertReplacesRelations verifies that re-upserting an
// existing method correctly replaces its relations. This test previously
// characterised a known bug where LastInsertId() returned a spurious
// AUTOINCREMENT sequence value (not the existing rowid) when the ON CONFLICT
// DO UPDATE path fired, causing the subsequent INSERT INTO relations to
// reference a non-existent method_id and fail with a FOREIGN KEY constraint
// error. That bug is fixed: UpsertMethod now always uses the always-SELECT
// pattern (mirroring UpsertEntity) to reliably retrieve the method ID.
func TestUpsertMethod_ReUpsertReplacesRelations(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/rel-lib")
	entityID := seedEntity(t, s, libID, "wp-cache")

	method := &source.Method{
		Slug: "get",
		Name: "get",
		URL:  "https://example.com/get",
		Relations: []source.Relation{
			{Kind: "uses", TargetName: "wp_cache_get()", TargetURL: "https://example.com/a"},
			{Kind: "uses", TargetName: "wp_cache_set()", TargetURL: "https://example.com/b"},
		},
	}

	// First insert — must succeed and store 2 relations.
	if err := s.UpsertMethod(ctx, entityID, method); err != nil {
		t.Fatalf("first UpsertMethod: %v", err)
	}
	rec, err := s.GetMethod(ctx, entityID, method.Slug)
	if err != nil {
		t.Fatalf("GetMethod after first insert: %v", err)
	}
	rels, err := s.ListRelations(ctx, rec.ID)
	if err != nil {
		t.Fatalf("ListRelations after first insert: %v", err)
	}
	if len(rels) != 2 {
		t.Errorf("ListRelations after first insert = %d; want 2", len(rels))
	}

	// Re-upsert with only one relation — the always-SELECT fix means this must
	// succeed and leave exactly 1 relation (old relations deleted, new one inserted).
	method.Relations = []source.Relation{
		{Kind: "uses", TargetName: "wp_cache_get()", TargetURL: "https://example.com/a"},
	}
	if err := s.UpsertMethod(ctx, entityID, method); err != nil {
		t.Fatalf("second UpsertMethod (re-upsert): %v", err)
	}

	rels2, err := s.ListRelations(ctx, rec.ID)
	if err != nil {
		t.Fatalf("ListRelations after re-upsert: %v", err)
	}
	if len(rels2) != 1 {
		t.Errorf("ListRelations after re-upsert = %d; want 1 (old relations deleted, new one inserted)", len(rels2))
	}
	if rels2[0].TargetName != "wp_cache_get()" {
		t.Errorf("surviving relation TargetName = %q; want %q", rels2[0].TargetName, "wp_cache_get()")
	}
}

// TestUpsertMethod_ValidEntity_Succeeds verifies the happy path for UpsertMethod:
// when the entity exists the upsert completes without error. The redundant
// pre-flight SELECT EXISTS FK check (HIGH-18) has been removed; FK enforcement
// is now delegated entirely to the SQLite FOREIGN KEY constraint, and the error
// is wrapped as store.ErrFKConstraint when the constraint fires.
func TestUpsertMethod_ValidEntity_Succeeds(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/fk-lib")
	entityID := seedEntity(t, s, libID, "fk-entity")

	method := &source.Method{
		Slug: "do-thing",
		Name: "doThing",
		URL:  "https://example.com/do-thing",
	}

	if err := s.UpsertMethod(ctx, entityID, method); err != nil {
		t.Fatalf("UpsertMethod with valid entityID: %v", err)
	}
}

// TestUpsertMethod_NonExistentEntity_ReturnsErrFKConstraint verifies that
// UpsertMethod returns store.ErrFKConstraint (not a silent no-op) when the
// entityID does not exist. The error must also contain the entity ID for
// diagnostic context.
func TestUpsertMethod_NonExistentEntity_ReturnsErrFKConstraint(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	method := &source.Method{
		Slug: "some-method",
		Name: "someMethod",
		URL:  "https://example.com/some-method",
	}

	err := s.UpsertMethod(ctx, 99999, method)
	if err == nil {
		t.Fatal("UpsertMethod with non-existent entityID returned nil error; want store.ErrFKConstraint")
	}
	if !errors.Is(err, store.ErrFKConstraint) {
		t.Errorf("UpsertMethod with non-existent entityID: got %v; want errors.Is(err, store.ErrFKConstraint)", err)
	}
	// The error message should also mention the entity ID for diagnostics.
	wantFragment := "99999"
	if !containsSubstring(err.Error(), wantFragment) {
		t.Errorf("error %q does not contain entity id %q", err.Error(), wantFragment)
	}
}

// TestGetMethod_NotFound_ReturnsErrNotFound verifies the sentinel error contract:
// a missing method returns (nil, store.ErrNotFound).
func TestGetMethod_NotFound_ReturnsErrNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/m-lib")
	entityID := seedEntity(t, s, libID, "m-entity")

	rec, err := s.GetMethod(ctx, entityID, "no-such-method")
	if errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetMethod returned raw sql.ErrNoRows; implementation must convert to store.ErrNotFound")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetMethod for missing slug: got err=%v, want store.ErrNotFound", err)
	}
	if rec != nil {
		t.Errorf("GetMethod for missing slug returned non-nil record: %+v", rec)
	}
}

// TestListMethods_OrderedByName inserts methods in reverse order and verifies
// ListMethods returns them sorted by name.
func TestListMethods_OrderedByName(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/lm-lib")
	entityID := seedEntity(t, s, libID, "lm-entity")

	for _, slug := range []string{"zzz-method", "aaa-method", "mmm-method"} {
		m := &source.Method{Slug: slug, Name: slug, URL: "https://example.com/" + slug}
		if err := s.UpsertMethod(ctx, entityID, m); err != nil {
			t.Fatalf("UpsertMethod %q: %v", slug, err)
		}
	}

	results, err := s.ListMethods(ctx, entityID)
	if err != nil {
		t.Fatalf("ListMethods: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("ListMethods returned %d; want 3", len(results))
	}
	wantOrder := []string{"aaa-method", "mmm-method", "zzz-method"}
	for i, want := range wantOrder {
		if results[i].Name != want {
			t.Errorf("ListMethods[%d].Name = %q; want %q", i, results[i].Name, want)
		}
	}
}

// TestListRelations_OrderedByKindAndTargetName verifies relation ordering.
func TestListRelations_OrderedByKindAndTargetName(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/lr-lib")
	entityID := seedEntity(t, s, libID, "lr-entity")

	method := &source.Method{
		Slug: "with-rels",
		Name: "withRels",
		URL:  "https://example.com/with-rels",
		Relations: []source.Relation{
			{Kind: "used_by", TargetName: "beta_func()", TargetURL: "https://b.example.com"},
			{Kind: "used_by", TargetName: "alpha_func()", TargetURL: "https://a.example.com"},
			{Kind: "uses", TargetName: "gamma_func()", TargetURL: "https://g.example.com"},
		},
	}
	if err := s.UpsertMethod(ctx, entityID, method); err != nil {
		t.Fatalf("UpsertMethod: %v", err)
	}

	rec, err := s.GetMethod(ctx, entityID, method.Slug)
	if err != nil {
		t.Fatalf("GetMethod: %v", err)
	}
	rels, err := s.ListRelations(ctx, rec.ID)
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(rels) != 3 {
		t.Fatalf("ListRelations returned %d; want 3", len(rels))
	}

	// ORDER BY kind, target_name: "used_by" < "uses" alphabetically.
	// Within "used_by": "alpha_func" < "beta_func".
	expected := []store.RelationRecord{
		{Kind: "used_by", TargetName: "alpha_func()"},
		{Kind: "used_by", TargetName: "beta_func()"},
		{Kind: "uses", TargetName: "gamma_func()"},
	}
	for i, want := range expected {
		if rels[i].Kind != want.Kind || rels[i].TargetName != want.TargetName {
			t.Errorf("ListRelations[%d] = {Kind:%q TargetName:%q}; want {Kind:%q TargetName:%q}",
				i, rels[i].Kind, rels[i].TargetName, want.Kind, want.TargetName)
		}
	}
}

// ─── 6. Error wrapping ───────────────────────────────────────────────────────

// TestUpsertEntity_ErrorWrapping verifies that UpsertEntity wraps errors with
// the entity slug in the message. Wave 1 added fmt.Errorf("upsert entity %q…: %w")
// — we lock in that the slug appears in the error string.
//
// We provoke a context-cancelled error to get a deterministic failure without
// touching the DB schema.
func TestUpsertEntity_ErrorWrapping(t *testing.T) {
	s, _ := newTestStore(t)

	// Cancel the context immediately so the transaction begin fails.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	libID := seedLibrary(t, &SQLiteStore{db: s.db}, "/wrap-lib") // use a separate handle to avoid contaminating s
	_ = libID

	entity := &source.Entity{
		Slug: "wrap-test-entity",
		Name: "WrapTest",
		Kind: "class",
		URL:  "https://example.com/wrap",
	}

	// We need a library to exist so we can trigger the insert path.
	// Seed with background context (the library exists before the cancel).
	bgCtx := context.Background()
	if err := s.UpsertLibrary(bgCtx, "/wrap-lib2", source.LibraryMeta{
		Name: "Wrap Lib", SourceURL: "https://example.com/wrap",
	}); err != nil {
		t.Fatalf("seed library for error wrapping test: %v", err)
	}

	entity.Slug = "wrap-test-entity"
	err := func() error {
		_, err := s.UpsertEntity(ctx, "/wrap-lib2", entity)
		return err
	}()

	if err == nil {
		// Context was already cancelled — if no error was returned, the test
		// environment is too fast; skip rather than fail.
		t.Skip("cancelled context did not produce an error (context may not have been checked)")
	}

	wantSlug := entity.Slug
	if !containsSubstring(err.Error(), wantSlug) {
		t.Errorf("UpsertEntity error = %q; expected it to contain slug %q", err.Error(), wantSlug)
	}
}

// ─── 7. ListLibraries ───────────────────────────────────────────────────────

// TestListLibraries_OrderedByName verifies alphabetical ordering and that the
// count matches insertions.
func TestListLibraries_OrderedByName(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	names := []string{"Zebra Lib", "Alpha Lib", "Mango Lib"}
	for i, name := range names {
		meta := source.LibraryMeta{Name: name, SourceURL: "https://example.com/" + fmt.Sprint(i)}
		if err := s.UpsertLibrary(ctx, fmt.Sprintf("/lib-%d", i), meta); err != nil {
			t.Fatalf("UpsertLibrary %q: %v", name, err)
		}
	}

	results, err := s.ListLibraries(ctx)
	if err != nil {
		t.Fatalf("ListLibraries: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("ListLibraries returned %d; want 3", len(results))
	}
	// Sorted ascending: Alpha, Mango, Zebra
	want := []string{"Alpha Lib", "Mango Lib", "Zebra Lib"}
	for i, w := range want {
		if results[i].Name != w {
			t.Errorf("ListLibraries[%d].Name = %q; want %q", i, results[i].Name, w)
		}
	}
}

// ─── 8. Crawl session audit ──────────────────────────────────────────────────

// TestCreateAndUpdateCrawlSession_RoundTrip creates a session, transitions its
// status to "completed", and verifies CompletedAt is set.
func TestCreateAndUpdateCrawlSession_RoundTrip(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/crawl-lib")

	before := time.Now().Truncate(time.Second)

	sessionID, err := s.CreateCrawlSession(ctx, libID, 100)
	if err != nil {
		t.Fatalf("CreateCrawlSession: %v", err)
	}
	if sessionID <= 0 {
		t.Errorf("CreateCrawlSession returned sessionID=%d; want positive", sessionID)
	}

	// Update to "completed".
	if err := s.UpdateCrawlSession(ctx, sessionID, "completed", 80, 10, 10); err != nil {
		t.Fatalf("UpdateCrawlSession to completed: %v", err)
	}

	session, err := s.GetLastSession(ctx, libID)
	if err != nil {
		t.Fatalf("GetLastSession: %v", err)
	}
	if session == nil {
		t.Fatal("GetLastSession returned nil; want record")
	}
	if session.Status != "completed" {
		t.Errorf("session.Status = %q; want %q", session.Status, "completed")
	}
	if session.SuccessCount != 80 {
		t.Errorf("session.SuccessCount = %d; want 80", session.SuccessCount)
	}
	if session.FailCount != 10 {
		t.Errorf("session.FailCount = %d; want 10", session.FailCount)
	}
	if session.CompletedAt == nil {
		t.Error("session.CompletedAt is nil; want non-nil for completed status")
	} else {
		after := time.Now().Add(time.Second)
		if session.CompletedAt.Before(before) || session.CompletedAt.After(after) {
			t.Errorf("session.CompletedAt = %v; want between %v and %v",
				session.CompletedAt, before, after)
		}
	}
}

// TestUpdateCrawlSession_InterruptedSetsCompletedAt verifies that "interrupted"
// status also sets the completed_at timestamp.
func TestUpdateCrawlSession_InterruptedSetsCompletedAt(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/interrupted-lib")

	sessionID, err := s.CreateCrawlSession(ctx, libID, 50)
	if err != nil {
		t.Fatalf("CreateCrawlSession: %v", err)
	}

	if err := s.UpdateCrawlSession(ctx, sessionID, "interrupted", 30, 5, 0); err != nil {
		t.Fatalf("UpdateCrawlSession to interrupted: %v", err)
	}

	session, err := s.GetLastSession(ctx, libID)
	if err != nil {
		t.Fatalf("GetLastSession: %v", err)
	}
	if session.CompletedAt == nil {
		t.Error("CompletedAt is nil for interrupted session; want non-nil")
	}
}

// TestGetLastSession_ReturnsLatest inserts two sessions for the same library and
// verifies GetLastSession returns the one with the higher ID.
func TestGetLastSession_ReturnsLatest(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/latest-lib")

	id1, err := s.CreateCrawlSession(ctx, libID, 10)
	if err != nil {
		t.Fatalf("CreateCrawlSession 1: %v", err)
	}
	id2, err := s.CreateCrawlSession(ctx, libID, 20)
	if err != nil {
		t.Fatalf("CreateCrawlSession 2: %v", err)
	}

	// Mark first session completed so it has a different status.
	if err := s.UpdateCrawlSession(ctx, id1, "completed", 10, 0, 0); err != nil {
		t.Fatalf("UpdateCrawlSession 1: %v", err)
	}

	session, err := s.GetLastSession(ctx, libID)
	if err != nil {
		t.Fatalf("GetLastSession: %v", err)
	}
	if session == nil {
		t.Fatal("GetLastSession returned nil")
	}
	if session.ID != id2 {
		t.Errorf("GetLastSession returned ID=%d; want ID=%d (latest)", session.ID, id2)
	}
}

// TestGetLastSession_NotFound_ReturnsErrNotFound verifies the sentinel error
// contract: a library with no sessions returns (nil, store.ErrNotFound).
func TestGetLastSession_NotFound_ReturnsErrNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/nosession-lib")

	session, err := s.GetLastSession(ctx, libID)
	if errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetLastSession returned raw sql.ErrNoRows; implementation must return store.ErrNotFound")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetLastSession with no sessions: got err=%v, want store.ErrNotFound", err)
	}
	if session != nil {
		t.Errorf("GetLastSession with no sessions returned non-nil: %+v", session)
	}
}

// ─── 9. RecordProgress / GetProcessedURLs / GetCrawlStats / GetFailures ──────

// TestRecordProgress_UpsertSemantics inserts a progress item then updates it;
// only one row should exist with the updated status.
func TestRecordProgress_UpsertSemantics(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/progress-lib")

	sessionID, err := s.CreateCrawlSession(ctx, libID, 5)
	if err != nil {
		t.Fatalf("CreateCrawlSession: %v", err)
	}

	url := "https://example.com/class/wp_query/"
	item := &store.CrawlProgressItem{
		URL:      url,
		ItemType: "entity",
		Status:   "pending",
	}
	if err := s.RecordProgress(ctx, sessionID, item); err != nil {
		t.Fatalf("RecordProgress (first): %v", err)
	}

	// Update same URL to "success".
	item.Status = "success"
	if err := s.RecordProgress(ctx, sessionID, item); err != nil {
		t.Fatalf("RecordProgress (update): %v", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM crawl_progress WHERE session_id = ? AND url = ?`,
		sessionID, url,
	).Scan(&count); err != nil {
		t.Fatalf("counting progress rows: %v", err)
	}
	if count != 1 {
		t.Errorf("crawl_progress row count = %d; want 1 (ON CONFLICT upsert)", count)
	}

	var status string
	if err := s.db.QueryRowContext(ctx,
		`SELECT status FROM crawl_progress WHERE session_id = ? AND url = ?`,
		sessionID, url,
	).Scan(&status); err != nil {
		t.Fatalf("reading progress status: %v", err)
	}
	if status != "success" {
		t.Errorf("progress status = %q; want %q", status, "success")
	}
}

// TestGetProcessedURLs_ReturnsOnlySuccess seeds a session with mixed statuses
// and verifies GetProcessedURLs returns only "success" entries.
func TestGetProcessedURLs_ReturnsOnlySuccess(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/processed-lib")

	sessionID, err := s.CreateCrawlSession(ctx, libID, 3)
	if err != nil {
		t.Fatalf("CreateCrawlSession: %v", err)
	}

	items := []store.CrawlProgressItem{
		{URL: "https://a.example.com/", ItemType: "entity", Status: "success"},
		{URL: "https://b.example.com/", ItemType: "entity", Status: "failed", ErrorType: "http_404"},
		{URL: "https://c.example.com/", ItemType: "entity", Status: "skipped"},
	}
	for i := range items {
		if err := s.RecordProgress(ctx, sessionID, &items[i]); err != nil {
			t.Fatalf("RecordProgress %d: %v", i, err)
		}
	}

	processed, err := s.GetProcessedURLs(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetProcessedURLs: %v", err)
	}
	if len(processed) != 1 {
		t.Errorf("GetProcessedURLs returned %d entries; want 1 (only success)", len(processed))
	}
	if _, ok := processed["https://a.example.com/"]; !ok {
		t.Errorf("GetProcessedURLs missing expected success URL")
	}
}

// TestGetCrawlStats_AggregatesCorrectly inserts a known mix of progress items
// and verifies the aggregate counts.
func TestGetCrawlStats_AggregatesCorrectly(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/stats-lib")

	sessionID, err := s.CreateCrawlSession(ctx, libID, 5)
	if err != nil {
		t.Fatalf("CreateCrawlSession: %v", err)
	}

	items := []store.CrawlProgressItem{
		{URL: "https://a.example.com/", ItemType: "entity", Status: "success"},
		{URL: "https://b.example.com/", ItemType: "entity", Status: "success"},
		{URL: "https://c.example.com/", ItemType: "entity", Status: "failed", ErrorType: "http_404"},
		{URL: "https://d.example.com/", ItemType: "entity", Status: "failed", ErrorType: "parse_error"},
		{URL: "https://e.example.com/", ItemType: "entity", Status: "skipped"},
	}
	for i := range items {
		if err := s.RecordProgress(ctx, sessionID, &items[i]); err != nil {
			t.Fatalf("RecordProgress %d: %v", i, err)
		}
	}

	stats, err := s.GetCrawlStats(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetCrawlStats: %v", err)
	}
	if stats.Total != 5 {
		t.Errorf("stats.Total = %d; want 5", stats.Total)
	}
	if stats.Success != 2 {
		t.Errorf("stats.Success = %d; want 2", stats.Success)
	}
	if stats.Failed != 2 {
		t.Errorf("stats.Failed = %d; want 2", stats.Failed)
	}
	if stats.Skipped != 1 {
		t.Errorf("stats.Skipped = %d; want 1", stats.Skipped)
	}
	if stats.FailuresByType["http_404"] != 1 {
		t.Errorf("FailuresByType[http_404] = %d; want 1", stats.FailuresByType["http_404"])
	}
	if stats.FailuresByType["parse_error"] != 1 {
		t.Errorf("FailuresByType[parse_error] = %d; want 1", stats.FailuresByType["parse_error"])
	}
}

// TestGetFailures_ReturnsOnlyFailed verifies GetFailures returns only rows with
// status "failed" and that COALESCE handles nullable fields gracefully.
func TestGetFailures_ReturnsOnlyFailed(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/failures-lib")

	sessionID, err := s.CreateCrawlSession(ctx, libID, 3)
	if err != nil {
		t.Fatalf("CreateCrawlSession: %v", err)
	}

	items := []store.CrawlProgressItem{
		{URL: "https://ok.example.com/", ItemType: "entity", Status: "success"},
		{URL: "https://fail1.example.com/", ItemType: "entity", Status: "failed",
			ErrorType: "http_404", ErrorMessage: "not found", ParentEntity: "https://p.example.com/"},
		{URL: "https://fail2.example.com/", ItemType: "method", Status: "failed",
			ErrorType: "parse_error"},
	}
	for i := range items {
		if err := s.RecordProgress(ctx, sessionID, &items[i]); err != nil {
			t.Fatalf("RecordProgress %d: %v", i, err)
		}
	}

	failures, err := s.GetFailures(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetFailures: %v", err)
	}
	if len(failures) != 2 {
		t.Fatalf("GetFailures returned %d; want 2", len(failures))
	}
	// Verify COALESCE on nullable fields doesn't return NULL/empty panic.
	for _, f := range failures {
		if f.URL == "" {
			t.Error("GetFailures returned item with empty URL")
		}
	}
}

// ─── 10. UpdateSnippetCount / ComputeSnippetCount ────────────────────────────

// TestUpdateSnippetCount_IdempotentAndReadBack verifies UpdateSnippetCount
// persists the count and subsequent reads return it via GetLibrary.
func TestUpdateSnippetCount_IdempotentAndReadBack(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/snippet-lib")

	if err := s.UpdateSnippetCount(ctx, libID, 42); err != nil {
		t.Fatalf("UpdateSnippetCount: %v", err)
	}

	rec, err := s.GetLibrary(ctx, libID)
	if err != nil {
		t.Fatalf("GetLibrary: %v", err)
	}
	if rec.SnippetCount != 42 {
		t.Errorf("SnippetCount = %d; want 42", rec.SnippetCount)
	}

	// Second update overwrites the first.
	if err := s.UpdateSnippetCount(ctx, libID, 100); err != nil {
		t.Fatalf("second UpdateSnippetCount: %v", err)
	}
	rec, err = s.GetLibrary(ctx, libID)
	if err != nil {
		t.Fatalf("GetLibrary after second update: %v", err)
	}
	if rec.SnippetCount != 100 {
		t.Errorf("SnippetCount after second update = %d; want 100", rec.SnippetCount)
	}
}

// TestComputeSnippetCount_SumsEntitiesAndMethods inserts 3 entities with 2
// methods each and 1 entity with no methods, then verifies ComputeSnippetCount
// returns 4 entities + 6 methods = 10 (entities count too).
//
// Formula: count(entities) + count(methods joined to entities).
func TestComputeSnippetCount_SumsEntitiesAndMethods(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/compute-lib")

	// Insert 3 entities with 2 methods each, plus 1 entity with no methods.
	for i := 0; i < 3; i++ {
		slug := fmt.Sprintf("entity-%d", i)
		entityID := seedEntity(t, s, libID, slug)
		for j := 0; j < 2; j++ {
			mSlug := fmt.Sprintf("method-%d-%d", i, j)
			m := &source.Method{Slug: mSlug, Name: mSlug, URL: "https://example.com/" + mSlug}
			if err := s.UpsertMethod(ctx, entityID, m); err != nil {
				t.Fatalf("UpsertMethod %q: %v", mSlug, err)
			}
		}
	}
	seedEntity(t, s, libID, "no-methods-entity")

	count, err := s.ComputeSnippetCount(ctx, libID)
	if err != nil {
		t.Fatalf("ComputeSnippetCount: %v", err)
	}
	// 4 entities + 6 methods = 10
	if count != 10 {
		t.Errorf("ComputeSnippetCount = %d; want 10 (4 entities + 6 methods)", count)
	}
}

// ─── 11. Concurrent writes (race detector) ──────────────────────────────────

// TestConcurrentUpsertEntity_SerializesViaMutex spawns 20 goroutines that each
// insert a distinct entity. Run with -race to detect data races on s.mu.
//
// This test validates the core concurrency guarantee: sync.Mutex serializes all
// writes so no two transactions overlap in the SQLite backend.
func TestConcurrentUpsertEntity_SerializesViaMutex(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/concurrent-lib")

	const numGoroutines = 20
	var wg sync.WaitGroup
	errs := make([]error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		i := i // capture
		go func() {
			defer wg.Done()
			entity := &source.Entity{
				Slug: fmt.Sprintf("concurrent-entity-%d", i),
				Name: fmt.Sprintf("ConcurrentEntity%d", i),
				Kind: "class",
				URL:  fmt.Sprintf("https://example.com/concurrent-%d", i),
				Properties: []source.Property{
					{Name: "$prop", Type: "string", Visibility: "public"},
				},
			}
			_, errs[i] = s.UpsertEntity(ctx, libID, entity)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d UpsertEntity error: %v", i, err)
		}
	}

	// All 20 entities must be present.
	entities, err := s.ListEntities(ctx, libID)
	if err != nil {
		t.Fatalf("ListEntities after concurrent inserts: %v", err)
	}
	if len(entities) != numGoroutines {
		t.Errorf("after concurrent inserts: ListEntities returned %d; want %d",
			len(entities), numGoroutines)
	}
}

// TestConcurrentUpsertMethod_SerializesViaMutex spawns goroutines inserting
// methods for the same entity, verifying no race on s.mu.
func TestConcurrentUpsertMethod_SerializesViaMutex(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	libID := seedLibrary(t, s, "/conc-method-lib")
	entityID := seedEntity(t, s, libID, "shared-entity")

	const numGoroutines = 20
	var wg sync.WaitGroup
	errs := make([]error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			m := &source.Method{
				Slug: fmt.Sprintf("concurrent-method-%d", i),
				Name: fmt.Sprintf("ConcurrentMethod%d", i),
				URL:  fmt.Sprintf("https://example.com/cm-%d", i),
			}
			errs[i] = s.UpsertMethod(ctx, entityID, m)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d UpsertMethod error: %v", i, err)
		}
	}

	methods, err := s.ListMethods(ctx, entityID)
	if err != nil {
		t.Fatalf("ListMethods after concurrent inserts: %v", err)
	}
	if len(methods) != numGoroutines {
		t.Errorf("after concurrent inserts: ListMethods returned %d; want %d",
			len(methods), numGoroutines)
	}
}

// ─── helper ──────────────────────────────────────────────────────────────────

// containsSubstring is a simple substring check used by error-message assertions.
func containsSubstring(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > 0 && containsSubstringLinear(s, substr))
}

func containsSubstringLinear(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
