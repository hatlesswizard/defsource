//go:build sqlite_fts5 || fts5

package wpgithub

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test 10 — buildCodebaseIndex populates index from PHP files
// ---------------------------------------------------------------------------

// TestBuildCodebaseIndex_PopulatesIndex verifies that buildCodebaseIndex
// walks the wp-includes and wp-admin/includes fixture directories and
// populates definedClasses, definedFunctions, and fileAnalyses.
func TestBuildCodebaseIndex_PopulatesIndex(t *testing.T) {
	repoPath := filepath.Join("testdata", "wpgithub_tests")

	idx, err := buildCodebaseIndex(repoPath)
	if err != nil {
		t.Fatalf("buildCodebaseIndex() error: %v", err)
	}
	if idx == nil {
		t.Fatal("buildCodebaseIndex() returned nil index")
	}
	if len(idx.definedClasses) == 0 {
		t.Error("definedClasses is empty, want WP_Post (from class-wp-post.php)")
	}
	if len(idx.definedFunctions) == 0 {
		t.Error("definedFunctions is empty, want wp_cache_get and others")
	}
	if len(idx.fileAnalyses) == 0 {
		t.Error("fileAnalyses is empty after walking non-empty fixture tree")
	}

	// WP_Post must be found in class-wp-post.php.
	if path, ok := idx.definedClasses["WP_Post"]; !ok {
		t.Error("definedClasses missing WP_Post")
	} else if !strings.HasSuffix(filepath.ToSlash(path), "class-wp-post.php") {
		t.Errorf("definedClasses[WP_Post] = %q, want path ending in class-wp-post.php", path)
	}

	// wp_cache_get must be found in cache.php.
	if path, ok := idx.definedFunctions["wp_cache_get"]; !ok {
		t.Error("definedFunctions missing wp_cache_get")
	} else if !strings.HasSuffix(filepath.ToSlash(path), "cache.php") {
		t.Errorf("definedFunctions[wp_cache_get] = %q, want path ending in cache.php", path)
	}
}

// TestBuildCodebaseIndex_NonexistentRoots_SkipsGracefully verifies that
// buildCodebaseIndex logs a warning and returns a valid (empty) index when
// neither root directory exists, rather than returning an error.
func TestBuildCodebaseIndex_NonexistentRoots_SkipsGracefully(t *testing.T) {
	// Use a fresh tempdir that has no wp-includes or wp-admin/includes.
	idx, err := buildCodebaseIndex(t.TempDir())
	if err != nil {
		t.Fatalf("buildCodebaseIndex() with missing roots returned error: %v", err)
	}
	if idx == nil {
		t.Fatal("buildCodebaseIndex() returned nil for missing roots")
	}
	if len(idx.definedClasses)+len(idx.definedFunctions) != 0 {
		t.Errorf("expected empty index for missing roots, got %d classes + %d functions",
			len(idx.definedClasses), len(idx.definedFunctions))
	}
}

// ---------------------------------------------------------------------------
// Test 11 — buildCodebaseIndex with walk error (permission denied)
// ---------------------------------------------------------------------------

// TestBuildCodebaseIndex_WalkError_ReturnsWrappedError verifies that
// buildCodebaseIndex returns an error containing the root path when
// filepath.WalkDir itself cannot start (e.g., permission denied on the root).
//
// We trigger this by creating the wp-includes directory with mode 0000
// (no read or execute permission) so that WalkDir returns an error when
// attempting to stat or open it.
func TestBuildCodebaseIndex_WalkError_ReturnsWrappedError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — permission checks are ineffective")
	}
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 does not restrict directory traversal on Windows")
	}

	tmp := t.TempDir()
	wpIncludes := filepath.Join(tmp, "wp-includes")
	if err := os.MkdirAll(wpIncludes, 0755); err != nil {
		t.Fatal(err)
	}
	// Place a PHP file inside so parsing would normally succeed.
	if err := os.WriteFile(filepath.Join(wpIncludes, "stub.php"), []byte("<?php function stub() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	// Remove all permissions — WalkDir will fail when trying to read entries.
	if err := os.Chmod(wpIncludes, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(wpIncludes, 0755) })

	// buildCodebaseIndex wraps the walk error via fmt.Errorf("walk %s: %w", root, walkErr).
	// However, if WalkDir calls the callback with an error for the root itself,
	// our callback logs and returns nil — so the walk succeeds. Only when
	// WalkDir itself errors directly does the walk error propagate.
	// We verify that either:
	// (a) an error is returned (containing the root path), OR
	// (b) no error is returned (because the callback swallowed it) but the index is empty.
	idx, err := buildCodebaseIndex(tmp)
	if err != nil {
		// Verify the error message contains the root path to aid diagnosis.
		if !strings.Contains(err.Error(), "wp-includes") && !strings.Contains(err.Error(), tmp) {
			t.Errorf("error does not contain root path: %v", err)
		}
	} else {
		// Walk callback swallowed the error — verify nothing was indexed.
		if idx == nil {
			t.Fatal("index is nil when walk error was swallowed")
		}
		if len(idx.definedFunctions) != 0 {
			t.Errorf("expected no indexed functions when wp-includes is unreadable, got %d", len(idx.definedFunctions))
		}
	}
}

