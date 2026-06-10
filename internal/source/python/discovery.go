package python

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codebaseIndex holds all discovered entities and their locations for
// cross-file wrapper resolution and entity prioritisation.
type codebaseIndex struct {
	definedClasses   map[string]string // className -> filePath
	definedFunctions map[string]string // funcName -> filePath
	refCounts        map[string]int    // name -> number of files referencing it
	allExports       map[string]map[string]bool // filePath -> set of __all__ names
	fileAnalyses     map[string]*fileAnalysis
	sourceRoots      []string
}

// emptyIndex returns a new, fully-initialised (but empty) codebaseIndex.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		definedClasses:   map[string]string{},
		definedFunctions: map[string]string{},
		refCounts:        map[string]int{},
		allExports:       map[string]map[string]bool{},
		fileAnalyses:     map[string]*fileAnalysis{},
	}
}

// FileForClass returns the file path for an exactly-named class.
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

// FileForFunction returns the file path for a named top-level function.
func (idx *codebaseIndex) FileForFunction(name string) (string, bool) {
	v, ok := idx.definedFunctions[name]
	return v, ok
}

// HasFunction reports whether the index contains a top-level function.
func (idx *codebaseIndex) HasFunction(name string) bool {
	_, ok := idx.definedFunctions[name]
	return ok
}

// HasClass reports whether the index contains a class with the given name.
func (idx *codebaseIndex) HasClass(name string) bool {
	_, ok := idx.definedClasses[name]
	return ok
}

// IsPublic checks whether an entity name is public for its file.
// If the file has __all__, only names in __all__ are public.
// Otherwise, names not starting with underscore are public.
func (idx *codebaseIndex) IsPublic(filePath, name string) bool {
	if allNames, hasAll := idx.allExports[filePath]; hasAll {
		return allNames[name]
	}
	return !strings.HasPrefix(name, "_")
}

// skipDirs contains directory names to skip during filesystem walk.
var skipDirs = map[string]bool{
	".git":         true,
	"__pycache__":  true,
	".tox":         true,
	".mypy_cache":  true,
	".pytest_cache": true,
	"node_modules": true,
	"vendor":       true,
	".eggs":        true,
	"*.egg-info":   true,
	"venv":         true,
	".venv":        true,
	"env":          true,
}

// skipFilePatterns contains filename patterns to skip.
func shouldSkipFile(name string) bool {
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".py") {
		return true
	}
	// Skip test files
	if strings.HasPrefix(lower, "test_") || strings.HasSuffix(lower, "_test.py") {
		return true
	}
	if lower == "conftest.py" || lower == "setup.py" {
		return true
	}
	// Skip migration files (Django pattern)
	if len(lower) > 4 && lower[0] >= '0' && lower[0] <= '9' {
		// Likely a migration file like "0001_initial.py"
		parts := strings.SplitN(lower, "_", 2)
		if len(parts) == 2 {
			allDigits := true
			for _, c := range parts[0] {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return true
			}
		}
	}
	return false
}

// shouldSkipDir checks whether a directory should be skipped.
func shouldSkipDir(name string) bool {
	if skipDirs[name] {
		return true
	}
	// Skip directories that match egg-info pattern
	if strings.HasSuffix(name, ".egg-info") {
		return true
	}
	// Skip test directories
	lower := strings.ToLower(name)
	if lower == "tests" || lower == "test" {
		return true
	}
	// Skip migration directories
	if lower == "migrations" {
		return true
	}
	return false
}

// buildCodebaseIndex walks the source roots and indexes all Python entities.
func buildCodebaseIndex(repoPath string, roots []string) (*codebaseIndex, error) {
	idx := emptyIndex()
	idx.sourceRoots = roots

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
				if shouldSkipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}

			if shouldSkipFile(d.Name()) {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				log.Printf("WARNING: read %s: %v", path, err)
				return nil
			}

			// Parse __all__ for the file
			allNames := parseAllExports(content)
			if allNames != nil {
				idx.allExports[path] = allNames
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

	// Build reference counts from call references
	for _, analysis := range idx.fileAnalyses {
		seen := map[string]bool{}
		for _, call := range analysis.Calls {
			if !seen[call.Name] {
				idx.refCounts[call.Name]++
				seen[call.Name] = true
			}
		}
	}

	log.Printf("Indexed %d classes, %d functions from Python source in %v",
		len(idx.definedClasses), len(idx.definedFunctions), roots)

	return idx, nil
}

// parseAllExports extracts names from __all__ = [...] at the module level.
// Returns nil if no __all__ is found, or the set of exported names.
func parseAllExports(content []byte) map[string]bool {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var inAll bool
	var allContent strings.Builder
	bracketDepth := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !inAll {
			// Look for __all__ = [
			if strings.HasPrefix(trimmed, "__all__") && strings.Contains(trimmed, "=") {
				inAll = true
				afterEq := trimmed[strings.Index(trimmed, "=")+1:]
				afterEq = strings.TrimSpace(afterEq)
				allContent.WriteString(afterEq)
				bracketDepth += strings.Count(afterEq, "[") + strings.Count(afterEq, "(")
				bracketDepth -= strings.Count(afterEq, "]") + strings.Count(afterEq, ")")
				if bracketDepth <= 0 {
					break
				}
			}
			continue
		}

		allContent.WriteString(trimmed)
		bracketDepth += strings.Count(trimmed, "[") + strings.Count(trimmed, "(")
		bracketDepth -= strings.Count(trimmed, "]") + strings.Count(trimmed, ")")
		if bracketDepth <= 0 {
			break
		}
	}

	if !inAll {
		return nil
	}

	// Extract quoted strings from the collected content
	raw := allContent.String()
	names := make(map[string]bool)
	inStr := false
	var quote byte
	var current strings.Builder

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if !inStr {
			if ch == '"' || ch == '\'' {
				inStr = true
				quote = ch
				current.Reset()
			}
		} else {
			if ch == quote {
				name := current.String()
				if name != "" {
					names[name] = true
				}
				inStr = false
			} else {
				current.WriteByte(ch)
			}
		}
	}

	if len(names) == 0 {
		return nil
	}
	return names
}

// priority returns a sort priority based on reference count.
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

// buildEntityList returns a sorted list of entity IDs to crawl.
func (idx *codebaseIndex) buildEntityList() []string {
	type item struct {
		id       string
		priority int
		sortKey  string
	}

	var items []item
	for cls, path := range idx.definedClasses {
		if !idx.IsPublic(path, cls) {
			continue
		}
		items = append(items, item{
			id:       path + "#" + cls,
			priority: idx.priority(cls),
			sortKey:  strings.ToLower(cls),
		})
	}
	for fn, path := range idx.definedFunctions {
		if !idx.IsPublic(path, fn) {
			continue
		}
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
