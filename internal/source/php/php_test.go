//go:build sqlite_fts5 || fts5

package php

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// readFixture reads a file from the testdata directory and returns its
// contents as a byte slice. Fails the test if the file cannot be read.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFixture(%q): %v", name, err)
	}
	return data
}

// seedWPRepo creates a minimal WordPress-like repository layout under dir,
// suitable for exercising DiscoverEntities and buildCodebaseIndex.
// It places the pre-built fixture tree from testdata/php_tests.
// Returns the repoPath.
func seedWPRepo(t *testing.T) string {
	t.Helper()
	// Use the pre-built fixture directory directly; tests must not mutate it.
	return filepath.Join("testdata", "php_tests")
}

// ---------------------------------------------------------------------------
// Test 1 — NewPHPSource construction and identity
// ---------------------------------------------------------------------------

// TestNew_ConstructsWithRepoPath verifies that New returns a non-nil
// PHPSource with the correct repoPath set, and that ID() and Meta()
// return the expected values without requiring a real WordPress clone.
func TestNew_ConstructsWithRepoPath(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.repoPath != tmpDir {
		t.Errorf("repoPath = %q, want %q", s.repoPath, tmpDir)
	}

	if got := s.ID(); got != "/wordpress" {
		t.Errorf("ID() = %q, want %q", got, "/wordpress")
	}

	meta := s.Meta()
	if meta.Name == "" {
		t.Error("Meta().Name is empty")
	}
	if meta.TrustScore <= 0 || meta.TrustScore > 1 {
		t.Errorf("Meta().TrustScore = %v, want (0,1]", meta.TrustScore)
	}
	if meta.SourceURL == "" {
		t.Error("Meta().SourceURL is empty")
	}
}

// TestNew_ImplementsSourceInterface ensures PHPSource satisfies the
// source.Source interface at compile time. The var _ assertion in php.go
// already checks this, but an explicit runtime assertion here documents the
// contract in the test binary.
func TestNew_ImplementsSourceInterface(t *testing.T) {
	var _ source.Source = New(t.TempDir())
}

// TestNew_MetaVersionUnknown_WhenNoVersionFile verifies that Meta().Version
// returns "unknown" when the repoPath does not contain a version file.
func TestNew_MetaVersionUnknown_WhenNoVersionFile(t *testing.T) {
	s := New(t.TempDir()) // empty dir — no wp-includes/version.php
	if got := s.Meta().Version; got != "unknown" {
		t.Errorf("Meta().Version = %q, want %q when no version file", got, "unknown")
	}
}

// TestNew_IndexInitializedNonNil verifies that New() initialises s.index to a
// non-nil empty index so that DetectWrapper and ResolveWrapperURL can be
// called safely before DiscoverEntities.
func TestNew_IndexInitializedNonNil(t *testing.T) {
	s := New(t.TempDir())
	if s.index == nil {
		t.Fatal("New() left s.index nil; want non-nil empty index")
	}
	// The empty index should have no classes or functions yet.
	if len(s.index.definedClasses) != 0 {
		t.Errorf("New() index has %d classes, want 0", len(s.index.definedClasses))
	}
	if len(s.index.definedFunctions) != 0 {
		t.Errorf("New() index has %d functions, want 0", len(s.index.definedFunctions))
	}
}

// ---------------------------------------------------------------------------
// Test 2 — DiscoverEntities with seeded WordPress-like dir
// ---------------------------------------------------------------------------

// TestDiscoverEntities_ReturnsEntityURLs verifies that DiscoverEntities
// populates the index and returns at least one entity URL for each PHP file
// placed in the wp-includes and wp-admin/includes fixture roots.
func TestDiscoverEntities_ReturnsEntityURLs(t *testing.T) {
	repoPath := seedWPRepo(t)
	s := New(repoPath)

	ids, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities() error: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("DiscoverEntities() returned 0 entities, want > 0")
	}

	// Every returned ID must be a non-empty string pointing at a .php file
	// or a fragment URL ending with #functionName.
	for _, id := range ids {
		if id == "" {
			t.Error("DiscoverEntities() returned an empty string ID")
		}
		base := id
		if idx := strings.Index(id, "#"); idx >= 0 {
			base = id[:idx]
		}
		if !strings.HasSuffix(strings.ToLower(base), ".php") {
			t.Errorf("DiscoverEntities() returned non-.php ID: %q", id)
		}
	}

	// After DiscoverEntities, the index must be populated.
	if s.index == nil {
		t.Fatal("s.index is nil after DiscoverEntities()")
	}
}