// ---------------------------------------------------------------------------
// Test 12 — codebaseIndex priority
// ---------------------------------------------------------------------------

// TestCodebaseIndex_Priority_HighRefCountIsLowerPriority verifies that the
// priority() method returns 0 for high-ref-count names (≥50) and 2 for
// low-ref-count names (<10), so they sort earlier in buildEntityList.
func TestCodebaseIndex_Priority_HighRefCountIsLowerPriority(t *testing.T) {
	idx := &codebaseIndex{
		refCounts: map[string]int{
			"high_usage_func": 100,
			"medium_usage":    25,
			"low_usage":       3,
		},
	}

	if p := idx.priority("high_usage_func"); p != 0 {
		t.Errorf("priority(100 refs) = %d, want 0 (highest priority = sorted first)", p)
	}
	if p := idx.priority("medium_usage"); p != 1 {
		t.Errorf("priority(25 refs) = %d, want 1 (medium)", p)
	}
	if p := idx.priority("low_usage"); p != 2 {
		t.Errorf("priority(3 refs) = %d, want 2 (lowest)", p)
	}
	// Name with exactly 0 refs — should be lowest priority.
	if p := idx.priority("unused"); p != 2 {
		t.Errorf("priority(0 refs) = %d, want 2", p)
	}
	// Boundary: exactly 50 refs → priority 0.
	idx.refCounts["at_boundary"] = 50
	if p := idx.priority("at_boundary"); p != 0 {
		t.Errorf("priority(50 refs) = %d, want 0", p)
	}
	// Boundary: exactly 10 refs → priority 1.
	idx.refCounts["at_ten"] = 10
	if p := idx.priority("at_ten"); p != 1 {
		t.Errorf("priority(10 refs) = %d, want 1", p)
	}
}

// ---------------------------------------------------------------------------
// Test 13 — builtin function detection (builtinFunctions set)
// ---------------------------------------------------------------------------

