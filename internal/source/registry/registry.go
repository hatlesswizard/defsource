package registry

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/hatlesswizard/defsource/internal/source"
)

var ErrSourceNotFound = errors.New("source not registered")

type SourceFactory func(repoPath string, opts FactoryOptions) (source.Source, error)

type FactoryOptions struct {
	Ref     string
	Version string
}

type Registry struct {
	mu        sync.RWMutex
	factories map[string]SourceFactory
	aliases   map[string]string
}

func New() *Registry {
	return &Registry{
		factories: make(map[string]SourceFactory),
		aliases:   make(map[string]string),
	}
}

var Default = New()

func (r *Registry) Register(id string, f SourceFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[id] = f
}

func (r *Registry) Alias(alias, canonicalID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aliases[alias] = canonicalID
}

func (r *Registry) Create(id string, repoPath string, opts FactoryOptions) (source.Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if canonical, ok := r.aliases[id]; ok {
		id = canonical
	}
	f, ok := r.factories[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrSourceNotFound, id)
	}
	return f(repoPath, opts)
}

func (r *Registry) Get(id string) (SourceFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if canonical, ok := r.aliases[id]; ok {
		id = canonical
	}
	f, ok := r.factories[id]
	return f, ok
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.factories))
	for id := range r.factories {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (r *Registry) ListByLanguage(language string) []string {
	prefix := language + "/"
	r.mu.RLock()
	defer r.mu.RUnlock()
	var ids []string
	for id := range r.factories {
		if len(id) > len(prefix) && id[:len(prefix)] == prefix {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// LanguageInfo holds the name of a language and the count of registered frameworks.
type LanguageInfo struct {
	Language       string `json:"language"`
	FrameworkCount int    `json:"framework_count"`
}

// Languages returns all distinct languages with their framework counts,
// sorted alphabetically by language name.
func (r *Registry) Languages() []LanguageInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := make(map[string]int)
	for id := range r.factories {
		if idx := indexOf(id, '/'); idx > 0 {
			counts[id[:idx]]++
		}
	}
	result := make([]LanguageInfo, 0, len(counts))
	for lang, count := range counts {
		result = append(result, LanguageInfo{Language: lang, FrameworkCount: count})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Language < result[j].Language
	})
	return result
}

// indexOf returns the index of the first occurrence of sep in s, or -1.
func indexOf(s string, sep byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return i
		}
	}
	return -1
}
