package javascript

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codebaseIndex holds the results of scanning all JS source files in a
// repository, mapping entity names to their file paths and tracking
// reference counts for priority-based discovery ordering.
type codebaseIndex struct {
	definedClasses   map[string]string // className -> filePath
	definedFunctions map[string]string // funcName -> filePath
	definedModules   map[string]string // moduleName -> filePath
	definedConstants map[string]string // constName -> filePath
	refCounts        map[string]int
	builtinFunctions map[string]bool
	fileAnalyses     map[string]*fileAnalysis
}

// emptyIndex returns a new, fully-initialised (but empty) codebaseIndex.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		definedClasses:   map[string]string{},
		definedFunctions: map[string]string{},
		definedModules:   map[string]string{},
		definedConstants: map[string]string{},
		refCounts:        map[string]int{},
		builtinFunctions: map[string]bool{},
		fileAnalyses:     map[string]*fileAnalysis{},
	}
}

// FileForClass returns the file path for a named class.
func (idx *codebaseIndex) FileForClass(name string) (string, bool) {
	v, ok := idx.definedClasses[name]
	return v, ok
}

// FileForFunction returns the file path for a named function.
func (idx *codebaseIndex) FileForFunction(name string) (string, bool) {
	v, ok := idx.definedFunctions[name]
	return v, ok
}

// FileForModule returns the file path for a named module export.
func (idx *codebaseIndex) FileForModule(name string) (string, bool) {
	v, ok := idx.definedModules[name]
	return v, ok
}

// skipDirs contains directory names to skip during discovery.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"test":         true,
	"tests":        true,
	"__tests__":    true,
	"__mocks__":    true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	".next":        true,
	".nuxt":        true,
}

// isJSFile returns true if the filename has a JavaScript extension.
func isJSFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".js") ||
		strings.HasSuffix(lower, ".mjs") ||
		strings.HasSuffix(lower, ".cjs")
}

// isTestFile returns true if the filename matches test file patterns.
func isTestFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".test.js") ||
		strings.HasSuffix(lower, ".spec.js") ||
		strings.HasSuffix(lower, ".test.mjs") ||
		strings.HasSuffix(lower, ".spec.mjs") ||
		strings.HasSuffix(lower, ".test.cjs") ||
		strings.HasSuffix(lower, ".spec.cjs") ||
		strings.Contains(lower, "__test__")
}

// buildCodebaseIndex walks the given root directories and builds an index
// of all exported JavaScript entities.
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

			if !isJSFile(d.Name()) || isTestFile(d.Name()) {
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

			// Only index exported entities.
			for _, cls := range analysis.Classes {
				if cls.Exported {
					idx.definedClasses[cls.Name] = path
				}
			}
			for _, fn := range analysis.Functions {
				if fn.Exported {
					idx.definedFunctions[fn.Name] = path
				}
			}
			for _, mod := range analysis.Modules {
				if mod.Exported {
					idx.definedModules[mod.Name] = path
				}
			}
			for _, c := range analysis.Constants {
				if c.Exported {
					idx.definedConstants[c.Name] = path
				}
			}

			idx.fileAnalyses[path] = analysis
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %s: %w", root, walkErr)
		}
	}

	// Build ref counts and detect builtins.
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

	log.Printf("Indexed %d classes, %d functions, %d modules, %d constants, detected %d builtins from %s",
		len(idx.definedClasses), len(idx.definedFunctions),
		len(idx.definedModules), len(idx.definedConstants),
		len(idx.builtinFunctions), repoPath)

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

// buildEntityList returns a sorted list of entity IDs, ordered by priority
// (most-referenced first) then alphabetically.
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
	for fn, path := range idx.definedFunctions {
		items = append(items, item{
			id:       path + "#" + fn,
			priority: idx.priority(fn),
			sortKey:  strings.ToLower(fn),
		})
	}
	for mod, path := range idx.definedModules {
		items = append(items, item{
			id:       path + "#" + mod,
			priority: idx.priority(mod),
			sortKey:  strings.ToLower(mod),
		})
	}
	for c, path := range idx.definedConstants {
		items = append(items, item{
			id:       path + "#" + c,
			priority: idx.priority(c),
			sortKey:  strings.ToLower(c),
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
