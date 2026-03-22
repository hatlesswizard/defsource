package search

import (
	"sort"
	"strings"

	"github.com/hatlesswizard/defsource/internal/store"
)

// RankLibraries scores library results for resolve-library-id.
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
