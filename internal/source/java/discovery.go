package java

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codebaseIndex stores the results of a full source tree scan.
type codebaseIndex struct {
	// definedClasses maps qualified class name -> absolute file path.
	definedClasses map[string]string
	// refCounts tracks how many files reference each class.
	refCounts map[string]int
	// fileAnalyses caches parsed file contents during discovery.
	fileAnalyses map[string]*fileAnalysis
}

// emptyIndex returns a new, fully-initialised but empty codebaseIndex.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		definedClasses: map[string]string{},
		refCounts:      map[string]int{},
		fileAnalyses:   map[string]*fileAnalysis{},
	}
}

// FileForClass returns the file path for a named class and whether it was found.
func (idx *codebaseIndex) FileForClass(name string) (string, bool) {
	v, ok := idx.definedClasses[name]
	return v, ok
}

// HasClass reports whether the index contains a class with the given name.
func (idx *codebaseIndex) HasClass(name string) bool {
	_, ok := idx.definedClasses[name]
	return ok
}

// skipDirs is the set of directory names to skip during discovery.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	"build":        true,
	".gradle":      true,
	".mvn":         true,
	".idea":        true,
}

// skipPathSegments are path segments indicating test or non-public code.
// These are checked relative to the source root, NOT the absolute path.
var skipPathSegments = []string{
	"/src/test/",
	"/src/testFixtures/",
	"/src/it/",
	"/testkit/",
	"/benchmark/",
	"/jmh/",
}

// buildCodebaseIndex walks the configured source roots and builds a full index.
func buildCodebaseIndex(repoPath string, sourceRoots []string) (*codebaseIndex, error) {
	idx := emptyIndex()

	for _, root := range sourceRoots {
		absRoot := filepath.Join(repoPath, root)
		info, err := os.Stat(absRoot)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("WARNING: Java source root %q does not exist, skipping", absRoot)
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			log.Printf("WARNING: Java source root %q is not a directory, skipping", absRoot)
			continue
		}

		walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("WARNING: walking %s: %v", path, err)
				return nil
			}

			if d.IsDir() {
				name := d.Name()
				if skipDirs[name] {
					return filepath.SkipDir
				}
				return nil
			}

			if !isJavaSourceFile(d.Name()) {
				return nil
			}

			// Skip test paths (use path relative to repo root for pattern matching).
			relFromRepo, _ := filepath.Rel(repoPath, path)
			relPath := filepath.ToSlash(relFromRepo)
			if shouldSkipPath(relPath) {
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

			// Register all public types.
			for _, t := range analysis.Types {
				if t.Visibility == "public" || t.Visibility == "" {
					idx.definedClasses[t.Name] = path
					// Register public inner types.
					registerInnerTypes(idx, path, t.Name, t.InnerTypes)
				}
			}
			idx.fileAnalyses[path] = analysis
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %s: %w", absRoot, walkErr)
		}
	}

	// Count references.
	for _, analysis := range idx.fileAnalyses {
		seen := map[string]bool{}
		for _, call := range analysis.Calls {
			if !seen[call.Name] {
				idx.refCounts[call.Name]++
				seen[call.Name] = true
			}
		}
	}

	log.Printf("Indexed %d Java types from %s", len(idx.definedClasses), repoPath)
	return idx, nil
}

// registerInnerTypes recursively registers public inner/nested types.
func registerInnerTypes(idx *codebaseIndex, path, prefix string, innerTypes []javaType) {
	for _, inner := range innerTypes {
		if inner.Visibility == "public" || inner.Visibility == "" {
			qualifiedName := prefix + "." + inner.Name
			idx.definedClasses[qualifiedName] = path
			registerInnerTypes(idx, path, qualifiedName, inner.InnerTypes)
		}
	}
}

// isJavaSourceFile checks if the filename is a Java source file.
func isJavaSourceFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".java")
}

// shouldSkipPath checks if a relative path (from repo root) should be excluded.
func shouldSkipPath(path string) bool {
	// Normalize to forward slash and ensure leading slash for matching.
	normalized := "/" + strings.TrimPrefix(path, "/")
	for _, seg := range skipPathSegments {
		if strings.Contains(normalized, seg) {
			return true
		}
	}
	return false
}

// buildEntityList produces the sorted list of entity identifiers.
func (idx *codebaseIndex) buildEntityList() []string {
	type item struct {
		id       string
		priority int
		sortKey  string
	}

	var items []item
	for name, path := range idx.definedClasses {
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

// priority assigns a crawl priority based on reference counts.
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
