//go:build sqlite_fts5 || fts5

// Package server_test holds characterization tests for the HTTP handler layer.
//
// Scaffolding strategy: we use a real sqlite.New(t.TempDir()) store seeded with
// known data so that the tests exercise the full stack from HTTP request through
// the defsource client to the SQLite database.  A real store is simpler than a
// mock (no interface-stub boilerplate) and gives realistic end-to-end behaviour.
//
// Where a 404 / not-found branch is needed the test deliberately uses a library ID
// that was never inserted, so the store's GetLibrary returns (nil, nil) and
// defsource.QueryDocs returns the "not found" error that handlers.go maps to 404.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	defsource "github.com/hatlesswizard/defsource"
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/store/sqlite"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// newTestServer builds a real *http.Server wired to a fresh SQLite database
// seeded according to the provided seed function.  It returns the server's
// ServeMux handler directly so that tests can call ServeHTTP without a
// network listener.
func newTestServer(t *testing.T, seed func(context.Context, *sqlite.SQLiteStore)) http.Handler {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("store.Close: %v", err)
		}
	})
	if seed != nil {
		seed(context.Background(), store)
	}
	client := defsource.NewWithStore(store)
	srv := New(client, ":0", "*")
	return srv.Handler
}

// do executes a single HTTP request against the given handler and returns the
// recorder so tests can inspect status and body.
func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	h.ServeHTTP(rr, req)
	return rr
}

// decodeError unmarshals {"error": "..."} from the response body.
func decodeError(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decodeError: body is not valid JSON %q: %v", body, err)
	}
	msg, ok := m["error"]
	if !ok {
		t.Fatalf("decodeError: no 'error' key in %q", body)
	}
	return msg
}

// seedLibrary inserts a minimal library record plus one entity and one method
// so that queryDocs and listEntities have something to return.
func seedLibrary(t *testing.T, ctx context.Context, st *sqlite.SQLiteStore) {
	t.Helper()
	meta := source.LibraryMeta{
		Name:        "WordPress 6.5",
		Description: "WordPress core",
		SourceURL:   "https://github.com/WordPress/WordPress",
		Version:     "6.5",
		TrustScore:  1.0,
	}
	if err := st.UpsertLibrary(ctx, "wordpress-6.5", meta); err != nil {
		t.Fatalf("UpsertLibrary: %v", err)
	}
	entity := &source.Entity{
		Slug:        "wp-query",
		Name:        "WP_Query",
		Kind:        "class",
		Description: "Core class used to implement the WordPress query.",
		SourceCode:  "class WP_Query {}",
		URL:         "https://developer.wordpress.org/reference/classes/wp_query/",
	}
	eid, err := st.UpsertEntity(ctx, "wordpress-6.5", entity)
	if err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	method := &source.Method{
		Slug:        "get-posts",
		Name:        "get_posts",
		Signature:   "public function get_posts(): WP_Post[]",
		Description: "Retrieves an array of posts based on query variables.",
		ReturnType:  "WP_Post[]",
		ReturnDesc:  "Array of post objects.",
		SourceCode:  "public function get_posts() { ... }",
		URL:         "https://developer.wordpress.org/reference/classes/wp_query/get_posts/",
	}
	if err := st.UpsertMethod(ctx, eid, method); err != nil {
		t.Fatalf("UpsertMethod: %v", err)
	}
	// Rebuild FTS5 search index so that Search() actually returns results.
	if err := st.RebuildIndex(ctx, "wordpress-6.5"); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}
}

// ── searchLibraries ───────────────────────────────────────────────────────────

// Test 1: missing query param → 400
func TestSearchLibraries_MissingQueryParam(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/libraries/search?libraryName=wordpress")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	body := rr.Body.Bytes()
	msg := decodeError(t, body)
	if !strings.Contains(msg, "required") {
		t.Errorf("error message %q should mention 'required'", msg)
	}
}

// Test 2: query too long (>500 characters) → 400
func TestSearchLibraries_QueryTooLong(t *testing.T) {
	h := newTestServer(t, nil)
	longQuery := strings.Repeat("a", 501)
	rr := do(h, http.MethodGet, "/api/v1/libraries/search?libraryName=wordpress&query="+longQuery)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "500") {
		t.Errorf("error message %q should mention the 500-char limit", msg)
	}
}

// Test 3: query with null byte → 400
func TestSearchLibraries_QueryNullByte(t *testing.T) {
	h := newTestServer(t, nil)
	// Embed a literal null byte in the query string.
	// httptest.NewRequest decodes percent-encoded %00 as \x00.
	rr := do(h, http.MethodGet, "/api/v1/libraries/search?libraryName=wordpress&query=hello%00world")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "invalid") {
		t.Errorf("error message %q should mention 'invalid'", msg)
	}
}