// TestDiscoverEntities_SetsIndex verifies the index is populated with classes
// and functions from the fixture repo.
func TestDiscoverEntities_SetsIndex(t *testing.T) {
	repoPath := seedWPRepo(t)
	s := New(repoPath)

	_, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities() error: %v", err)
	}

	if s.index == nil {
		t.Fatal("s.index is nil after DiscoverEntities()")
	}
	if len(s.index.definedClasses)+len(s.index.definedFunctions) == 0 {
		t.Error("index has no classes or functions after DiscoverEntities()")
	}
}

// ---------------------------------------------------------------------------
// Test 3 — ParseEntity / ParseMethod round-trip
// ---------------------------------------------------------------------------

// TestParseEntity_ClassFile parses the simple_class.php fixture as a class
// entity and verifies the returned Entity has correct fields.
func TestParseEntity_ClassFile(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "simple_class.php")
	absPath := filepath.Join("testdata", "simple_class.php")

	entity, methodURLs, err := s.ParseEntity(context.Background(), absPath, body)
	if err != nil {
		t.Fatalf("ParseEntity() error: %v", err)
	}
	if entity == nil {
		t.Fatal("ParseEntity() returned nil entity")
	}
	if entity.Kind != "class" {
		t.Errorf("entity.Kind = %q, want %q", entity.Kind, "class")
	}
	if entity.Name != "WP_Post" {
		t.Errorf("entity.Name = %q, want %q", entity.Name, "WP_Post")
	}
	if entity.Slug != "wp_post" {
		t.Errorf("entity.Slug = %q, want %q (must be lowercased)", entity.Slug, "wp_post")
	}
	if entity.Description == "" {
		t.Error("entity.Description is empty, want non-empty PHPDoc description")
	}
	if len(entity.Properties) == 0 {
		t.Error("entity.Properties is empty, want at least 2 properties")
	}
	if len(methodURLs) == 0 {
		t.Error("methodURLs is empty, want at least 2 method URLs")
	}
	// Method URLs must contain a fragment with ClassName::methodName.
	for _, mu := range methodURLs {
		if !strings.Contains(mu, "#WP_Post::") {
			t.Errorf("methodURL %q does not contain expected fragment pattern", mu)
		}
	}
}

// TestParseMethod_ClassMethod parses a specific method from simple_class.php
// and verifies the returned Method has correct fields.
func TestParseMethod_ClassMethod(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "simple_class.php")
	absPath := filepath.Join("testdata", "simple_class.php")
	// Construct the method URL as the crawler would: path#ClassName::methodName
	methodURL := absPath + "#WP_Post::get_instance"

	method, err := s.ParseMethod(context.Background(), methodURL, body)
	if err != nil {
		t.Fatalf("ParseMethod() error: %v", err)
	}
	if method == nil {
		t.Fatal("ParseMethod() returned nil")
	}
	if method.Name != "get_instance" {
		t.Errorf("method.Name = %q, want %q", method.Name, "get_instance")
	}
	if method.Slug != "get_instance" {
		t.Errorf("method.Slug = %q, want %q (must be lowercase name)", method.Slug, "get_instance")
	}
	if method.Signature == "" {
		t.Error("method.Signature is empty, want non-empty")
	}
	if method.Description == "" {
		t.Error("method.Description is empty, want PHPDoc description")
	}
	if method.SourceCode == "" {
		t.Error("method.SourceCode is empty, want method body bytes")
	}
	if method.URL == "" {
		t.Error("method.URL is empty, want GitHub URL")
	}
}

