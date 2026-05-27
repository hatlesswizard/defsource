package wpgithub

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type codebaseIndex struct {
	definedClasses   map[string]string
	definedFunctions map[string]string
	refCounts        map[string]int
	builtinFunctions map[string]bool
	fileAnalyses     map[string]*fileAnalysis
}

// emptyIndex returns a new, fully-initialised (but empty) codebaseIndex.
// Used by New() so that DetectWrapper and ResolveWrapperURL are safe to call
// before DiscoverEntities has been run.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		definedClasses:   map[string]string{},
		definedFunctions: map[string]string{},
		refCounts:        map[string]int{},
		builtinFunctions: map[string]bool{},
		fileAnalyses:     map[string]*fileAnalysis{},
	}
}

// HasClass reports whether the index contains a class with the given name
// (exact-case match).
func (idx *codebaseIndex) HasClass(name string) bool {
	_, ok := idx.definedClasses[name]
	return ok
}

// FileForClass returns the file path for an exactly-named class and whether it
// was found.
func (idx *codebaseIndex) FileForClass(name string) (string, bool) {
	v, ok := idx.definedClasses[name]
	return v, ok
}

// LookupClass performs a case-insensitive lookup for a class, returning the
// file path and canonical (original-case) name. ok is false if not found.
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

// HasFunction reports whether the index contains a top-level function with
// the given name (exact-case match).
func (idx *codebaseIndex) HasFunction(name string) bool {
	_, ok := idx.definedFunctions[name]
	return ok
}

// FileForFunction returns the file path for a named top-level function and
// whether it was found.
func (idx *codebaseIndex) FileForFunction(name string) (string, bool) {
	v, ok := idx.definedFunctions[name]
	return v, ok
}

// IsBuiltin reports whether name was observed only as a callee — never defined
// in the indexed PHP files — and is therefore treated as a PHP built-in.
func (idx *codebaseIndex) IsBuiltin(name string) bool {
	return idx.builtinFunctions[name]
}

// RefCount returns the number of files that call name.
func (idx *codebaseIndex) RefCount(name string) int {
	return idx.refCounts[name]
}

func buildCodebaseIndex(repoPath string) (*codebaseIndex, error) {
	idx := emptyIndex()

	roots := []string{
		filepath.Join(repoPath, "wp-includes"),
		filepath.Join(repoPath, "wp-admin", "includes"),
	}

	skipDirs := map[string]bool{
		".git":         true,
		"vendor":       true,
		"node_modules": true,
	}

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

			if !strings.HasSuffix(strings.ToLower(d.Name()), ".php") {
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
				idx.definedClasses[cls.Name] = path
			}
			for _, fn := range analysis.Functions {
				idx.definedFunctions[fn.Name] = path
			}
			idx.fileAnalyses[path] = analysis
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %s: %w", root, walkErr)
		}
	}

	for _, analysis := range idx.fileAnalyses {
		seen := map[string]bool{}
		for _, call := range analysis.Calls {
			if call.Kind == "function" {
				if _, defined := idx.definedFunctions[call.Name]; !defined {
					idx.builtinFunctions[call.Name] = true
				}
			}
			if !seen[call.Name] {
				idx.refCounts[call.Name]++
				seen[call.Name] = true
			}
		}
	}

	log.Printf("Indexed %d classes, %d functions, detected %d builtins from %s",
		len(idx.definedClasses), len(idx.definedFunctions), len(idx.builtinFunctions), repoPath)

	return idx, nil
}

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

func (idx *codebaseIndex) buildEntityList() []string {
	type item struct {
		id       string
		priority int
		sortKey  string
	}

	var items []item
	for cls, path := range idx.definedClasses {
		items = append(items, item{
			id:       path,
			priority: idx.priority(cls),
			sortKey:  strings.ToLower(cls),
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