// Test 4: non-alphanumeric query (e.g. "!!!") → 400
func TestSearchLibraries_NonAlphanumericQuery(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/libraries/search?libraryName=wordpress&query=!!!")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "alphanumeric") {
		t.Errorf("error message %q should mention 'alphanumeric'", msg)
	}
}

// Test 5: valid query, empty DB → 404 (no libraries found)
func TestSearchLibraries_EmptyDB_Returns404(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/libraries/search?libraryName=wordpress&query=posts")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "no libraries") {
		t.Errorf("error message %q should mention 'no libraries'", msg)
	}
}

// Test 6: valid query, populated DB → 200 + results list
func TestSearchLibraries_PopulatedDB_Returns200WithResults(t *testing.T) {
	h := newTestServer(t, func(ctx context.Context, st *sqlite.SQLiteStore) {
		seedLibrary(t, ctx, st)
	})
	rr := do(h, http.MethodGet, "/api/v1/libraries/search?libraryName=WordPress&query=posts")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	results, ok := resp["results"]
	if !ok {
		t.Fatal("response missing 'results' key")
	}
	list, ok := results.([]any)
	if !ok {
		t.Fatalf("'results' is not an array, got %T", results)
	}
	if len(list) == 0 {
		t.Error("expected at least one result, got empty list")
	}
}

// ── queryDocs ─────────────────────────────────────────────────────────────────

// Test 7: missing id (libraryId) → 400
func TestQueryDocs_MissingLibraryID(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/docs?query=posts")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "required") {
		t.Errorf("error message %q should mention 'required'", msg)
	}
}

// Test 8: not-found library → 404
func TestQueryDocs_NotFoundLibrary_Returns404(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/docs?libraryId=does-not-exist&query=posts")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "not found") {
		t.Errorf("error message %q should mention 'not found'", msg)
	}
}

// Test 9: valid library + default (Markdown) output → 200, text/markdown
func TestQueryDocs_ValidLibrary_MarkdownOutput(t *testing.T) {
	h := newTestServer(t, func(ctx context.Context, st *sqlite.SQLiteStore) {
		seedLibrary(t, ctx, st)
	})
	rr := do(h, http.MethodGet, "/api/v1/docs?libraryId=wordpress-6.5&query=query")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/markdown") {
		t.Errorf("Content-Type = %q, want text/markdown", ct)
	}
	body := rr.Body.String()
	if body == "" {
		t.Error("expected non-empty markdown body")
	}
}

