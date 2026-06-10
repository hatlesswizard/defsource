package golang

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codebaseIndex holds discovered entities and their locations for wrapper
// resolution. It stores only name->path mappings and call graph edges,
// not full AST trees or file content (per HIGH-07 memory contract).
type codebaseIndex struct {
	definedTypes     map[string]string // typeName -> filePath
	definedFunctions map[string]string // funcName -> filePath
	refCounts        map[string]int    // entityName -> reference count
	stdlibFunctions  map[string]bool   // known stdlib functions (for wrapper exclusion)
}

// emptyIndex returns a new, fully-initialised (but empty) codebaseIndex.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		definedTypes:     map[string]string{},
		definedFunctions: map[string]string{},
		refCounts:        map[string]int{},
		stdlibFunctions:  map[string]bool{},
	}
}

// FileForType returns the file path for a named type and whether it was found.
func (idx *codebaseIndex) FileForType(name string) (string, bool) {
	v, ok := idx.definedTypes[name]
	return v, ok
}

// FileForFunction returns the file path for a named function and whether it
// was found.
func (idx *codebaseIndex) FileForFunction(name string) (string, bool) {
	v, ok := idx.definedFunctions[name]
	return v, ok
}

// IsStdlib reports whether name is a known Go stdlib function.
func (idx *codebaseIndex) IsStdlib(name string) bool {
	return idx.stdlibFunctions[name]
}

// RefCount returns the number of files that reference name.
func (idx *codebaseIndex) RefCount(name string) int {
	return idx.refCounts[name]
}

// Default directories to skip during discovery.
var defaultExcludeDirs = map[string]bool{
	".git":         true,
	"vendor":       true,
	"node_modules": true,
	"testdata":     true,
	"_test":        true,
}

// buildCodebaseIndex walks the repository and indexes all exported Go entities.
func buildCodebaseIndex(repoPath string, cfg Config) (*codebaseIndex, error) {
	idx := emptyIndex()

	// If ExcludeDirs is explicitly set (even if empty), use only those.
	// Otherwise merge with defaults.
	var excludeDirs map[string]bool
	if cfg.ExcludeDirs != nil {
		excludeDirs = make(map[string]bool, len(cfg.ExcludeDirs))
		for _, d := range cfg.ExcludeDirs {
			excludeDirs[d] = true
		}
	} else {
		excludeDirs = make(map[string]bool, len(defaultExcludeDirs))
		for k, v := range defaultExcludeDirs {
			excludeDirs[k] = v
		}
	}

	roots := cfg.RootDirs
	if len(roots) == 0 {
		roots = []string{""}
	}

	for _, root := range roots {
		rootPath := filepath.Join(repoPath, root)
		info, err := os.Stat(rootPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("WARNING: discovery root %q does not exist, skipping", rootPath)
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			log.Printf("WARNING: discovery root %q is not a directory, skipping", rootPath)
			continue
		}

		walkErr := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("WARNING: walking %s: %v", path, err)
				return nil
			}

			if d.IsDir() {
				name := d.Name()
				if excludeDirs[name] {
					return filepath.SkipDir
				}
				// Skip internal packages
				if name == "internal" {
					return filepath.SkipDir
				}
				return nil
			}

			// Only process .go files, skip test files
			if !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			if strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				log.Printf("WARNING: read %s: %v", path, err)
				return nil
			}

			// Skip generated files
			if isGeneratedFile(content) {
				return nil
			}

			analysis := parseFile(content)
			if analysis == nil {
				return nil
			}

			// Index exported structs
			for _, st := range analysis.Structs {
				if isExported(st.Name) {
					idx.definedTypes[st.Name] = path
				}
			}

			// Index exported interfaces
			for _, iface := range analysis.Interfaces {
				if isExported(iface.Name) {
					idx.definedTypes[iface.Name] = path
				}
			}

			// Index exported functions
			for _, fn := range analysis.Functions {
				if isExported(fn.Name) {
					idx.definedFunctions[fn.Name] = path
				}
			}

			// Index exported type aliases
			for _, ta := range analysis.TypeAliases {
				if isExported(ta.Name) {
					idx.definedTypes[ta.Name] = path
				}
			}

			// Index exported constants
			for _, c := range analysis.Constants {
				if isExported(c.Name) {
					idx.definedFunctions[c.Name] = path
				}
			}

			// Collect call references for priority sorting
			for _, call := range analysis.Calls {
				idx.refCounts[call]++
			}

			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %s: %w", rootPath, walkErr)
		}
	}

	log.Printf("Indexed %d types, %d functions from %s",
		len(idx.definedTypes), len(idx.definedFunctions), repoPath)

	return idx, nil
}

// isGeneratedFile checks if a file contains a "DO NOT EDIT" comment indicating
// it is generated code.
func isGeneratedFile(content []byte) bool {
	// Only check the first 2KB for the generated marker
	check := content
	if len(check) > 2048 {
		check = check[:2048]
	}
	s := string(check)
	return strings.Contains(s, "DO NOT EDIT") ||
		strings.Contains(s, "Code generated") ||
		strings.Contains(s, "generated by")
}

// priority returns a sort priority for an entity based on reference count.
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

// buildEntityList returns a sorted list of entity identifiers for discovery.
func (idx *codebaseIndex) buildEntityList() []string {
	type item struct {
		id       string
		priority int
		sortKey  string
	}

	var items []item

	// Add types (structs, interfaces, type aliases)
	for name, path := range idx.definedTypes {
		items = append(items, item{
			id:       path + "#" + name,
			priority: idx.priority(name),
			sortKey:  strings.ToLower(name),
		})
	}

	// Add functions and constants
	for name, path := range idx.definedFunctions {
		items = append(items, item{
			id:       path + "#" + name,
			priority: idx.priority(name),
			sortKey:  strings.ToLower(name),
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
