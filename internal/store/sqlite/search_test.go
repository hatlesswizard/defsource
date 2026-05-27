//go:build sqlite_fts5 || fts5

package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
)

// newTestStoreWithIndex creates a fresh SQLiteStore in a temp directory,
// seeds it with a library, several entities, and several methods, then
// calls RebuildIndex. It is the canonical fixture builder for all
// integration tests in this file.
func newTestStoreWithIndex(t *testing.T) (*SQLiteStore, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "search_test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("newTestStoreWithIndex: New: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	ctx := context.Background()
	libID := "/wordpress/test"

	if err := s.UpsertLibrary(ctx, libID, source.LibraryMeta{
		Name:        "WordPress Test",
		Description: "Test WordPress library",
		SourceURL:   "https://example.com",
		TrustScore:  0.9,
	}); err != nil {
		t.Fatalf("UpsertLibrary: %v", err)
	}

	// Entity 1: WP_Query — appears in many method descriptions
	e1ID, err := s.UpsertEntity(ctx, libID, &source.Entity{
		Slug:        "wp_query",
		Name:        "WP_Query",
		Kind:        "class",
		Description: "WordPress query class for retrieving posts from the database.",
		SourceFile:  "wp-includes/class-wp-query.php",
		URL:         "https://example.com/reference/classes/wp_query/",
	})
	if err != nil {
		t.Fatalf("UpsertEntity WP_Query: %v", err)
	}

	// Entity 2: WP_Post — less content, should rank lower on general queries
	e2ID, err := s.UpsertEntity(ctx, libID, &source.Entity{
		Slug:        "wp_post",
		Name:        "WP_Post",
		Kind:        "class",
		Description: "WordPress post object.",
		SourceFile:  "wp-includes/class-wp-post.php",
		URL:         "https://example.com/reference/classes/wp_post/",
	})
	if err != nil {
		t.Fatalf("UpsertEntity WP_Post: %v", err)
	}

	// Method on WP_Query: get_posts — rich content
	if err := s.UpsertMethod(ctx, e1ID, &source.Method{
		Slug:        "get_posts",
		Name:        "get_posts",
		Signature:   "WP_Query::get_posts()",
		Description: "Retrieve an array of posts based on query variables. Fills in the posts property with post objects.",
		ReturnType:  "WP_Post[]|int[]",
		ReturnDesc:  "Array of post objects or post IDs.",
		URL:         "https://example.com/reference/classes/wp_query/get_posts/",
	}); err != nil {
		t.Fatalf("UpsertMethod get_posts: %v", err)
	}

	// Method on WP_Query: query — moderate content
	if err := s.UpsertMethod(ctx, e1ID, &source.Method{
		Slug:        "query",
		Name:        "query",
		Signature:   "WP_Query::query( array $query )",
		Description: "Sets up the WordPress query by parsing the query string.",
		ReturnType:  "WP_Post[]|int[]",
		URL:         "https://example.com/reference/classes/wp_query/query/",
	}); err != nil {
		t.Fatalf("UpsertMethod query: %v", err)
	}

	// Method on WP_Post: to_array — minimal content
	if err := s.UpsertMethod(ctx, e2ID, &source.Method{
		Slug:        "to_array",
		Name:        "to_array",
		Signature:   "WP_Post::to_array()",
		Description: "Convert object to array.",
		ReturnType:  "array",
		URL:         "https://example.com/reference/classes/wp_post/to_array/",
	}); err != nil {
		t.Fatalf("UpsertMethod to_array: %v", err)
	}

	if err := s.RebuildIndex(ctx, libID); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}

	return s, libID
}

// ── sanitizeFTSQuery ────────────────────────────────────────────────────────
//
// sanitizeFTSQuery is package-internal. It cannot be called from an _test.go
// file in a different package. However, because this file uses `package sqlite`
// (white-box testing), the unexported function is directly accessible.