// TestParseMethod_NoFragment_ReturnsError verifies that ParseMethod returns
// an error when the URL has no fragment.
func TestParseMethod_NoFragment_ReturnsError(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "simple_class.php")
	_, err := s.ParseMethod(context.Background(), "testdata/simple_class.php", body)
	if err == nil {
		t.Error("ParseMethod() with no fragment returned nil error, want error")
	}
}

// TestParseMethod_InvalidFragment_ReturnsError verifies that ParseMethod
// returns an error when the fragment does not contain "::".
func TestParseMethod_InvalidFragment_ReturnsError(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "simple_class.php")
	_, err := s.ParseMethod(context.Background(), "testdata/simple_class.php#justname", body)
	if err == nil {
		t.Error("ParseMethod() with non-method fragment returned nil error, want error")
	}
}

// ---------------------------------------------------------------------------
// Test 4 — ParseSourceCode extracts source by name
// ---------------------------------------------------------------------------

// TestParseSourceCode_FunctionByFragment verifies that ParseSourceCode
// extracts the source code of a specific function identified by URL fragment.
func TestParseSourceCode_FunctionByFragment(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "top_level_functions.php")
	url := "testdata/top_level_functions.php#get_post"

	src, err := s.ParseSourceCode(url, body)
	if err != nil {
		t.Fatalf("ParseSourceCode() error: %v", err)
	}
	if src == "" {
		t.Error("ParseSourceCode() returned empty source, want non-empty")
	}
	if !strings.Contains(src, "get_post") {
		t.Errorf("ParseSourceCode() returned source not containing function name: %q", src)
	}
}

// TestParseSourceCode_MethodByFragment verifies that ParseSourceCode
// extracts a class method's source when the fragment is ClassName::methodName.
func TestParseSourceCode_MethodByFragment(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "simple_class.php")
	url := "testdata/simple_class.php#WP_Post::get_id"

	src, err := s.ParseSourceCode(url, body)
	if err != nil {
		t.Fatalf("ParseSourceCode() error: %v", err)
	}
	if src == "" {
		t.Error("ParseSourceCode() returned empty source, want non-empty")
	}
	if !strings.Contains(src, "get_id") {
		t.Errorf("ParseSourceCode() returned source not containing method name: %q", src)
	}
}

// TestParseSourceCode_NoFragment_ReturnsWholeFile verifies that
// ParseSourceCode returns the full file contents when no fragment is present.
func TestParseSourceCode_NoFragment_ReturnsWholeFile(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "top_level_functions.php")
	url := "testdata/top_level_functions.php"

	src, err := s.ParseSourceCode(url, body)
	if err != nil {
		t.Fatalf("ParseSourceCode() error: %v", err)
	}
	if string(src) != string(body) {
		t.Error("ParseSourceCode() without fragment should return full file contents")
	}
}

// TestParseSourceCode_UnknownFunction_ReturnsError verifies that
// ParseSourceCode returns an error when the fragment names a function that
// does not exist in the file.
func TestParseSourceCode_UnknownFunction_ReturnsError(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "top_level_functions.php")
	url := "testdata/top_level_functions.php#nonexistent_function"

	_, err := s.ParseSourceCode(url, body)
	if err == nil {
		t.Error("ParseSourceCode() with unknown function returned nil error, want error")
	}
}

// ---------------------------------------------------------------------------
// Test 5 — DetectWrapper on a function that wraps another
// ---------------------------------------------------------------------------