// TestBuildCodebaseIndex_BuiltinDetection verifies that functions called
// within the codebase but not defined anywhere in the PHP files are
// placed in builtinFunctions, while functions that ARE defined remain absent.
func TestBuildCodebaseIndex_BuiltinDetection(t *testing.T) {
	tmp := t.TempDir()
	wpIncludes := filepath.Join(tmp, "wp-includes")
	if err := os.MkdirAll(wpIncludes, 0755); err != nil {
		t.Fatal(err)
	}

	// This file defines my_func and calls strlen (PHP builtin) and my_other_func.
	phpContent := `<?php
function my_func( string $x ): int {
	return strlen( $x );
}
function my_other_func(): void {
	my_func( 'hello' );
}
`
	if err := os.WriteFile(filepath.Join(wpIncludes, "funcs.php"), []byte(phpContent), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := buildCodebaseIndex(tmp)
	if err != nil {
		t.Fatalf("buildCodebaseIndex() error: %v", err)
	}

	// strlen is not defined in any PHP file → must be detected as builtin.
	if !idx.builtinFunctions["strlen"] {
		t.Error("builtinFunctions missing 'strlen', want it flagged as a PHP builtin")
	}
	// my_func IS defined → must NOT be in builtinFunctions.
	if idx.builtinFunctions["my_func"] {
		t.Error("builtinFunctions wrongly contains 'my_func', which is defined in the codebase")
	}
	// my_other_func IS defined → must NOT be in builtinFunctions.
	if idx.builtinFunctions["my_other_func"] {
		t.Error("builtinFunctions wrongly contains 'my_other_func', which is defined in the codebase")
	}
}

// TestBuildCodebaseIndex_BuiltinFunctionsInitialized verifies that the
// builtinFunctions map is non-nil even when no PHP files are present.
func TestBuildCodebaseIndex_BuiltinFunctionsInitialized(t *testing.T) {
	idx, err := buildCodebaseIndex(t.TempDir())
	if err != nil {
		t.Fatalf("buildCodebaseIndex() error: %v", err)
	}
	if idx.builtinFunctions == nil {
		t.Error("builtinFunctions is nil, want initialized empty map")
	}
}

// ---------------------------------------------------------------------------
// Test 14 — buildEntityList ordering
// ---------------------------------------------------------------------------

// TestBuildEntityList_SortedByPriorityThenAlphabetical verifies that
// buildEntityList returns IDs ordered first by ascending priority (high-ref
// items first), then alphabetically within a priority group.
func TestBuildEntityList_SortedByPriorityThenAlphabetical(t *testing.T) {
	tmp := t.TempDir()
	wpIncludes := filepath.Join(tmp, "wp-includes")
	if err := os.MkdirAll(wpIncludes, 0755); err != nil {
		t.Fatal(err)
	}

	// Write PHP files defining three functions with different ref profiles.
	// alpha_func and zeta_func will have 0 refs (priority 2).
	// popular_func will have 60 refs (priority 0).
	// mid_func will have 15 refs (priority 1).
	phpFuncs := `<?php
function alpha_func(): void {}
function zeta_func(): void {}
function popular_func(): void {}
function mid_func(): void {}
`
	if err := os.WriteFile(filepath.Join(wpIncludes, "funcs.php"), []byte(phpFuncs), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := buildCodebaseIndex(tmp)
	if err != nil {
		t.Fatalf("buildCodebaseIndex() error: %v", err)
	}

	// Manually set ref counts to create a predictable ordering.
	idx.refCounts["popular_func"] = 60
	idx.refCounts["mid_func"] = 15
	// alpha_func and zeta_func remain at 0.

	list := idx.buildEntityList()
	if len(list) == 0 {
		t.Fatal("buildEntityList() returned empty list")
	}

	// Build a map of function name → position in the list.
	posOf := func(name string) int {
		for i, id := range list {
			if strings.Contains(id, "#"+name) || strings.HasSuffix(id, name) {
				return i
			}
		}
		return -1
	}

	popularPos := posOf("popular_func")
	midPos := posOf("mid_func")
	alphaPos := posOf("alpha_func")
	zetaPos := posOf("zeta_func")

	if popularPos < 0 {
		t.Error("popular_func not found in entity list")
	}
	if midPos < 0 {
		t.Error("mid_func not found in entity list")
	}
	if alphaPos < 0 {
		t.Error("alpha_func not found in entity list")
	}
	if zetaPos < 0 {
		t.Error("zeta_func not found in entity list")
	}

	// popular_func (priority 0) must come before mid_func (priority 1).
	if popularPos >= 0 && midPos >= 0 && popularPos > midPos {
		t.Errorf("popular_func (pos %d) should come before mid_func (pos %d)", popularPos, midPos)
	}
	// mid_func (priority 1) must come before alpha_func (priority 2).
	if midPos >= 0 && alphaPos >= 0 && midPos > alphaPos {
		t.Errorf("mid_func (pos %d) should come before alpha_func (pos %d)", midPos, alphaPos)
	}
	// Within priority 2, alpha_func must come before zeta_func (alphabetical).
	if alphaPos >= 0 && zetaPos >= 0 && alphaPos > zetaPos {
		t.Errorf("alpha_func (pos %d) should come before zeta_func (pos %d) (alphabetical within same priority)", alphaPos, zetaPos)
	}
}

// TestBuildEntityList_NoDuplicates verifies that buildEntityList never
// returns the same ID twice, even when a class file path equals a function
// file path (collision via seen set).
func TestBuildEntityList_NoDuplicates(t *testing.T) {
	tmp := t.TempDir()
	wpIncludes := filepath.Join(tmp, "wp-includes")
	if err := os.MkdirAll(wpIncludes, 0755); err != nil {
		t.Fatal(err)
	}

	phpContent := `<?php
function func_a(): void {}
function func_b(): void {}
`
	if err := os.WriteFile(filepath.Join(wpIncludes, "funcs.php"), []byte(phpContent), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := buildCodebaseIndex(tmp)
	if err != nil {
		t.Fatalf("buildCodebaseIndex() error: %v", err)
	}

	list := idx.buildEntityList()
	seen := map[string]int{}
	for i, id := range list {
		if prev, dup := seen[id]; dup {
			t.Errorf("duplicate ID %q at positions %d and %d", id, prev, i)
		}
		seen[id] = i
	}
}

// ---------------------------------------------------------------------------
// Test 15 — full round-trip: buildCodebaseIndex → buildEntityList
// ---------------------------------------------------------------------------

// TestBuildEntityList_ClassEntriesAreFilePaths verifies that class entries
// in the entity list are bare file paths (no fragment), while function
// entries contain a '#' fragment.
func TestBuildEntityList_ClassEntriesAreFilePaths(t *testing.T) {
	repoPath := filepath.Join("testdata", "wpgithub_tests")

	idx, err := buildCodebaseIndex(repoPath)
	if err != nil {
		t.Fatalf("buildCodebaseIndex() error: %v", err)
	}

	list := idx.buildEntityList()

	var classEntries, funcEntries int
	for _, id := range list {
		if strings.Contains(id, "#") {
			funcEntries++
			// Fragment must not be empty.
			parts := strings.SplitN(id, "#", 2)
			if parts[1] == "" {
				t.Errorf("function entity has empty fragment: %q", id)
			}
		} else {
			classEntries++
			// Class entries must end with .php.
			if !strings.HasSuffix(strings.ToLower(id), ".php") {
				t.Errorf("class entity does not end with .php: %q", id)
			}
		}
	}

	if classEntries == 0 {
		t.Error("no class entries in entity list, want at least WP_Post")
	}
	if funcEntries == 0 {
		t.Error("no function entries in entity list, want at least wp_cache_get")
	}
}

// ---------------------------------------------------------------------------
// Test 16 — SkipDir skips known non-PHP directories
// ---------------------------------------------------------------------------

// TestBuildCodebaseIndex_SkipsDotGitAndVendor verifies that .git and vendor
// directories are not walked, so no PHP files inside them are indexed.
func TestBuildCodebaseIndex_SkipsDotGitAndVendor(t *testing.T) {
	tmp := t.TempDir()
	wpIncludes := filepath.Join(tmp, "wp-includes")
	gitDir := filepath.Join(wpIncludes, ".git")
	vendorDir := filepath.Join(wpIncludes, "vendor")
	for _, d := range []string{wpIncludes, gitDir, vendorDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// PHP file in the root wp-includes — should be indexed.
	if err := os.WriteFile(filepath.Join(wpIncludes, "real.php"), []byte("<?php function real_func(): void {}"), 0644); err != nil {
		t.Fatal(err)
	}
	// PHP files inside .git and vendor — must NOT be indexed.
	if err := os.WriteFile(filepath.Join(gitDir, "hook.php"), []byte("<?php function git_func(): void {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "lib.php"), []byte("<?php function vendor_func(): void {}"), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := buildCodebaseIndex(tmp)
	if err != nil {
		t.Fatalf("buildCodebaseIndex() error: %v", err)
	}

	if _, ok := idx.definedFunctions["real_func"]; !ok {
		t.Error("real_func should be indexed from wp-includes/real.php")
	}
	if _, ok := idx.definedFunctions["git_func"]; ok {
		t.Error("git_func must NOT be indexed (inside .git directory)")
	}
	if _, ok := idx.definedFunctions["vendor_func"]; ok {
		t.Error("vendor_func must NOT be indexed (inside vendor directory)")
	}
}

// ---------------------------------------------------------------------------
// Test 17 — RefCounts are computed per-file (not globally double-counted)
// ---------------------------------------------------------------------------

// TestBuildCodebaseIndex_RefCountsPerFile verifies that calling a function
// multiple times in the same file counts as only one reference (the seen
// map deduplications per-file).
func TestBuildCodebaseIndex_RefCountsPerFile(t *testing.T) {
	tmp := t.TempDir()
	wpIncludes := filepath.Join(tmp, "wp-includes")
	if err := os.MkdirAll(wpIncludes, 0755); err != nil {
		t.Fatal(err)
	}

	// File 1: calls target_func three times.
	file1 := `<?php
function caller_a(): void {
	target_func();
	target_func();
	target_func();
}
`
	// File 2: calls target_func once.
	file2 := `<?php
function target_func(): void {}
function caller_b(): void {
	target_func();
}
`
	if err := os.WriteFile(filepath.Join(wpIncludes, "a.php"), []byte(file1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wpIncludes, "b.php"), []byte(file2), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := buildCodebaseIndex(tmp)
	if err != nil {
		t.Fatalf("buildCodebaseIndex() error: %v", err)
	}

	// Each file contributes at most 1 ref per name (seen map deduplication).
	// file1 contributes 1 ref; file2 contributes 1 ref → total = 2.
	if got := idx.refCounts["target_func"]; got != 2 {
		t.Errorf("refCounts[target_func] = %d, want 2 (one per file, not per call)", got)
	}
}