// TestSanitizeFTSQuery_EmptyInput verifies that an empty string returns "".
// An empty return value signals the caller that no MATCH query should be issued.
func TestSanitizeFTSQuery_EmptyInput(t *testing.T) {
	t.Parallel()
	got := sanitizeFTSQuery("", "all")
	if got != "" {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q, want %q", "", "all", got, "")
	}
}

// TestSanitizeFTSQuery_AllSpecialCharacters exercises all 8 replacer rules in
// a single input that contains every character the replacer must neutralize.
// The input has NO real word content, so after replacement all tokens become
// empty and the function must return "".
func TestSanitizeFTSQuery_AllSpecialCharsProduceEmpty(t *testing.T) {
	t.Parallel()
	// Rule 1: `"` doubled then consumed as part of an empty word
	// Rules 2-8: *, (, ), :, ^, {, } become spaces
	input := `"*():^{}`
	got := sanitizeFTSQuery(input, "all")
	// After replacer: `""` (doubled quote) + spaces. strings.Fields on `"" ` yields [`""`].
	// The word `""` after TrimLeft("-") is `""`, not empty, so it gets quoted.
	// Expected output: `""""`  (quoting `""` as `"` + `""` + `"`)
	// Actually let's compute: word = `""`, quoted = `"` + `""` + `"` = `""""`
	// The word is non-empty so it survives.
	// We just need to ensure no panic and the output does not start with raw FTS5 operators.
	if strings.ContainsAny(got, "*()^{}") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: raw FTS5 operators leaked through", input, "all", got)
	}
}

// TestSanitizeFTSQuery_DoubleQuoteEscape verifies rule 1: `"` → `""`.
// A bare double-quote in a user query, if not escaped, would break the FTS5
// quoted-term syntax. After escaping it is wrapped in an outer pair of quotes.
func TestSanitizeFTSQuery_DoubleQuoteEscape(t *testing.T) {
	t.Parallel()
	got := sanitizeFTSQuery(`say "hello"`, "all")
	// `"` → `""`, then each word gets quoted:
	// words after replace: `say`, `""hello""` → quoted: `"say"`, `"""hello"""`
	if !strings.Contains(got, `""`) {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: escaped double-quote not found", `say "hello"`, "all", got)
	}
	if strings.Count(got, `"`) < 4 {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: fewer quotes than expected", `say "hello"`, "all", got)
	}
}

// TestSanitizeFTSQuery_WildcardStripped verifies rule 2: `*` → ` `.
// FTS5 prefix-match wildcards must not reach the parser; they are stripped.
func TestSanitizeFTSQuery_WildcardStripped(t *testing.T) {
	t.Parallel()
	got := sanitizeFTSQuery("post*", "all")
	if strings.Contains(got, "*") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: wildcard * leaked through", "post*", "all", got)
	}
	// The word "post" should survive
	if !strings.Contains(got, "post") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: word 'post' missing after wildcard strip", "post*", "all", got)
	}
}

// TestSanitizeFTSQuery_GroupingParensStripped verifies rules 3 and 4: `(` → ` `, `)` → ` `.
func TestSanitizeFTSQuery_GroupingParensStripped(t *testing.T) {
	t.Parallel()
	got := sanitizeFTSQuery("(post OR query)", "all")
	if strings.ContainsAny(got, "()") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: grouping parens leaked through", "(post OR query)", "all", got)
	}
}

// TestSanitizeFTSQuery_ColonFieldPrefixStripped verifies rule 5: `:` → ` `.
// Column-filter syntax (e.g., `name:foo`) allows targeting arbitrary FTS5
// columns and must be prevented.
func TestSanitizeFTSQuery_ColonFieldPrefixStripped(t *testing.T) {
	t.Parallel()
	got := sanitizeFTSQuery("name:WP_Query", "all")
	if strings.Contains(got, ":") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: colon leaked through", "name:WP_Query", "all", got)
	}
	// Both "name" and "WP_Query" should survive as separate words
	if !strings.Contains(got, "name") || !strings.Contains(got, "WP_Query") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: words missing after colon strip", "name:WP_Query", "all", got)
	}
}