// TestDetectWrapper_WrapperFunction verifies that DetectWrapper correctly
// identifies get_transient as a wrapper around wp_cache_get.
func TestDetectWrapper_WrapperFunction(t *testing.T) {
	repoPath := seedWPRepo(t)
	s := New(repoPath)

	// Prime the index so BuiltinFunctions is populated.
	_, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities() error: %v", err)
	}

	body := readFixture(t, "wrapper_function.php")
	url := "testdata/wrapper_function.php#get_transient"

	_, _, err = s.ParseEntity(context.Background(), url, body)
	if err != nil {
		t.Fatalf("ParseEntity() error: %v", err)
	}

	// Extract the source code for the get_transient function.
	src, err := s.ParseSourceCode(url, body)
	if err != nil {
		t.Fatalf("ParseSourceCode() error: %v", err)
	}

	m := &source.Method{
		Name:       "get_transient",
		SourceCode: src,
	}

	isWrapper, targetName, targetKind := s.DetectWrapper(m)
	if !isWrapper {
		t.Error("DetectWrapper() returned isWrapper=false for a known wrapper function")
	}
	if targetName != "wp_cache_get" {
		t.Errorf("DetectWrapper() targetName = %q, want %q", targetName, "wp_cache_get")
	}
	if targetKind != "function" {
		t.Errorf("DetectWrapper() targetKind = %q, want %q", targetKind, "function")
	}
}

// ---------------------------------------------------------------------------
// Test 6 — DetectWrapper does NOT chain into PHP builtins
// ---------------------------------------------------------------------------

// TestDetectWrapper_PHPBuiltin_NotChained verifies that DetectWrapper returns
// (false, "", "") when the wrapped function is in the BuiltinFunctions set.
// This exercises the phpBuiltins guard: functions not defined in the WP
// codebase should stop the wrapper chain.
func TestDetectWrapper_PHPBuiltin_NotChained(t *testing.T) {
	repoPath := seedWPRepo(t)
	s := New(repoPath)

	_, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities() error: %v", err)
	}

	// wp_strlen wraps strlen(), which is a PHP builtin not defined in the repo.
	body := readFixture(t, "builtin_wrapper.php")
	url := "testdata/builtin_wrapper.php#wp_strlen"

	src, err := s.ParseSourceCode(url, body)
	if err != nil {
		t.Fatalf("ParseSourceCode() error: %v", err)
	}

	// strlen must be in BuiltinFunctions because it is called in cache.php
	// (current_user_can is called in admin-helpers.php — also a builtin).
	// Manually ensure strlen is treated as a builtin for this test.
	s.index.builtinFunctions["strlen"] = true

	m := &source.Method{
		Name:       "wp_strlen",
		SourceCode: src,
	}

	isWrapper, targetName, targetKind := s.DetectWrapper(m)
	if isWrapper {
		t.Errorf("DetectWrapper() returned isWrapper=true for a PHP builtin wrapper; targetName=%q targetKind=%q", targetName, targetKind)
	}
	if targetName != "" || targetKind != "" {
		t.Errorf("DetectWrapper() returned non-empty target for builtin: name=%q kind=%q", targetName, targetKind)
	}
}

// TestDetectWrapper_NilMethod_ReturnsFalse verifies that DetectWrapper does
// not panic and returns (false,"","") when given a nil Method.
func TestDetectWrapper_NilMethod_ReturnsFalse(t *testing.T) {
	s := New(t.TempDir())
	isWrapper, name, kind := s.DetectWrapper(nil)
	if isWrapper || name != "" || kind != "" {
		t.Errorf("DetectWrapper(nil) = (%v,%q,%q), want (false,\"\",\"\")", isWrapper, name, kind)
	}
}

// ---------------------------------------------------------------------------
// Test 7 — ResolveWrapperURL
// ---------------------------------------------------------------------------

// TestResolveWrapperURL_KnownFunction verifies that ResolveWrapperURL returns
// a non-empty URL for a function present in the index.
func TestResolveWrapperURL_KnownFunction(t *testing.T) {
	repoPath := seedWPRepo(t)
	s := New(repoPath)

	_, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities() error: %v", err)
	}

	// wp_cache_get is defined in cache.php in the fixture repo.
	url := s.ResolveWrapperURL("wp_cache_get", "function", "")
	if url == "" {
		t.Error("ResolveWrapperURL() returned empty string for known function")
	}
	if !strings.Contains(url, "#wp_cache_get") {
		t.Errorf("ResolveWrapperURL() = %q, want URL with #wp_cache_get fragment", url)
	}
}

