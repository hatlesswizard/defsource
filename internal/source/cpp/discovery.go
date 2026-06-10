package cpp

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codebaseIndex holds the indexed entities from a C++ codebase.
type codebaseIndex struct {
	definedClasses   map[string]string // qualifiedName -> filePath
	definedFunctions map[string]string // qualifiedName -> filePath
	definedStructs   map[string]string // qualifiedName -> filePath
	definedEnums     map[string]string // qualifiedName -> filePath
	definedAliases   map[string]string // qualifiedName -> filePath
	definedConcepts  map[string]string // qualifiedName -> filePath
	refCounts        map[string]int
	fileAnalyses     map[string]*fileAnalysis
}

// emptyIndex returns a fully-initialized but empty codebaseIndex.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		definedClasses:   map[string]string{},
		definedFunctions: map[string]string{},
		definedStructs:   map[string]string{},
		definedEnums:     map[string]string{},
		definedAliases:   map[string]string{},
		definedConcepts:  map[string]string{},
		refCounts:        map[string]int{},
		fileAnalyses:     map[string]*fileAnalysis{},
	}
}

// FileForClass returns the file path for an exactly-named class.
func (idx *codebaseIndex) FileForClass(name string) (string, bool) {
	v, ok := idx.definedClasses[name]
	if ok {
		return v, true
	}
	// Also check structs
	v, ok = idx.definedStructs[name]
	return v, ok
}

// FileForFunction returns the file path for a named function.
func (idx *codebaseIndex) FileForFunction(name string) (string, bool) {
	v, ok := idx.definedFunctions[name]
	return v, ok
}

// defaultSkipDirs are always excluded from discovery.
var defaultSkipDirs = map[string]bool{
	".git":         true,
	"vendor":       true,
	"node_modules": true,
	"build":        true,
	"cmake-build":  true,
	"_build":       true,
	"__pycache__":  true,
	// Internal/implementation directories
	"internal": true,
	"detail":   true,
	"impl":     true,
	"private":  true,
}

// cppExtensions contains valid C++ header file extensions.
var cppExtensions = map[string]bool{
	".h":   true,
	".hpp": true,
	".hxx": true,
	".h++": true,
	".hh":  true,
}

// buildCodebaseIndex walks the source directories and indexes all C++ entities.
func buildCodebaseIndex(repoPath string, includeDirs, skipDirs []string) (*codebaseIndex, error) {
	idx := emptyIndex()

	// Build set of extra skip dirs
	extraSkip := make(map[string]bool, len(skipDirs))
	for _, d := range skipDirs {
		extraSkip[strings.ToLower(d)] = true
	}

	// Determine root directories to walk
	roots := resolveRoots(repoPath, includeDirs)
	if len(roots) == 0 {
		// Fallback: walk the entire repo
		roots = []string{repoPath}
	}

	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("WARNING: C++ discovery root %q does not exist, skipping", root)
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			log.Printf("WARNING: C++ discovery root %q is not a directory, skipping", root)
			continue
		}

		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("WARNING: walking %s: %v", path, err)
				return nil
			}

			if d.IsDir() {
				dirName := strings.ToLower(d.Name())
				if defaultSkipDirs[dirName] || extraSkip[dirName] {
					return filepath.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(d.Name()))
			if !cppExtensions[ext] {
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

			// Normalize path to forward slashes for consistency
			normPath := filepath.ToSlash(path)

			for _, cls := range analysis.Classes {
				idx.definedClasses[cls.QualifiedName] = normPath
			}
			for _, st := range analysis.Structs {
				idx.definedStructs[st.QualifiedName] = normPath
			}
			for _, fn := range analysis.Functions {
				idx.definedFunctions[fn.QualifiedName] = normPath
			}
			for _, en := range analysis.Enums {
				idx.definedEnums[en.QualifiedName] = normPath
			}
			for _, ta := range analysis.TypeAliases {
				idx.definedAliases[ta.QualifiedName] = normPath
			}
			for _, c := range analysis.Concepts {
				idx.definedConcepts[c.QualifiedName] = normPath
			}

			idx.fileAnalyses[normPath] = analysis
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}

	// Count references
	for _, analysis := range idx.fileAnalyses {
		seen := map[string]bool{}
		for _, call := range analysis.Calls {
			if !seen[call.Name] {
				idx.refCounts[call.Name]++
				seen[call.Name] = true
			}
		}
	}

	log.Printf("C++ index: %d classes, %d structs, %d functions, %d enums, %d aliases, %d concepts",
		len(idx.definedClasses), len(idx.definedStructs), len(idx.definedFunctions),
		len(idx.definedEnums), len(idx.definedAliases), len(idx.definedConcepts))

	return idx, nil
}

// resolveRoots determines which directories to walk for public headers.
func resolveRoots(repoPath string, includeDirs []string) []string {
	var roots []string
	for _, dir := range includeDirs {
		root := filepath.Join(repoPath, dir)
		if _, err := os.Stat(root); err == nil {
			roots = append(roots, root)
		}
	}
	// Also check common patterns if none of the specified dirs exist
	if len(roots) == 0 {
		commonDirs := []string{"include", "src", "lib"}
		for _, dir := range commonDirs {
			root := filepath.Join(repoPath, dir)
			if _, err := os.Stat(root); err == nil {
				roots = append(roots, root)
			}
		}
	}
	return roots
}

// priority returns sorting priority based on reference count.
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

// buildEntityList returns a sorted list of entity identifiers.
func (idx *codebaseIndex) buildEntityList() []string {
	type item struct {
		id       string
		priority int
		sortKey  string
	}

	var items []item

	for qn, path := range idx.definedClasses {
		items = append(items, item{
			id:       path + "#" + qn,
			priority: idx.priority(qn),
			sortKey:  strings.ToLower(qn),
		})
	}
	for qn, path := range idx.definedStructs {
		items = append(items, item{
			id:       path + "#" + qn,
			priority: idx.priority(qn),
			sortKey:  strings.ToLower(qn),
		})
	}
	for qn, path := range idx.definedFunctions {
		items = append(items, item{
			id:       path + "#" + qn,
			priority: idx.priority(qn),
			sortKey:  strings.ToLower(qn),
		})
	}
	for qn, path := range idx.definedEnums {
		items = append(items, item{
			id:       path + "#" + qn,
			priority: idx.priority(qn),
			sortKey:  strings.ToLower(qn),
		})
	}
	for qn, path := range idx.definedAliases {
		items = append(items, item{
			id:       path + "#" + qn,
			priority: idx.priority(qn),
			sortKey:  strings.ToLower(qn),
		})
	}
	for qn, path := range idx.definedConcepts {
		items = append(items, item{
			id:       path + "#" + qn,
			priority: idx.priority(qn),
			sortKey:  strings.ToLower(qn),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].priority != items[j].priority {
			return items[i].priority < items[j].priority
		}
		return items[i].sortKey < items[j].sortKey
	})

	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.id
	}
	return ids
}