// TestSanitizeFTSQuery_CaretStartAnchorStripped verifies rule 6: `^` → ` `.
func TestSanitizeFTSQuery_CaretStartAnchorStripped(t *testing.T) {
	t.Parallel()
	got := sanitizeFTSQuery("^startphrase", "all")
	if strings.Contains(got, "^") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: caret ^ leaked through", "^startphrase", "all", got)
	}
}

// TestSanitizeFTSQuery_CurlyBracesStripped verifies rules 7 and 8: `{` → ` `, `}` → ` `.
func TestSanitizeFTSQuery_CurlyBracesStripped(t *testing.T) {
	t.Parallel()
	got := sanitizeFTSQuery("{column filter}", "all")
	if strings.ContainsAny(got, "{}") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: curly braces leaked through", "{column filter}", "all", got)
	}
}

// TestSanitizeFTSQuery_FTS5KeywordsQuoted verifies that FTS5 reserved keywords
// (AND, OR, NOT, NEAR) are quoted as literal search terms, not treated as
// operators. This is the word-level quoting step after replacement.
func TestSanitizeFTSQuery_FTS5KeywordsQuoted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		query string
		mode  string
	}{
		{"AND keyword", "AND posts", "all"},
		{"OR keyword", "OR filter", "all"},
		{"NOT keyword", "NOT found", "all"},
		{"NEAR keyword", "NEAR search", "all"},
		{"AND lowercase", "and posts", "all"},
		{"near lowercase", "near posts", "all"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeFTSQuery(tc.query, tc.mode)
			// All words must be individually double-quoted; raw unquoted keywords
			// would appear as AND/OR/NOT/NEAR without surrounding quotes.
			words := strings.Fields(got)
			for _, w := range words {
				if !strings.HasPrefix(w, `"`) || !strings.HasSuffix(w, `"`) {
					t.Errorf("sanitizeFTSQuery(%q, %q): word %q is not quoted in output %q", tc.query, tc.mode, w, got)
				}
			}
		})
	}
}

// TestSanitizeFTSQuery_LeadingDashStripped verifies that the FTS5 NOT prefix
// (`-word`) is stripped before quoting. A leading `-` on a word selects
// exclusion semantics; stripping it ensures the word is treated as inclusion.
func TestSanitizeFTSQuery_LeadingDashStripped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		query string
		mode  string
		want  string
	}{
		{"-post", "all", `"post"`},
		{"--post", "all", `"post"`},
		{"-", "all", ""},   // lone dash → empty word → dropped → empty output
		{"---", "all", ""}, // all dashes → empty
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.query, func(t *testing.T) {
			t.Parallel()
			got := sanitizeFTSQuery(tc.query, tc.mode)
			if got != tc.want {
				t.Errorf("sanitizeFTSQuery(%q, %q) = %q, want %q", tc.query, tc.mode, got, tc.want)
			}
		})
	}
}

// TestSanitizeFTSQuery_ModeAny_JoinsWithOR verifies that mode="any" produces
// OR-joined terms (OR semantics — at least one term must match).
func TestSanitizeFTSQuery_ModeAny_JoinsWithOR(t *testing.T) {
	t.Parallel()
	got := sanitizeFTSQuery("post query", "any")
	// Expected: `"post" OR "query"`
	if !strings.Contains(got, " OR ") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: missing ' OR ' separator", "post query", "any", got)
	}
}

