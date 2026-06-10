package rust

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codebaseIndex tracks all discovered Rust entities for cross-reference
// resolution and entity list construction.
type codebaseIndex struct {
	// definedTypes maps type name -> file path (structs, enums, traits)
	definedTypes map[string]string
	// definedFunctions maps function name -> file path
	definedFunctions map[string]string
	// refCounts tracks how many files reference each name
	refCounts map[string]int
	// implBlocks stores all discovered impl blocks for method association
	implBlocks []implBlockDef
	// reExports stores all pub use re-exports
	reExports []reExport
	// fileAnalyses caches the analysis for each file
	fileAnalyses map[string]*fileAnalysis
}

// emptyIndex returns a new, fully-initialised (but empty) codebaseIndex.
func emptyIndex() *codebaseIndex {
	return &codebaseIndex{
		definedTypes:     map[string]string{},
		definedFunctions: map[string]string{},
		refCounts:        map[string]int{},
		fileAnalyses:     map[string]*fileAnalysis{},
	}
}

// FileForType returns the file path for a named type (struct/enum/trait).
func (idx *codebaseIndex) FileForType(name string) (string, bool) {
	v, ok := idx.definedTypes[name]
	return v, ok
}

// FileForFunction returns the file path for a named function.
func (idx *codebaseIndex) FileForFunction(name string) (string, bool) {
	v, ok := idx.definedFunctions[name]
	return v, ok
}

// buildCodebaseIndex walks the Rust source directory and builds the index.
func buildCodebaseIndex(srcRoot string) (*codebaseIndex, error) {
	idx := emptyIndex()

	skipDirs := map[string]bool{
		".git":         true,
		"target":       true,
		"node_modules": true,
		"examples":     true,
		"benches":      true,
		"tests":        true,
	}

	info, err := os.Stat(srcRoot)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("WARNING: Rust source root %q does not exist", srcRoot)
			return idx, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		log.Printf("WARNING: Rust source root %q is not a directory", srcRoot)
		return idx, nil
	}

	walkErr := filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("WARNING: walking %s: %v", path, err)
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if skipDirs[name] {
				return filepath.SkipDir
			}
			// Skip hidden directories
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .rs files
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".rs") {
			return nil
		}

		// Skip test files
		base := strings.TrimSuffix(d.Name(), ".rs")
		if strings.HasSuffix(base, "_test") || strings.HasSuffix(base, "_tests") {
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

		// Register all discovered entities
		for _, st := range analysis.Structs {
			idx.definedTypes[st.Name] = path
		}
		for _, en := range analysis.Enums {
			idx.definedTypes[en.Name] = path
		}
		for _, tr := range analysis.Traits {
			idx.definedTypes[tr.Name] = path
		}
		for _, fn := range analysis.Functions {
			idx.definedFunctions[fn.Name] = path
		}
		for _, ta := range analysis.TypeAliases {
			idx.definedTypes[ta.Name] = path
		}
		for _, m := range analysis.Macros {
			idx.definedFunctions[m.Name] = path
		}
		for _, c := range analysis.Constants {
			idx.definedFunctions[c.Name] = path
		}

		// Store impl blocks with their file path for method resolution
		for i := range analysis.ImplBlocks {
			idx.implBlocks = append(idx.implBlocks, analysis.ImplBlocks[i])
		}

		// Store re-exports
		idx.reExports = append(idx.reExports, analysis.ReExports...)

		idx.fileAnalyses[path] = analysis
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	// Follow pub use re-exports: if a re-export points to an entity we know,
	// register the alias as well
	for _, re := range idx.reExports {
		// Extract the final segment of the path as the entity name
		parts := strings.Split(re.Path, "::")
		if len(parts) == 0 {
			continue
		}
		originalName := parts[len(parts)-1]
		aliasName := re.Alias
		if aliasName == "" {
			aliasName = originalName
		}

		// If we already have the original, register the alias pointing to same file
		if path, ok := idx.definedTypes[originalName]; ok {
			if _, exists := idx.definedTypes[aliasName]; !exists {
				idx.definedTypes[aliasName] = path
			}
		}
		if path, ok := idx.definedFunctions[originalName]; ok {
			if _, exists := idx.definedFunctions[aliasName]; !exists {
				idx.definedFunctions[aliasName] = path
			}
		}
	}

	log.Printf("Indexed %d types, %d functions/macros/constants, %d impl blocks from %s",
		len(idx.definedTypes), len(idx.definedFunctions), len(idx.implBlocks), srcRoot)

	return idx, nil
}

// priority returns a priority level for sorting entities (0 = highest).
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

// buildEntityList returns a sorted list of entity identifiers in the format
// "filepath#EntityName".
func (idx *codebaseIndex) buildEntityList() []string {
	type item struct {
		id       string
		priority int
		sortKey  string
	}

	var items []item

	// Types (structs, enums, traits, type aliases)
	for name, path := range idx.definedTypes {
		items = append(items, item{
			id:       path + "#" + name,
			priority: idx.priority(name),
			sortKey:  strings.ToLower(name),
		})
	}

	// Functions, macros, constants
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

	// Deduplicate
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