// TestResolveWrapperURL_UnknownFunction_ReturnsEmpty verifies that
// ResolveWrapperURL returns "" for a function not in the index.
func TestResolveWrapperURL_UnknownFunction_ReturnsEmpty(t *testing.T) {
	repoPath := seedWPRepo(t)
	s := New(repoPath)

	_, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities() error: %v", err)
	}

	url := s.ResolveWrapperURL("nonexistent_function_xyz", "function", "")
	if url != "" {
		t.Errorf("ResolveWrapperURL() = %q, want empty string for unknown function", url)
	}
}

// TestResolveWrapperURL_StaticMethod verifies that ResolveWrapperURL returns
// a valid URL for a static method via ClassName::methodName.
func TestResolveWrapperURL_StaticMethod(t *testing.T) {
	repoPath := seedWPRepo(t)
	s := New(repoPath)

	_, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities() error: %v", err)
	}

	// WP_Post is defined in class-wp-post.php in the fixture repo.
	url := s.ResolveWrapperURL("WP_Post::get_instance", "static_method", "")
	if url == "" {
		t.Error("ResolveWrapperURL() returned empty string for known static method")
	}
	if !strings.Contains(url, "#WP_Post::get_instance") {
		t.Errorf("ResolveWrapperURL() = %q, want URL containing #WP_Post::get_instance", url)
	}
}

// ---------------------------------------------------------------------------
// Test 8 — Fragment URL handling
// ---------------------------------------------------------------------------

// TestFragmentURL_ParseEntity_FunctionEntity verifies that ParseEntity
// correctly dispatches to parseFunctionEntity when the URL has a '#' fragment,
// and that the returned entity has the slug derived from the fragment (not h1).
func TestFragmentURL_ParseEntity_FunctionEntity(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "top_level_functions.php")
	url := "testdata/top_level_functions.php#get_post"

	entity, methodURLs, err := s.ParseEntity(context.Background(), url, body)
	if err != nil {
		t.Fatalf("ParseEntity() error: %v", err)
	}
	if entity.Kind != "function" {
		t.Errorf("entity.Kind = %q, want %q", entity.Kind, "function")
	}
	if entity.Slug != "get_post" {
		t.Errorf("entity.Slug = %q, want %q (slug from fragment, not h1)", entity.Slug, "get_post")
	}
	if len(methodURLs) != 0 {
		t.Errorf("ParseEntity() for function returned %d methodURLs, want 0", len(methodURLs))
	}
}

// TestFragmentURL_ParseSourceCode_MethodFragment verifies that
// ParseSourceCode correctly handles a fragment of the form ClassName::method.
func TestFragmentURL_ParseSourceCode_MethodFragment(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "mixed_class_and_functions.php")
	// Fragment with "::" — method lookup path.
	url := "testdata/mixed_class_and_functions.php#WP_Query::get_posts"

	src, err := s.ParseSourceCode(url, body)
	if err != nil {
		t.Fatalf("ParseSourceCode() with method fragment error: %v", err)
	}
	if src == "" {
		t.Error("ParseSourceCode() returned empty source for method fragment")
	}
	if !strings.Contains(src, "get_posts") {
		t.Errorf("ParseSourceCode() result %q does not contain method name", src)
	}
}

// TestFragmentURL_ParseMethod_SplitsCorrectly verifies that ParseMethod
// correctly splits a fragment of the form ClassName::methodName into its
// constituent parts and finds the matching method.
func TestFragmentURL_ParseMethod_SplitsCorrectly(t *testing.T) {
	s := New(t.TempDir())
	body := readFixture(t, "mixed_class_and_functions.php")
	url := "testdata/mixed_class_and_functions.php#WP_Query::get"

	method, err := s.ParseMethod(context.Background(), url, body)
	if err != nil {
		t.Fatalf("ParseMethod() error: %v", err)
	}
	if method.Name != "get" {
		t.Errorf("method.Name = %q, want %q", method.Name, "get")
	}
}