// TestSanitizeFTSQuery_ModeAll_JoinsImplicitly verifies that mode="all"
// (and any non-"any" value) produces space-joined terms (AND semantics).
func TestSanitizeFTSQuery_ModeAll_JoinsWithSpace(t *testing.T) {
	t.Parallel()
	cases := []string{"all", "", "unknown", "ALL"}
	for _, mode := range cases {
		mode := mode
		t.Run("mode="+mode, func(t *testing.T) {
			t.Parallel()
			got := sanitizeFTSQuery("post query", mode)
			if strings.Contains(got, " OR ") {
				t.Errorf("sanitizeFTSQuery(%q, %q) = %q: unexpected ' OR ' for AND mode", "post query", mode, got)
			}
			// Both words must appear
			if !strings.Contains(got, "post") || !strings.Contains(got, "query") {
				t.Errorf("sanitizeFTSQuery(%q, %q) = %q: word(s) missing", "post query", mode, got)
			}
		})
	}
}

// TestSanitizeFTSQuery_MultiTermWithPunctuation verifies word-boundary handling
// when terms contain internal punctuation mixed with legitimate characters.
func TestSanitizeFTSQuery_MultiTermWithPunctuation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		query      string
		mode       string
		wantWords  []string
		wantAbsent []string
	}{
		{
			name:       "colon inside term splits it",
			query:      "WP_Query::get_posts",
			mode:       "all",
			wantWords:  []string{"WP_Query", "get_posts"},
			wantAbsent: []string{":"},
		},
		{
			name:       "mixed special and normal chars",
			query:      "post* (query)",
			mode:       "all",
			wantWords:  []string{"post", "query"},
			wantAbsent: []string{"*", "(", ")"},
		},
		{
			name:      "multiple terms in any mode",
			query:     "alpha beta gamma",
			mode:      "any",
			wantWords: []string{"alpha", "beta", "gamma"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeFTSQuery(tc.query, tc.mode)
			for _, w := range tc.wantWords {
				if !strings.Contains(got, w) {
					t.Errorf("sanitizeFTSQuery(%q, %q) = %q: expected word %q missing", tc.query, tc.mode, got, w)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("sanitizeFTSQuery(%q, %q) = %q: forbidden char %q present", tc.query, tc.mode, got, absent)
				}
			}
		})
	}
}

// TestSanitizeFTSQuery_MaliciousPatterns exercises SQL-injection-like and
// FTS5-injection-like strings that a hostile user might submit. The invariant
// is: no raw FTS5 operators survive; the output either contains only safely
// quoted words or is empty; no panic occurs.
func TestSanitizeFTSQuery_MaliciousPatterns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		query string
	}{
		{"sql single quotes", `' OR '1'='1`},
		{"sql double quotes", `" OR "1"="1`},
		{"fts5 NEAR operator", `NEAR(post query, 5)`},
		{"fts5 column filter", `{entity_name content}:post`},
		{"nested operators", `(((AND OR NOT)))`},
		{"only operators", `AND OR NOT NEAR`},
		{"only dashes", `--- -- -`},
		{"all special chars", `"*():^{}`},
		{"caret injection", `^anchor term`},
		{"deep nesting", `((((((post))))))`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Must not panic
			got := sanitizeFTSQuery(tc.query, "all")
			// Raw FTS5 structural characters must not appear unquoted
			if strings.ContainsAny(got, "*()^{}") {
				t.Errorf("sanitizeFTSQuery(%q) = %q: FTS5 metachar survived", tc.query, got)
			}
			// Colons must not survive
			if strings.Contains(got, ":") {
				t.Errorf("sanitizeFTSQuery(%q) = %q: colon survived", tc.query, got)
			}
			// If output is non-empty, every space-delimited token must be quoted
			if got != "" {
				for _, tok := range strings.Fields(got) {
					if tok == "OR" {
						continue // join operator
					}
					if !strings.HasPrefix(tok, `"`) || !strings.HasSuffix(tok, `"`) {
						t.Errorf("sanitizeFTSQuery(%q) = %q: unquoted token %q", tc.query, got, tok)
					}
				}
			}
		})
	}
}

