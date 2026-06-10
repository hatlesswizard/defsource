package typescript

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codebaseIndex holds the discovered entities and their locations for
// cross-referencing during wrapper detection and entity resolution.
type codebaseIndex struct {
	// definedClasses maps class/interface name -> file path
	definedClasses map[string]string

	// definedFunctions maps function name -> file path
	definedFunctions map[string]string

	// definedTypes maps type alias/enum name -> file path
	definedTypes map[string]string

	// definedNamespaces maps namespace name -> file path
	definedNamespaces map[string]string

	// refCounts maps entity name -> number of files referencing it
	refCounts map[string]int

	// reExports maps exported name -> original module path
	reExports map[string]string

	// fileAnalyses caches parsed file results
	fileAnalyses map[string]*fileAnalysis
}

// emptyIndex returns a new, fully-initialised (but empty) codebaseIndex.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		definedClasses:    map[string]string{},
		definedFunctions:  map[string]string{},
		definedTypes:      map[string]string{},
		definedNamespaces: map[string]string{},
		refCounts:         map[string]int{},
		reExports:         map[string]string{},
		fileAnalyses:      map[string]*fileAnalysis{},
	}
}

// FileForClass returns the file path for a class or interface by name.
func (idx *codebaseIndex) FileForClass(name string) (string, bool) {
	v, ok := idx.definedClasses[name]
	return v, ok
}

// FileForFunction returns the file path for a function by name.
func (idx *codebaseIndex) FileForFunction(name string) (string, bool) {
	v, ok := idx.definedFunctions[name]
	return v, ok
}

// FileForType returns the file path for a type alias or enum by name.
func (idx *codebaseIndex) FileForType(name string) (string, bool) {
	v, ok := idx.definedTypes[name]
	return v, ok
}

// HasEntity reports whether the index contains any entity with the given name.
func (idx *codebaseIndex) HasEntity(name string) bool {
	if _, ok := idx.definedClasses[name]; ok {
		return true
	}
	if _, ok := idx.definedFunctions[name]; ok {
		return true
	}
	if _, ok := idx.definedTypes[name]; ok {
		return true
	}
	if _, ok := idx.definedNamespaces[name]; ok {
		return true
	}
	return false
}

// defaultExcludeDirs are always skipped during discovery.
var defaultExcludeDirs = map[string]bool{
	"node_modules": true,
	"test":         true,
	"tests":        true,
	"__tests__":    true,
	"__mocks__":    true,
	"dist":         true,
	"build":        true,
	".git":         true,
	"coverage":     true,
	"fixtures":     true,
	"e2e":          true,
}

// isTestFile returns true if the filename matches a test pattern.
func isTestFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".test.ts") || strings.HasSuffix(lower, ".test.tsx") {
		return true
	}
	if strings.HasSuffix(lower, ".spec.ts") || strings.HasSuffix(lower, ".spec.tsx") {
		return true
	}
	if strings.HasSuffix(lower, "_test.ts") || strings.HasSuffix(lower, "_test.tsx") {
		return true
	}
	return false
}

// isTypeScriptFile returns true if the filename is a TypeScript source file.
func isTypeScriptFile(name string, includeDecl bool) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".d.ts") {
		return includeDecl
	}
	return strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx")
}

// buildCodebaseIndex walks the configured source directories and indexes all
// exported TypeScript entities.
func buildCodebaseIndex(repoPath string, cfg Config) (*codebaseIndex, error) {
	idx := emptyIndex()

	excludeDirs := make(map[string]bool, len(defaultExcludeDirs)+len(cfg.ExcludeDirs))
	for k, v := range defaultExcludeDirs {
		excludeDirs[k] = v
	}
	for _, d := range cfg.ExcludeDirs {
		excludeDirs[d] = true
	}

	roots := make([]string, 0, len(cfg.SourceDirs))
	for _, dir := range cfg.SourceDirs {
		roots = append(roots, filepath.Join(repoPath, filepath.FromSlash(dir)))
	}

	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("WARNING: TypeScript discovery root %q does not exist, skipping", root)
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			log.Printf("WARNING: TypeScript discovery root %q is not a directory, skipping", root)
			continue
		}

		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("WARNING: walking %s: %v", path, err)
				return nil
			}

			if d.IsDir() {
				if excludeDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}

			name := d.Name()
			if !isTypeScriptFile(name, cfg.IncludeDeclarationFiles) {
				return nil
			}
			if isTestFile(name) {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				log.Printf("WARNING: read %s: %v", path, err)
				return nil
			}

			analysis := parseFile(content, path)
			if analysis == nil {
				return nil
			}

			// Index exported entities. Declaration files (.d.ts) describe
			// ambient/global APIs whose top-level declarations carry no
			// `export` keyword (e.g. lib.dom.d.ts), so everything in them
			// is considered public.
			isDecl := strings.HasSuffix(strings.ToLower(name), ".d.ts")
			for _, cls := range analysis.Classes {
				if cls.Exported || isDecl {
					idx.definedClasses[cls.Name] = path
				}
			}
			for _, iface := range analysis.Interfaces {
				if iface.Exported || isDecl {
					idx.definedClasses[iface.Name] = path
				}
			}
			for _, fn := range analysis.Functions {
				if fn.Exported || isDecl {
					idx.definedFunctions[fn.Name] = path
				}
			}
			for _, ta := range analysis.TypeAliases {
				if ta.Exported || isDecl {
					idx.definedTypes[ta.Name] = path
				}
			}
			for _, en := range analysis.Enums {
				if en.Exported || isDecl {
					idx.definedTypes[en.Name] = path
				}
			}
			for _, ns := range analysis.Namespaces {
				if ns.Exported || isDecl {
					idx.definedNamespaces[ns.Name] = path
				}
			}
			for _, re := range analysis.ReExports {
				idx.reExports[re.Name] = re.ModulePath
			}

			idx.fileAnalyses[path] = analysis
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %s: %w", root, walkErr)
		}
	}

	// Build reference counts from imports/calls
	for _, analysis := range idx.fileAnalyses {
		seen := map[string]bool{}
		for _, ref := range analysis.References {
			if !seen[ref] {
				idx.refCounts[ref]++
				seen[ref] = true
			}
		}
	}

	log.Printf("TypeScript indexed %d classes/interfaces, %d functions, %d types, %d namespaces from %s",
		len(idx.definedClasses), len(idx.definedFunctions), len(idx.definedTypes), len(idx.definedNamespaces), repoPath)

	return idx, nil
}

// priority returns a priority bucket for an entity based on reference count.
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

// buildEntityList returns a sorted, deduplicated list of entity identifiers.
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
	for name, path := range idx.definedFunctions {
		items = append(items, item{
			id:       path + "#" + name,
			priority: idx.priority(name),
			sortKey:  strings.ToLower(name),
		})
	}
	for name, path := range idx.definedTypes {
		items = append(items, item{
			id:       path + "#" + name,
			priority: idx.priority(name),
			sortKey:  strings.ToLower(name),
		})
	}
	for name, path := range idx.definedNamespaces {
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