// ---------------------------------------------------------------------------
// Test 9 — DetectWrapper before DiscoverEntities (temporal coupling)
// ---------------------------------------------------------------------------

// TestDetectWrapper_BeforeDiscoverEntities_CharacterizesZeroValues verifies
// that DetectWrapper does not panic when called before DiscoverEntities.
// New() pre-initialises the index to a non-nil empty state (emptyIndex),
// so DetectWrapper and ResolveWrapperURL are safe to call at any point.
// A function body with more than 5 statements is used to guarantee a
// (false,"","") result regardless of which builtins are in the index.
func TestDetectWrapper_BeforeDiscoverEntities_CharacterizesZeroValues(t *testing.T) {
	// Fresh source — New() pre-initialises index to an empty non-nil state so
	// DetectWrapper and ResolveWrapperURL are safe to call before DiscoverEntities.
	s := New(t.TempDir())
	if s.index == nil {
		t.Fatal("precondition failed: index must be non-nil (emptyIndex) after New()")
	}

	// 6 statements — exceeds maxWrapperStatements=5, so isWrapper must be false.
	m := &source.Method{
		Name: "some_func",
		SourceCode: `{
			$a = init_a();
			$b = init_b();
			$c = init_c();
			$d = init_d();
			$e = init_e();
			return compute($a, $b, $c, $d, $e);
		}`,
	}

	// Must not panic.
	isWrapper, name, kind := s.DetectWrapper(m)
	if isWrapper || name != "" || kind != "" {
		t.Errorf("DetectWrapper before DiscoverEntities = (%v,%q,%q), want (false,\"\",\"\")", isWrapper, name, kind)
	}
}

// TestResolveWrapperURL_BeforeDiscoverEntities_ReturnsEmpty characterizes that
// ResolveWrapperURL returns "" before DiscoverEntities is called.
func TestResolveWrapperURL_BeforeDiscoverEntities_ReturnsEmpty(t *testing.T) {
	s := New(t.TempDir())
	if s.index == nil {
		t.Fatal("precondition: index must be non-nil (emptyIndex) after New()")
	}

	url := s.ResolveWrapperURL("wp_cache_get", "function", "")
	if url != "" {
		t.Errorf("ResolveWrapperURL before DiscoverEntities = %q, want empty string", url)
	}
}

// ---------------------------------------------------------------------------
// Additional helper tests
// ---------------------------------------------------------------------------

// TestSplitFragment verifies the splitFragment helper function.
func TestSplitFragment(t *testing.T) {
	cases := []struct {
		input    string
		wantBase string
		wantFrag string
	}{
		{"path/to/file.php#FuncName", "path/to/file.php", "FuncName"},
		{"path/to/file.php#Class::method", "path/to/file.php", "Class::method"},
		{"path/to/file.php", "path/to/file.php", ""},
		{"#fragment", "", "fragment"},
		{"", "", ""},
	}
	for _, tc := range cases {
		gotBase, gotFrag := splitFragment(tc.input)
		if gotBase != tc.wantBase || gotFrag != tc.wantFrag {
			t.Errorf("splitFragment(%q) = (%q, %q), want (%q, %q)",
				tc.input, gotBase, gotFrag, tc.wantBase, tc.wantFrag)
		}
	}
}

// TestLineNumber_CountsCorrectly verifies the lineNumber helper function.
func TestLineNumber_CountsCorrectly(t *testing.T) {
	cases := []struct {
		content string
		pos     int
		want    int
	}{
		{"abc", 0, 1},
		{"abc\ndef", 4, 2},
		{"a\nb\nc", 4, 3},
		{"abc", 10, 1}, // pos beyond end: clamped
	}
	for _, tc := range cases {
		got := lineNumber([]byte(tc.content), tc.pos)
		if got != tc.want {
			t.Errorf("lineNumber(%q, %d) = %d, want %d", tc.content, tc.pos, got, tc.want)
		}
	}
}