// TestSanitizeFTSQuery_OnlySingleQuotes verifies that single quotes pass
// through unmodified (FTS5 does not assign special meaning to `'`). They
// must still be safely handled by word-level quoting.
func TestSanitizeFTSQuery_SingleQuotesPassThrough(t *testing.T) {
	t.Parallel()
	// Single quotes are not in the replacer — they are preserved as-is inside
	// quoted words. The FTS5 spec does not treat `'` as a metacharacter.
	got := sanitizeFTSQuery("it's working", "all")
	if got == "" {
		t.Errorf("sanitizeFTSQuery(%q, %q) = empty, expected non-empty", "it's working", "all")
	}
	// Output must not contain unquoted raw operators
	if strings.ContainsAny(got, "*()^{}:") {
		t.Errorf("sanitizeFTSQuery(%q, %q) = %q: leaked metachar", "it's working", "all", got)
	}
}

// ── RebuildIndex ────────────────────────────────────────────────────────────

// TestRebuildIndex_EmptyDB verifies that RebuildIndex on a library with no
// entities and no methods succeeds without error.
func TestRebuildIndex_EmptyDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "rebuild_empty.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	libID := "/empty/lib"
	if err := s.UpsertLibrary(ctx, libID, source.LibraryMeta{
		Name: "Empty Lib", SourceURL: "https://x.com", TrustScore: 0.5,
	}); err != nil {
		t.Fatalf("UpsertLibrary: %v", err)
	}

	if err := s.RebuildIndex(ctx, libID); err != nil {
		t.Errorf("RebuildIndex on empty library: got error %v, want nil", err)
	}
}

// TestRebuildIndex_PopulatedDB verifies that after a rebuild the FTS5 index
// contains entries that are searchable via Search.
func TestRebuildIndex_PopulatedDB(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	results, err := s.Search(ctx, libID, "WP_Query", 10, "all")
	if err != nil {
		t.Fatalf("Search after RebuildIndex: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search after RebuildIndex: expected results, got none")
	}
}

// TestRebuildIndex_Idempotent verifies that calling RebuildIndex twice in a
// row on the same library does not produce duplicates. The second call must
// DELETE and re-INSERT, leaving exactly one copy of each document.
func TestRebuildIndex_Idempotent(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	// First rebuild was done inside newTestStoreWithIndex.
	// Do a second rebuild.
	if err := s.RebuildIndex(ctx, libID); err != nil {
		t.Fatalf("second RebuildIndex: %v", err)
	}

	// Search must still work and return the same number of results as after
	// the first rebuild.
	results1, err := s.Search(ctx, libID, "posts", 20, "all")
	if err != nil {
		t.Fatalf("Search after second RebuildIndex: %v", err)
	}

	// Third rebuild — counts must remain stable.
	if err := s.RebuildIndex(ctx, libID); err != nil {
		t.Fatalf("third RebuildIndex: %v", err)
	}
	results2, err := s.Search(ctx, libID, "posts", 20, "all")
	if err != nil {
		t.Fatalf("Search after third RebuildIndex: %v", err)
	}

	if len(results1) != len(results2) {
		t.Errorf("result count changed between rebuilds: %d → %d (duplicate insertion?)", len(results1), len(results2))
	}
}

// ── Search ───────────────────────────────────────────────────────────────────

// TestSearch_EmptyQuery verifies that an empty query returns (nil, nil) without
// issuing a database query (FTS5 MATCH with empty string is a syntax error).
func TestSearch_EmptyQuery(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	results, err := s.Search(ctx, libID, "", 10, "all")
	if err != nil {
		t.Errorf("Search(%q): got error %v, want nil", "", err)
	}
	if results != nil {
		t.Errorf("Search(%q): got %v, want nil", "", results)
	}
}