// Test 10: valid library + format=json output → 200, application/json, body schema
func TestQueryDocs_ValidLibrary_JSONOutput(t *testing.T) {
	h := newTestServer(t, func(ctx context.Context, st *sqlite.SQLiteStore) {
		seedLibrary(t, ctx, st)
	})
	rr := do(h, http.MethodGet, "/api/v1/docs?libraryId=wordpress-6.5&query=query&format=json")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var resp struct {
		Library  string `json:"library"`
		Query    string `json:"query"`
		Snippets []any  `json:"snippets"`
		Text     string `json:"text"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Library != "wordpress-6.5" {
		t.Errorf("library = %q, want wordpress-6.5", resp.Library)
	}
	if resp.Query != "query" {
		t.Errorf("query = %q, want query", resp.Query)
	}
	// snippets must be [] not null
	if resp.Snippets == nil {
		t.Error("snippets field must not be null; want []")
	}
}

// Test 11: queryDocs — mode validation: invalid mode → 400
func TestQueryDocs_InvalidMode_Returns400(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/docs?libraryId=wordpress-6.5&query=posts&mode=invalid")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "mode") {
		t.Errorf("error message %q should mention 'mode'", msg)
	}
}

// Test 12: queryDocs — snippets field is [] not null even for empty results
func TestQueryDocs_EmptyResults_SnippetsIsNotNull(t *testing.T) {
	h := newTestServer(t, func(ctx context.Context, st *sqlite.SQLiteStore) {
		seedLibrary(t, ctx, st)
	})
	// "zzzzzzzzzzz" is unlikely to match anything.
	rr := do(h, http.MethodGet, "/api/v1/docs?libraryId=wordpress-6.5&query=zzzzzzzzzzz&format=json")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	// The raw JSON must contain "snippets":[] not "snippets":null.
	raw := rr.Body.String()
	if strings.Contains(raw, `"snippets":null`) {
		t.Errorf("snippets serialised as null; want [].\nbody: %s", raw)
	}
}

// ── listLibraries ─────────────────────────────────────────────────────────────

// Test 13: listLibraries — empty DB → 200 + {"libraries":[]}
func TestListLibraries_EmptyDB_Returns200WithEmptyList(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/libraries")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	libs, ok := resp["libraries"]
	if !ok {
		t.Fatal("response missing 'libraries' key")
	}
	list, ok := libs.([]any)
	if !ok {
		t.Fatalf("'libraries' is not an array")
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

// Test 14: listLibraries — populated DB → 200 + non-empty list
func TestListLibraries_PopulatedDB_Returns200WithLibraries(t *testing.T) {
	h := newTestServer(t, func(ctx context.Context, st *sqlite.SQLiteStore) {
		seedLibrary(t, ctx, st)
	})
	rr := do(h, http.MethodGet, "/api/v1/libraries")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	list, _ := resp["libraries"].([]any)
	if len(list) == 0 {
		t.Error("expected at least one library, got empty list")
	}
}

// ── listEntities ──────────────────────────────────────────────────────────────

// Test 15: listEntities — missing libraryId → 400
func TestListEntities_MissingLibraryID(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/entities")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "required") {
		t.Errorf("error message %q should mention 'required'", msg)
	}
}

// Test 16: listEntities — valid libraryId, empty (unknown library) → 200 + {"entities":[]}
func TestListEntities_UnknownLibrary_Returns200WithEmptyEntities(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/entities?libraryId=does-not-exist")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	list, _ := resp["entities"].([]any)
	if len(list) != 0 {
		t.Errorf("expected empty list for unknown library, got %d items", len(list))
	}
}

// Test 17: listEntities — populated DB → 200 + entities
func TestListEntities_PopulatedDB_Returns200WithEntities(t *testing.T) {
	h := newTestServer(t, func(ctx context.Context, st *sqlite.SQLiteStore) {
		seedLibrary(t, ctx, st)
	})
	rr := do(h, http.MethodGet, "/api/v1/entities?libraryId=wordpress-6.5")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	list, _ := resp["entities"].([]any)
	if len(list) == 0 {
		t.Error("expected at least one entity, got empty list")
	}
}

// Test 18: listEntities — null byte in libraryId → 400
func TestListEntities_NullByteInLibraryID_Returns400(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/entities?libraryId=bad%00id")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "invalid") {
		t.Errorf("error message %q should mention 'invalid'", msg)
	}
}

// ── health ────────────────────────────────────────────────────────────────────

// Test 19: health endpoint → 200 {"status":"ok"}
func TestHealth_Returns200OK(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/health")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf(`status = %q, want "ok"`, resp["status"])
	}
}

// ── error response shape ──────────────────────────────────────────────────────

// Test 20: all 400-path error responses carry a consistent {"error": "..."} shape.
// We exercise searchLibraries, queryDocs, listEntities and confirm uniformity.
func TestErrorResponseShape_IsConsistentAcrossAllPaths(t *testing.T) {
	h := newTestServer(t, nil)

	paths := []struct {
		name   string
		method string
		path   string
	}{
		{"searchLibraries_noQuery", http.MethodGet, "/api/v1/libraries/search?libraryName=x"},
		{"searchLibraries_noName", http.MethodGet, "/api/v1/libraries/search?query=posts"},
		{"queryDocs_noLibraryId", http.MethodGet, "/api/v1/docs?query=posts"},
		{"queryDocs_noQuery", http.MethodGet, "/api/v1/docs?libraryId=x"},
		{"listEntities_noId", http.MethodGet, "/api/v1/entities"},
	}

	for _, tc := range paths {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rr := do(h, tc.method, tc.path)
			if rr.Code < 400 {
				t.Fatalf("expected error status, got %d", rr.Code)
			}
			ct := rr.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			var m map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
				t.Fatalf("body not valid JSON: %v", err)
			}
			errVal, ok := m["error"]
			if !ok {
				t.Error("response missing 'error' key")
			}
			if _, isStr := errVal.(string); !isStr {
				t.Errorf("'error' value is %T, want string", errVal)
			}
		})
	}
}

// ── validateQueryRequest (exercised via searchLibraries and queryDocs) ─────────

// Test 21: validateQueryRequest — libraryName too long → 400 with max-len hint
func TestValidateQueryRequest_LibraryNameTooLong(t *testing.T) {
	h := newTestServer(t, nil)
	longName := strings.Repeat("x", 201)
	rr := do(h, http.MethodGet, "/api/v1/libraries/search?libraryName="+longName+"&query=posts")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if !strings.Contains(msg, "200") {
		t.Errorf("error message %q should mention the 200-char limit for libraryName", msg)
	}
}

// Test 22: validateQueryRequest — libraryId too long → 400
func TestValidateQueryRequest_LibraryIDTooLong(t *testing.T) {
	h := newTestServer(t, nil)
	longID := strings.Repeat("x", 201)
	rr := do(h, http.MethodGet, "/api/v1/docs?libraryId="+longID+"&query=posts")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// Test 23: validateQueryRequest — null byte in id → 400
func TestValidateQueryRequest_NullByteInID(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/api/v1/docs?libraryId=lib%00id&query=posts")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// ── hasAlphanumeric (unit-level) ──────────────────────────────────────────────

// Test 24: hasAlphanumeric covers ASCII, digit, Unicode-letter, punctuation, empty cases.
func TestHasAlphanumeric(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"hello", true},
		{"123", true},
		{"!@#$%", false},
		{"", false},
		{"!a", true},          // mixed: one alphanumeric
		{"日本語", true},         // Unicode letters
		{"  \t\n", false},     // whitespace only
		{"---hello---", true}, // dashes around a word
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := hasAlphanumeric(tc.input)
			if got != tc.want {
				t.Errorf("hasAlphanumeric(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── itoa helper ───────────────────────────────────────────────────────────────

// Test 25: itoa covers zero, single-digit, multi-digit.
func TestItoa(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{200, "200"},
		{1000, "1000"},
	}
	for _, tc := range cases {
		got := itoa(tc.n)
		if got != tc.want {
			t.Errorf("itoa(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// ── 404 catch-all route ───────────────────────────────────────────────────────

// Test 26: unknown route → 404 JSON error
func TestUnknownRoute_Returns404JSON(t *testing.T) {
	h := newTestServer(t, nil)
	rr := do(h, http.MethodGet, "/does-not-exist")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	msg := decodeError(t, rr.Body.Bytes())
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

// ── queryDocs mode=any ────────────────────────────────────────────────────────

// Test 27: queryDocs mode=any is accepted and returns 200
func TestQueryDocs_ModeAny_Returns200(t *testing.T) {
	h := newTestServer(t, func(ctx context.Context, st *sqlite.SQLiteStore) {
		seedLibrary(t, ctx, st)
	})
	rr := do(h, http.MethodGet, "/api/v1/docs?libraryId=wordpress-6.5&query=posts&mode=any&format=json")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
}

// ── queryDocs markdown empty-result fallback ──────────────────────────────────

// Test 28: queryDocs markdown with no results produces a human-readable "no results" message
func TestQueryDocs_MarkdownNoResults_FallbackMessage(t *testing.T) {
	h := newTestServer(t, func(ctx context.Context, st *sqlite.SQLiteStore) {
		seedLibrary(t, ctx, st)
	})
	// A query that will not match anything in the seeded data.
	rr := do(h, http.MethodGet, "/api/v1/docs?libraryId=wordpress-6.5&query=zzzunmatchable")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	// The handler's fallback message includes "No results found".
	if !strings.Contains(body, "No results found") {
		t.Errorf("expected 'No results found' fallback in body, got: %s", body)
	}
}

// ── writeJSON does not mangle URLs (SetEscapeHTML=false) ─────────────────────

// Test 29: writeJSON uses SetEscapeHTML(false) — '&' and '<' must NOT be
// JSON-unicode-escaped in the output (critical for URLs in documentation snippets).
//
// Go's json.Encoder with EscapeHTML=true (default) replaces '&' with the
// literal 6-char sequence & and '<' with <, which mangles URLs.
// With SetEscapeHTML(false) they remain as literal UTF-8 bytes.
func TestWriteJSON_HTMLEscapingDisabled(t *testing.T) {
	rr := httptest.NewRecorder()
	payload := map[string]string{
		"url": "https://example.com/api?foo=1&bar=2",
		"doc": "<WP_Query>",
	}
	writeJSON(rr, http.StatusOK, payload)
	body := rr.Body.String()

	// With EscapeHTML=true the encoder emits the JSON unicode escape sequences
	// & (for &) and < (for <). These must be absent with EscapeHTML=false.
	if strings.Contains(body, "\\u0026") {
		t.Errorf("writeJSON escaped '&' as \\u0026; body: %s", body)
	}
	if strings.Contains(body, "\\u003c") {
		t.Errorf("writeJSON escaped '<' as \\u003c; body: %s", body)
	}
	// The literal characters must be present unmodified.
	if !strings.Contains(body, "&") {
		t.Errorf("body is missing literal '&'; body: %s", body)
	}
	if !strings.Contains(body, "<") {
		t.Errorf("body is missing literal '<'; body: %s", body)
	}
}

// ── writeError shape ──────────────────────────────────────────────────────────

// Test 30: writeError produces exactly {"error": msg} with correct status.
func TestWriteError_ProducesExpectedShape(t *testing.T) {
	cases := []struct {
		status int
		msg    string
	}{
		{http.StatusBadRequest, "bad input"},
		{http.StatusNotFound, "not found"},
		{http.StatusInternalServerError, "internal server error"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeError(rr, tc.status, tc.msg)
			if rr.Code != tc.status {
				t.Errorf("status = %d, want %d", rr.Code, tc.status)
			}
			msg := decodeError(t, rr.Body.Bytes())
			if msg != tc.msg {
				t.Errorf("error = %q, want %q", msg, tc.msg)
			}
		})
	}
}
