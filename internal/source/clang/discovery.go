package clang

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codebaseIndex holds the discovered entities from header files.
type codebaseIndex struct {
	// functions maps function name -> file path.
	functions map[string]string
	// structs maps struct name -> file path.
	structs map[string]string
	// enums maps enum name -> file path.
	enums map[string]string
	// unions maps union name -> file path.
	unions map[string]string
	// typedefs maps typedef name -> file path.
	typedefs map[string]string
	// macros maps macro name -> file path.
	macros map[string]string

	// headerDirs restricts which directories to scan.
	headerDirs []string
	// excludePatterns are path substrings to skip.
	excludePatterns []string
}

// emptyIndex returns a new, fully-initialised (but empty) codebaseIndex.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		functions: map[string]string{},
		structs:   map[string]string{},
		enums:     map[string]string{},
		unions:    map[string]string{},
		typedefs:  map[string]string{},
		macros:    map[string]string{},
	}
}

// FileForFunction returns the file path for a named function and whether it was found.
func (idx *codebaseIndex) FileForFunction(name string) (string, bool) {
	v, ok := idx.functions[name]
	return v, ok
}

// FileForMacro returns the file path for a named macro and whether it was found.
func (idx *codebaseIndex) FileForMacro(name string) (string, bool) {
	v, ok := idx.macros[name]
	return v, ok
}

// HasFunction reports whether name is a known function.
func (idx *codebaseIndex) HasFunction(name string) bool {
	_, ok := idx.functions[name]
	return ok
}

// buildEntityList builds a sorted list of entity identifiers from the index.
func (idx *codebaseIndex) buildEntityList() []string {
	// Estimate total capacity.
	total := len(idx.functions) + len(idx.structs) + len(idx.enums) +
		len(idx.unions) + len(idx.typedefs) + len(idx.macros)
	ids := make([]string, 0, total)

	for name, path := range idx.functions {
		ids = append(ids, path+"#"+name)
	}
	for name, path := range idx.structs {
		ids = append(ids, path+"#"+name)
	}
	for name, path := range idx.enums {
		ids = append(ids, path+"#"+name)
	}
	for name, path := range idx.unions {
		ids = append(ids, path+"#"+name)
	}
	for name, path := range idx.typedefs {
		ids = append(ids, path+"#"+name)
	}
	for name, path := range idx.macros {
		ids = append(ids, path+"#"+name)
	}

	sort.Strings(ids)
	return ids
}

// buildCodebaseIndex walks header files and indexes all public entities.
func buildCodebaseIndex(repoPath string, headerDirs, excludePatterns []string) (*codebaseIndex, error) {
	idx := emptyIndex()
	idx.headerDirs = headerDirs
	idx.excludePatterns = excludePatterns

	// Determine root directories to scan.
	roots := headerDirs
	if len(roots) == 0 {
		roots = []string{""} // scan from repo root
	}

	for _, dir := range roots {
		scanRoot := filepath.Join(repoPath, dir)
		info, err := os.Stat(scanRoot)
		if err != nil {
			log.Printf("Warning: header dir %q not found in %s: %v", dir, repoPath, err)
			continue
		}
		if !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip errors
			}
			if d.IsDir() {
				if shouldExcludeDir(path, excludePatterns) {
					return fs.SkipDir
				}
				return nil
			}

			// Only process header files.
			if !isHeaderFile(path) {
				return nil
			}

			// Skip internal/private headers.
			if isInternalHeader(path) {
				return nil
			}

			// Read and index the file.
			content, err := os.ReadFile(path)
			if err != nil {
				return nil // skip unreadable files
			}

			indexHeaderFile(idx, path, content)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	log.Printf("Indexed: %d functions, %d structs, %d enums, %d unions, %d typedefs, %d macros",
		len(idx.functions), len(idx.structs), len(idx.enums),
		len(idx.unions), len(idx.typedefs), len(idx.macros))

	return idx, nil
}

// indexHeaderFile parses a single header file and adds entities to the index.
func indexHeaderFile(idx *codebaseIndex, path string, content []byte) {
	// Parse with tree-sitter for AST entities.
	analysis := parseFileC(content)

	for _, fn := range analysis.Functions {
		if fn.Name != "" && !isReservedName(fn.Name) {
			idx.functions[fn.Name] = path
		}
	}

	for _, st := range analysis.Structs {
		if st.Name != "" {
			idx.structs[st.Name] = path
		}
	}

	for _, en := range analysis.Enums {
		if en.Name != "" {
			idx.enums[en.Name] = path
		}
	}

	for _, un := range analysis.Unions {
		if un.Name != "" {
			idx.unions[un.Name] = path
		}
	}

	for _, td := range analysis.Typedefs {
		if td.Name != "" {
			idx.typedefs[td.Name] = path
		}
	}

	// Extract macros via regex (tree-sitter does not parse preprocessor directives).
	macros := extractMacros(content)
	for _, m := range macros {
		if m.Name != "" && !isReservedName(m.Name) {
			idx.macros[m.Name] = path
		}
	}
}

// isHeaderFile returns true if the path is a C/C++ header file.
func isHeaderFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".h" || ext == ".hh" || ext == ".hpp"
}

// isInternalHeader returns true if the header is likely internal/private.
func isInternalHeader(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(path))

	// Skip files with _internal or _private in name.
	if strings.Contains(base, "_internal") || strings.Contains(base, "_private") {
		return true
	}

	// Skip directories named internal, private, detail, impl.
	internalDirs := []string{"/internal/", "/private/", "/detail/", "/impl/", "/_"}
	for _, d := range internalDirs {
		if strings.Contains(lower, d) {
			return true
		}
	}

	return false
}

// shouldExcludeDir returns true if a directory should be skipped.
func shouldExcludeDir(path string, excludePatterns []string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(path))

	// Always skip hidden directories and common non-source dirs.
	if strings.HasPrefix(base, ".") {
		return true
	}
	skipDirs := []string{"test", "tests", "testing", "examples", "bench", "benchmarks", "doc", "docs", "build", "cmake", ".git"}
	for _, d := range skipDirs {
		if base == d {
			return true
		}
	}

	// Apply user-specified exclude patterns. Patterns name directories, so
	// they are compared against whole path segments — a raw substring match
	// would let a pattern like "t" (h2o's test dir) match the letter t in
	// any ancestor path and skip the entire tree.
	for _, pattern := range excludePatterns {
		p := strings.ToLower(strings.Trim(filepath.ToSlash(pattern), "/"))
		if p == "" {
			continue
		}
		if strings.Contains(p, "/") {
			// Multi-segment patterns still match as path substrings.
			if strings.Contains(lower, p) {
				return true
			}
			continue
		}
		for _, seg := range strings.Split(lower, "/") {
			if seg == p {
				return true
			}
		}
	}

	return false
}

// isReservedName returns true if the name is a common C reserved or
// compiler-internal identifier that should not be indexed as a public API entity.
func isReservedName(name string) bool {
	// Double-underscore prefix is reserved for compiler/implementation.
	if strings.HasPrefix(name, "__") {
		return true
	}
	// Single underscore + uppercase is reserved.
	if len(name) >= 2 && name[0] == '_' && name[1] >= 'A' && name[1] <= 'Z' {
		return true
	}
	return false
}
