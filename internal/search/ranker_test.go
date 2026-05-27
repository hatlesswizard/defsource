//go:build sqlite_fts5 || fts5

package search

import (
	"testing"

	"github.com/hatlesswizard/defsource/internal/store"
)

// makeLib is a convenience helper that builds a LibraryRecord with the
// fields relevant to ranking tests. Fields not relevant to a specific test
// case are left at their zero values.
func makeLib(id, name string, snippetCount int, trustScore float64) store.LibraryRecord {
	return store.LibraryRecord{
		ID:           id,
		Name:         name,
		SnippetCount: snippetCount,
		TrustScore:   trustScore,
	}
}

// TestRankLibraries_ExactMatch verifies that a library whose name exactly
// matches the query receives a base score of 100 and therefore sorts first.
func TestRankLibraries_ExactMatch(t *testing.T) {
	libs := []store.LibraryRecord{
		makeLib("wordpress", "WordPress", 0, 0),
		makeLib("wp-query", "WP_Query helper", 0, 0),
	}
	got := RankLibraries("wordpress", libs)
	if len(got) != 2 {
		t.Fatalf("RankLibraries returned %d results, want 2", len(got))
	}
	// "WordPress" == "wordpress" (case-insensitive) → score 100.
	// "WP_Query helper" does not match → score 0.
	if got[0].ID != "wordpress" {
		t.Errorf("got[0].ID = %q, want \"wordpress\" (exact match must be first)", got[0].ID)
	}
}

// TestRankLibraries_TierOrdering verifies that exact (100) > prefix (80) >
// name-contains (60) > id-contains (40) when snippet count and trust score
// are identical across all libraries.
func TestRankLibraries_TierOrdering(t *testing.T) {
	// All libraries have zero SnippetCount and TrustScore so the only
	// differentiator is the name-match tier score.
	libs := []store.LibraryRecord{
		// id-contains match (40): ID has "press" but name does not
		makeLib("wordpress-core", "Core Library", 0, 0),
		// prefix match (80): name starts with "press"
		makeLib("pressbooks", "pressbooks plugin", 0, 0),
		// exact match (100): name == query
		makeLib("press", "press", 0, 0),
		// name-contains match (60): name contains "press" but doesn't start with it
		makeLib("bluepress", "The press system", 0, 0),
	}
	got := RankLibraries("press", libs)
	if len(got) != 4 {
		t.Fatalf("RankLibraries returned %d results, want 4", len(got))
	}

	wantOrder := []string{"press", "pressbooks", "bluepress", "wordpress-core"}
	for i, wantID := range wantOrder {
		if got[i].ID != wantID {
			t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, wantID)
		}
	}
}

// TestRankLibraries_TiebreakerSnippetCount verifies that when two libraries
// have the same name-match tier score, the one with a higher SnippetCount
// ranks first (via the SnippetCount × 0.01 tiebreaker).
func TestRankLibraries_TiebreakerSnippetCount(t *testing.T) {
	// Both libraries have a name that starts with the query ("wp" prefix → 80),
	// both have TrustScore 0. The only difference is SnippetCount.
	libs := []store.LibraryRecord{
		makeLib("wp-blocks", "wp-blocks", 10, 0), // score = 80 + 0.10
		makeLib("wp-cron", "wp-cron", 500, 0),    // score = 80 + 5.00
		makeLib("wp-api", "wp-api", 50, 0),       // score = 80 + 0.50
	}
	got := RankLibraries("wp", libs)
	if len(got) != 3 {
		t.Fatalf("RankLibraries returned %d results, want 3", len(got))
	}
	if got[0].ID != "wp-cron" {
		t.Errorf("got[0].ID = %q, want \"wp-cron\" (highest SnippetCount must be first)", got[0].ID)
	}
	if got[1].ID != "wp-api" {
		t.Errorf("got[1].ID = %q, want \"wp-api\"", got[1].ID)
	}
	if got[2].ID != "wp-blocks" {
		t.Errorf("got[2].ID = %q, want \"wp-blocks\"", got[2].ID)
	}
}

// TestRankLibraries_TiebreakerTrustScore verifies that when two libraries
// have the same name-match tier score AND the same SnippetCount, the one
// with a higher TrustScore ranks first (via the TrustScore × 10 tiebreaker).
func TestRankLibraries_TiebreakerTrustScore(t *testing.T) {
	// Both start with query "acme" → prefix match (80). Same SnippetCount (0).
	// Only TrustScore differs.
	libs := []store.LibraryRecord{
		makeLib("acme-low", "acme-low plugin", 0, 0.1),   // score = 80 + 0 + 1.0
		makeLib("acme-high", "acme-high plugin", 0, 0.9), // score = 80 + 0 + 9.0
	}
	got := RankLibraries("acme", libs)
	if len(got) != 2 {
		t.Fatalf("RankLibraries returned %d results, want 2", len(got))
	}
	if got[0].ID != "acme-high" {
		t.Errorf("got[0].ID = %q, want \"acme-high\" (higher TrustScore must be first)", got[0].ID)
	}
	if got[1].ID != "acme-low" {
		t.Errorf("got[1].ID = %q, want \"acme-low\"", got[1].ID)
	}
}

// TestRankLibraries_ZeroScoreLibrariesIncluded verifies that libraries which
// match on none of the four tiers (score == 0 from name/id tests) are still
// included in the returned slice and sorted after matching libraries.
func TestRankLibraries_ZeroScoreLibrariesIncluded(t *testing.T) {
	libs := []store.LibraryRecord{
		makeLib("zz-unrelated", "completely unrelated", 0, 0),
		makeLib("wordpress", "WordPress", 0, 0),
	}
	got := RankLibraries("wordpress", libs)
	if len(got) != 2 {
		t.Fatalf("RankLibraries returned %d results, want 2 (zero-score libs must be included)", len(got))
	}
	if got[0].ID != "wordpress" {
		t.Errorf("got[0].ID = %q, want \"wordpress\"", got[0].ID)
	}
}

// TestRankLibraries_EmptyInput verifies that an empty library slice returns
// an empty (non-nil) slice without panicking.
func TestRankLibraries_EmptyInput(t *testing.T) {
	got := RankLibraries("anything", []store.LibraryRecord{})
	if got == nil {
		t.Fatal("RankLibraries(empty) returned nil, want non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("RankLibraries(empty) returned %d items, want 0", len(got))
	}
}
