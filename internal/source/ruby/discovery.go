package ruby

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codebaseIndex holds the results of walking and parsing all Ruby files
// in the repository. It maps entity names to their file locations and
// collects call reference counts for prioritisation.
type codebaseIndex struct {
	definedClasses   map[string]string // qualifiedName -> filePath
	definedModules   map[string]string // qualifiedName -> filePath
	definedFunctions map[string]string // name -> filePath
	refCounts        map[string]int
	sourceRoots      []string
	fileAnalyses     map[string]*fileAnalysis
}

// emptyIndex returns a new, fully-initialised (but empty) codebaseIndex.
// Used by New() so that DetectWrapper and ResolveWrapperURL are safe to call
// before DiscoverEntities has been run.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		definedClasses:   map[string]string{},
		definedModules:   map[string]string{},
		definedFunctions: map[string]string{},
		refCounts:        map[string]int{},
		fileAnalyses:     map[string]*fileAnalysis{},
	}
}

// FileForClass returns the file path for a class with the given name.
func (idx *codebaseIndex) FileForClass(name string) (string, bool) {
	v, ok := idx.definedClasses[name]
	return v, ok
}

// FileForModule returns the file path for a module with the given name.
func (idx *codebaseIndex) FileForModule(name string) (string, bool) {
	v, ok := idx.definedModules[name]
	return v, ok
}

// FileForFunction returns the file path for a top-level function.
func (idx *codebaseIndex) FileForFunction(name string) (string, bool) {
	v, ok := idx.definedFunctions[name]
	return v, ok
}

// LookupClass performs a case-insensitive lookup for a class, returning the
// file path and canonical name.
func (idx *codebaseIndex) LookupClass(name string) (path, canonical string, ok bool) {
	if v, exact := idx.definedClasses[name]; exact {
		return v, name, true
	}
	lower := strings.ToLower(name)
	for k, v := range idx.definedClasses {
		if strings.ToLower(k) == lower {
			return v, k, true
		}
	}
	return "", "", false
}

// LookupModule performs a case-insensitive lookup for a module.
func (idx *codebaseIndex) LookupModule(name string) (path, canonical string, ok bool) {
	if v, exact := idx.definedModules[name]; exact {
		return v, name, true
	}
	lower := strings.ToLower(name)
	for k, v := range idx.definedModules {
		if strings.ToLower(k) == lower {
			return v, k, true
		}
	}
	return "", "", false
}

// HasClass reports whether the index contains a class.
func (idx *codebaseIndex) HasClass(name string) bool {
	_, ok := idx.definedClasses[name]
	return ok
}

// HasModule reports whether the index contains a module.
func (idx *codebaseIndex) HasModule(name string) bool {
	_, ok := idx.definedModules[name]
	return ok
}

// skipDirs contains directory names to skip during discovery.
var skipDirs = map[string]bool{
	".git":         true,
	"vendor":       true,
	"node_modules": true,
	"test":         true,
	"tests":        true,
	"spec":         true,
	"features":     true,
	"benchmark":    true,
	"benchmarks":   true,
	"examples":     true,
	"example":      true,
}

// buildCodebaseIndex walks the given roots, parsing all .rb files, and
// builds an index of classes, modules, and functions.
func buildCodebaseIndex(repoPath string, roots []string) (*codebaseIndex, error) {
	idx := emptyIndex()

	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("WARNING: discovery root %q does not exist, skipping", root)
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			log.Printf("WARNING: discovery root %q is not a directory, skipping", root)
			continue
		}

		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("WARNING: walking %s: %v", path, err)
				return nil
			}

			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}

			if !strings.HasSuffix(strings.ToLower(d.Name()), ".rb") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				log.Printf("WARNING: read %s: %v", path, err)
				return nil
			}

			analysis := parseFile(content)
			if analysis == nil {
				return nil
			}

			for _, cls := range analysis.Classes {
				// Only register the first definition (skip open-class redefs)
				if _, exists := idx.definedClasses[cls.QualifiedName]; !exists {
					idx.definedClasses[cls.QualifiedName] = path
				}
			}
			for _, mod := range analysis.Modules {
				if _, exists := idx.definedModules[mod.QualifiedName]; !exists {
					idx.definedModules[mod.QualifiedName] = path
				}
			}
			for _, fn := range analysis.Functions {
				if _, exists := idx.definedFunctions[fn.Name]; !exists {
					idx.definedFunctions[fn.Name] = path
				}
			}
			idx.fileAnalyses[path] = analysis
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %s: %w", root, walkErr)
		}
	}

	// Build reference counts from call sites
	for _, analysis := range idx.fileAnalyses {
		seen := map[string]bool{}
		for _, call := range analysis.Calls {
			if !seen[call.Name] {
				idx.refCounts[call.Name]++
				seen[call.Name] = true
			}
		}
	}

	log.Printf("Indexed %d classes, %d modules, %d functions from Ruby source",
		len(idx.definedClasses), len(idx.definedModules), len(idx.definedFunctions))

	return idx, nil
}

// priority returns a sort priority based on reference count (lower = higher priority).
func (idx *codebaseIndex) priority(name string) int {
	refs := idx.refCounts[name]
	if refs >= 50 {
		return 0
	}
	if refs >= 10 {
		return 1
	}
	return 2
}

// buildEntityList returns a sorted list of entity identifiers for crawling.
func (idx *codebaseIndex) buildEntityList() []string {
	type item struct {
		id       string
		priority int
		sortKey  string
	}

	var items []item
	for cls, path := range idx.definedClasses {
		items = append(items, item{
			id:       path + "#" + cls,
			priority: idx.priority(cls),
			sortKey:  strings.ToLower(cls),
		})
	}
	for mod, path := range idx.definedModules {
		items = append(items, item{
			id:       path + "#" + mod,
			priority: idx.priority(mod),
			sortKey:  strings.ToLower(mod),
		})
	}
	for fn, path := range idx.definedFunctions {
		items = append(items, item{
			id:       path + "#" + fn,
			priority: idx.priority(fn),
			sortKey:  strings.ToLower(fn),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].priority != items[j].priority {
			return items[i].priority < items[j].priority
		}
		return items[i].sortKey < items[j].sortKey
	})

	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if seen[it.id] {
			continue
		}
		seen[it.id] = true
		out = append(out, it.id)
	}
	return out
}