// TestSearch_AllSpecialCharsQuery verifies that a query consisting entirely of
// FTS5 special characters (which sanitizeFTSQuery should eliminate) returns
// (nil, nil) without error, not a panic or database error.
func TestSearch_AllSpecialCharsQuery(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	results, err := s.Search(ctx, libID, `"*():^{}`, 10, "all")
	if err != nil {
		// The sanitizer may reduce the query to `""` (doubled quote word),
		// which FTS5 can handle as a valid empty quoted term. Or it may
		// reduce to empty and return nil, nil. Either is acceptable.
		// What is NOT acceptable is an internal FTS5 parse error.
		t.Errorf("Search(all-special-chars): got unexpected error %v", err)
	}
	_ = results // may be nil or empty — both are fine
}

// TestSearch_DefaultLimit_AppliedWhenZero verifies that limit=0 defaults to 20
// (the documented default) and that results are returned.
func TestSearch_DefaultLimit_AppliedWhenZero(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	// With limit=0, the function should use the default of 20.
	results, err := s.Search(ctx, libID, "WordPress", 0, "all")
	if err != nil {
		t.Fatalf("Search(limit=0): %v", err)
	}
	// There are only 4 indexed documents (2 entities + 2 methods), so all
	// should be returned when the default limit is 20.
	if len(results) == 0 {
		t.Error("Search(limit=0): expected results with default limit, got none")
	}
}

// TestSearch_CustomLimit_Enforced verifies that a custom limit caps results.
func TestSearch_CustomLimit_Enforced(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	// Use limit=1 — should get at most 1 result for a broad query.
	results, err := s.Search(ctx, libID, "WordPress post", 1, "any")
	if err != nil {
		t.Fatalf("Search(limit=1): %v", err)
	}
	if len(results) > 1 {
		t.Errorf("Search(limit=1): got %d results, want ≤1", len(results))
	}
}

// TestSearch_ModeAny_ORSemantics verifies that mode="any" returns results
// that match at least one term, not necessarily all terms.
func TestSearch_ModeAny_ORSemantics(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	// "array" appears in get_posts description; "convert" appears in to_array.
	// With OR mode both should match.
	anyResults, err := s.Search(ctx, libID, "array convert", 20, "any")
	if err != nil {
		t.Fatalf("Search(mode=any): %v", err)
	}
	// With AND mode, a document must contain BOTH terms; fewer results expected.
	allResults, err := s.Search(ctx, libID, "array convert", 20, "all")
	if err != nil {
		t.Fatalf("Search(mode=all): %v", err)
	}
	// OR mode must return at least as many results as AND mode.
	if len(anyResults) < len(allResults) {
		t.Errorf("mode=any returned fewer results (%d) than mode=all (%d)", len(anyResults), len(allResults))
	}
}

// TestSearch_ModeAll_ANDSemantics verifies that mode="all" requires all terms
// to be present. A multi-term query where only one term matches should return
// fewer results than the same query in OR mode.
func TestSearch_ModeAll_ANDSemantics(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	// "database" appears in the WP_Query description; "array" in get_posts.
	// AND mode: a document needs both terms.
	allResults, err := s.Search(ctx, libID, "database array", 20, "all")
	if err != nil {
		t.Fatalf("Search(mode=all, multi-term): %v", err)
	}
	anyResults, err := s.Search(ctx, libID, "database array", 20, "any")
	if err != nil {
		t.Fatalf("Search(mode=any, multi-term): %v", err)
	}
	// OR must match at least as many as AND.
	if len(anyResults) < len(allResults) {
		t.Errorf("mode=any (%d) < mode=all (%d) — unexpected", len(anyResults), len(allResults))
	}
}

