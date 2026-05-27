package search

import (
	"sort"
	"strings"

	"github.com/hatlesswizard/defsource/internal/store"
)

// RankLibraries scores and sorts libs by relevance to query, returning a new
// slice ordered highest-score first. Scoring uses a four-tier name-match
// system: exact match (100 points), prefix match (80), substring match in
// the library name (60), or substring match in the library ID (40). Libraries
// that score zero on the name tiers are still included but ranked last.
//
// Two continuous tiebreakers are added on top of the tier score:
//   - SnippetCount × 0.01: larger, more fully-indexed libraries rank slightly
//     higher when name-match quality is equal.
//   - TrustScore × 10: explicitly curated quality signal overrides snippet
//     volume when present.
//
// The sort is not stable; libraries with identical total scores may appear in
// any order between calls.
func RankLibraries(query string, libs []store.LibraryRecord) []store.LibraryRecord {
	queryLower := strings.ToLower(query)
	type scored struct {
		lib   store.LibraryRecord
		score float64
	}
	var items []scored
	for _, lib := range libs {
		s := 0.0
		nameLower := strings.ToLower(lib.Name)
		idLower := strings.ToLower(lib.ID)

		if nameLower == queryLower {
			s += 100
		} else if strings.HasPrefix(nameLower, queryLower) {
			s += 80
		} else if strings.Contains(nameLower, queryLower) {
			s += 60
		} else if strings.Contains(idLower, queryLower) {
			s += 40
		}
		s += float64(lib.SnippetCount) * 0.01
		s += lib.TrustScore * 10
		items = append(items, scored{lib: lib, score: s})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})
	result := make([]store.LibraryRecord, len(items))
	for i, item := range items {
		result[i] = item.lib
	}
	return result
}