// TestRelativePath_ComputesRelative verifies relativePath returns a clean
// slash-separated path relative to repoPath.
func TestRelativePath_ComputesRelative(t *testing.T) {
	tmp := t.TempDir()
	abs := filepath.Join(tmp, "wp-includes", "class-wp-post.php")
	rel := relativePath(tmp, abs)
	if strings.Contains(rel, "\\") {
		t.Errorf("relativePath() returned Windows-style path: %q", rel)
	}
	if rel != "wp-includes/class-wp-post.php" {
		t.Errorf("relativePath() = %q, want %q", rel, "wp-includes/class-wp-post.php")
	}
}

// TestRelativePath_EmptyRepoPath_ReturnsAbsPath verifies that relativePath
// falls back to absPath when repoPath is empty.
func TestRelativePath_EmptyRepoPath_ReturnsAbsPath(t *testing.T) {
	absPath := "/some/absolute/path.php"
	got := relativePath("", absPath)
	if got != absPath {
		t.Errorf("relativePath(\"\", %q) = %q, want %q", absPath, got, absPath)
	}
}

// TestLookupClassFile_CaseInsensitive verifies that lookupClassFile finds a
// class using a lowercase lookup key even when the index stores the canonical
// case.
func TestLookupClassFile_CaseInsensitive(t *testing.T) {
	idx := map[string]string{
		"WP_Post": "/path/to/class-wp-post.php",
	}
	path, ok := lookupClassFile(idx, "wp_post")
	if !ok {
		t.Error("lookupClassFile() returned ok=false for valid case-insensitive lookup")
	}
	if path != "/path/to/class-wp-post.php" {
		t.Errorf("lookupClassFile() path = %q, want %q", path, "/path/to/class-wp-post.php")
	}
}

// TestCanonicalClassName_ReturnsOriginalCase verifies that
// canonicalClassName returns the original casing from the index.
func TestCanonicalClassName_ReturnsOriginalCase(t *testing.T) {
	idx := map[string]string{
		"WP_Query": "/path/to/class-wp-query.php",
	}
	got := canonicalClassName(idx, "wp_query")
	if got != "WP_Query" {
		t.Errorf("canonicalClassName() = %q, want %q", got, "WP_Query")
	}
}

// TestSplitTopLevel_CommaOutsideParens splits a parameter list.
func TestSplitTopLevel_CommaOutsideParens(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"a, b, c", []string{"a", " b", " c"}},
		{"a, array(1, 2), c", []string{"a", " array(1, 2)", " c"}},
		{"'a,b', c", []string{"'a,b'", " c"}},
		{"$a = 1", []string{"$a = 1"}},
		// Escape sequences inside string literals: the '\,' must not split.
		{`'a\,b', c`, []string{`'a\,b'`, " c"}},
	}
	for _, tc := range cases {
		got := splitTopLevel(tc.input, ',')
		if len(got) != len(tc.want) {
			t.Errorf("splitTopLevel(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitTopLevel(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

// TestDetectVersion_ReadsVersionFile verifies that detectVersion correctly
// reads the WordPress version from wp-includes/version.php.
func TestDetectVersion_ReadsVersionFile(t *testing.T) {
	tmp := t.TempDir()
	wpIncludes := filepath.Join(tmp, "wp-includes")
	if err := os.MkdirAll(wpIncludes, 0755); err != nil {
		t.Fatal(err)
	}
	versionFile := filepath.Join(wpIncludes, "version.php")
	content := []byte("<?php\n$wp_version = '6.5.3';\n")
	if err := os.WriteFile(versionFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	got := detectVersion(tmp)
	if got != "6.5.3" {
		t.Errorf("detectVersion() = %q, want %q", got, "6.5.3")
	}
}

// TestDetectVersion_ReturnsUnknown_WhenFileAbsent verifies that detectVersion
// returns "unknown" when the version file does not exist.
func TestDetectVersion_ReturnsUnknown_WhenFileAbsent(t *testing.T) {
	got := detectVersion(t.TempDir())
	if got != "unknown" {
		t.Errorf("detectVersion() = %q, want %q", got, "unknown")
	}
}