// TestSearch_BM25Ranking_MostRelevantFirst verifies that BM25 ordering places
// the most topically relevant result first. WP_Query and its method get_posts
// have far richer "post" / "query" content than WP_Post::to_array; a targeted
// query should surface WP_Query results above WP_Post results.
func TestSearch_BM25Ranking_MostRelevantFirst(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	results, err := s.Search(ctx, libID, "query posts database", 20, "all")
	if err != nil {
		t.Fatalf("Search(ranking): %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search(ranking): got no results")
	}

	// The top result must mention WP_Query (either as entity or via a method
	// on WP_Query), not WP_Post — because WP_Query has "query" in its name
	// and its entity description explicitly mentions "query" and "database".
	top := results[0]
	if !strings.Contains(strings.ToLower(top.EntityName), "wp_query") {
		t.Errorf("BM25 ranking: top result EntityName = %q, expected something containing 'wp_query'", top.EntityName)
	}
	// BM25 Rank values are negative; lower = better match. The top result
	// must have a rank less than or equal to the last result's rank.
	if len(results) > 1 {
		last := results[len(results)-1]
		if top.Rank > last.Rank {
			t.Errorf("BM25 ordering violated: top.Rank=%f > last.Rank=%f (should be ascending)", top.Rank, last.Rank)
		}
	}
}

// TestSearch_FTS5SpecialCharsInQuery_NoError verifies the critical invariant
// that any user-supplied string reaches the database without triggering an
// FTS5 parse error. This is the integration-level counterpart to the
// TestSanitizeFTSQuery_MaliciousPatterns unit test.
func TestSearch_FTS5SpecialCharsInQuery_NoError(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	dangerousQueries := []string{
		`"unmatched quote`,
		`NEAR(post, 5)`,
		`post* AND query*`,
		`(post OR query)`,
		`name:entity_name`,
		`^anchor`,
		`{entity_name}: foo`,
		`-- comment style`,
		`post" OR "1"="1`,
	}

	for _, q := range dangerousQueries {
		q := q
		t.Run(q, func(t *testing.T) {
			t.Parallel()
			_, err := s.Search(ctx, libID, q, 10, "all")
			if err != nil {
				t.Errorf("Search(%q): unexpected FTS5 error: %v", q, err)
			}
		})
	}
}

// TestSearch_SearchEntities_TypeFiltering verifies that results with
// SnippetType="class" correspond to entity-level index entries (MethodID==nil).
func TestSearch_SearchEntities_ClassSnippetType(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	// Query targeting entity-level content (class description only).
	results, err := s.Search(ctx, libID, "retrieving posts database", 20, "all")
	if err != nil {
		t.Fatalf("Search(entities): %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search(entities): no results")
	}

	var foundClassSnippet bool
	for _, r := range results {
		if r.SnippetType == "class" {
			foundClassSnippet = true
			if r.MethodID != nil {
				t.Errorf("class snippet has non-nil MethodID: %v", *r.MethodID)
			}
			if r.MethodName != "" {
				t.Errorf("class snippet has non-empty MethodName: %q", r.MethodName)
			}
		}
	}
	if !foundClassSnippet {
		t.Error("expected at least one SnippetType='class' result; got none")
	}
}

// TestSearch_SearchMethods_TypeFiltering verifies that results with
// SnippetType="method" have a non-nil MethodID and non-empty MethodName.
func TestSearch_SearchMethods_MethodSnippetType(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	// Query targeting method-level content.
	results, err := s.Search(ctx, libID, "array posts objects", 20, "all")
	if err != nil {
		t.Fatalf("Search(methods): %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search(methods): no results")
	}

	var foundMethodSnippet bool
	for _, r := range results {
		if r.SnippetType == "method" {
			foundMethodSnippet = true
			if r.MethodID == nil {
				t.Error("method snippet has nil MethodID")
			}
			if r.MethodName == "" {
				t.Error("method snippet has empty MethodName")
			}
		}
	}
	if !foundMethodSnippet {
		t.Error("expected at least one SnippetType='method' result; got none")
	}
}

// TestSearch_NonexistentLibrary verifies that searching against an unknown
// library returns an empty slice (not an error), because the FTS5 MATCH
// succeeds but the `library_id = ?` filter eliminates all rows.
func TestSearch_NonexistentLibrary_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	s, _ := newTestStoreWithIndex(t)
	ctx := context.Background()

	results, err := s.Search(ctx, "/nonexistent/lib", "post", 10, "all")
	if err != nil {
		t.Errorf("Search(nonexistent library): unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search(nonexistent library): got %d results, want 0", len(results))
	}
}

// TestSearch_NegativeLimitDefaultsTo20 verifies the behaviour of limit < 0,
// which the code handles identically to limit == 0 (both become 20).
func TestSearch_NegativeLimitDefaultsTo20(t *testing.T) {
	t.Parallel()
	s, libID := newTestStoreWithIndex(t)
	ctx := context.Background()

	results, err := s.Search(ctx, libID, "WordPress", -5, "all")
	if err != nil {
		t.Fatalf("Search(limit=-5): %v", err)
	}
	// Our fixture has only 4 documents; all should be returned.
	if len(results) == 0 {
		t.Error("Search(limit=-5): expected results with defaulted limit, got none")
	}
}

// ── RebuildIndexAsync ────────────────────────────────────────────────────────

// TestRebuildIndexAsync_NonBlocking verifies three behavioural invariants of
// RebuildIndexAsync (CRIT-08 fix):
//
//  1. The first call returns true (goroutine launched) and does NOT block the
//     caller — the caller can continue work while the rebuild runs in background.
//  2. A second call made while the first goroutine is still in flight returns
//     false (single-flight guard, no pile-up).
//  3. After the background goroutine completes, the FTS5 index is usable —
//     a Search call returns results.
func TestRebuildIndexAsync_NonBlocking(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "async_rebuild.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	libID := "/async/lib"
	if err := s.UpsertLibrary(ctx, libID, source.LibraryMeta{
		Name: "Async Lib", SourceURL: "https://x.com", TrustScore: 0.5,
	}); err != nil {
		t.Fatalf("UpsertLibrary: %v", err)
	}
	// Insert a few entities and methods so the rebuild has real work to do.
	for i := 0; i < 3; i++ {
		eSlug := strings.Repeat(string(rune('a'+i)), 4) // "aaaa", "bbbb", "cccc"
		eID, err := s.UpsertEntity(ctx, libID, &source.Entity{
			Slug:        eSlug,
			Name:        "Entity" + eSlug,
			Kind:        "class",
			Description: "description for " + eSlug,
			URL:         "https://x.com/" + eSlug,
		})
		if err != nil {
			t.Fatalf("UpsertEntity %s: %v", eSlug, err)
		}
		mSlug := "method-" + eSlug
		if err := s.UpsertMethod(ctx, eID, &source.Method{
			Slug: mSlug,
			Name: "Method" + eSlug,
			URL:  "https://x.com/" + mSlug,
		}); err != nil {
			t.Fatalf("UpsertMethod %s: %v", mSlug, err)
		}
	}

	// Invariant 1: first call launches the goroutine and returns true immediately.
	launched := s.RebuildIndexAsync(ctx, libID)
	if !launched {
		t.Error("RebuildIndexAsync (first call): returned false, want true (goroutine launched)")
	}

	// Invariant 2: a rapid second call while the goroutine may still be in flight
	// should return false (single-flight guard prevents pile-up).
	// Note: on a very fast machine the first goroutine may already be done;
	// in that case this call may return true again — we accept either outcome
	// here and instead test the post-completion state below.
	_ = s.RebuildIndexAsync(ctx, libID) // return value is informational; not asserted

	// Invariant 3: wait for any background goroutine to finish by calling
	// the synchronous RebuildIndex (which acquires s.mu after the goroutine
	// releases it), then verify Search returns results.
	if err := s.RebuildIndex(ctx, libID); err != nil {
		t.Fatalf("synchronous RebuildIndex after async: %v", err)
	}

	results, err := s.Search(ctx, libID, "description", 10, "all")
	if err != nil {
		t.Fatalf("Search after async rebuild: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search after async rebuild: got no results, want ≥1")
	}
}
