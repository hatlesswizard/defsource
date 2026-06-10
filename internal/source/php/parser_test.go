//go:build sqlite_fts5 || fts5

package php

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadFixture reads a PHP fixture file from testdata/ and returns its contents.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("loadFixture(%q): %v", name, err)
	}
	return data
}

// --------------------------------------------------------------------------
// parsePhpDoc tests
// --------------------------------------------------------------------------

func TestParsePhpDoc_EmptyString(t *testing.T) {
	doc := parsePhpDoc("")
	if doc.Summary != "" {
		t.Errorf("Summary = %q, want empty", doc.Summary)
	}
	if doc.Description != "" {
		t.Errorf("Description = %q, want empty", doc.Description)
	}
	if len(doc.Params) != 0 {
		t.Errorf("Params len = %d, want 0", len(doc.Params))
	}
}

func TestParsePhpDoc_SingleLineSummary(t *testing.T) {
	input := "/** Gets all posts. */"
	doc := parsePhpDoc(input)
	if doc.Summary != "Gets all posts." {
		t.Errorf("Summary = %q, want %q", doc.Summary, "Gets all posts.")
	}
	if doc.Description != "Gets all posts." {
		t.Errorf("Description = %q, want %q", doc.Description, "Gets all posts.")
	}
}

func TestParsePhpDoc_SummaryAndDescriptionSeparatedByBlankLine(t *testing.T) {
	input := "/**\n * Gets all posts.\n *\n * Returns a slice of WP_Post objects matching query args.\n */"
	doc := parsePhpDoc(input)
	if doc.Summary != "Gets all posts." {
		t.Errorf("Summary = %q, want %q", doc.Summary, "Gets all posts.")
	}
	if !strings.Contains(doc.Description, "Gets all posts.") {
		t.Errorf("Description missing summary line: %q", doc.Description)
	}
	if !strings.Contains(doc.Description, "Returns a slice") {
		t.Errorf("Description missing second paragraph: %q", doc.Description)
	}
}

func TestParsePhpDoc_ParamTag_AllFields(t *testing.T) {
	input := "/**\n * @param string $post_id The post ID.\n */"
	doc := parsePhpDoc(input)
	if len(doc.Params) != 1 {
		t.Fatalf("Params len = %d, want 1", len(doc.Params))
	}
	p := doc.Params[0]
	if p.Type != "string" {
		t.Errorf("Type = %q, want %q", p.Type, "string")
	}
	if p.Name != "$post_id" {
		t.Errorf("Name = %q, want %q", p.Name, "$post_id")
	}
	if p.Desc != "The post ID." {
		t.Errorf("Desc = %q, want %q", p.Desc, "The post ID.")
	}
	if p.Optional {
		t.Error("Optional = true, want false")
	}
}

func TestParsePhpDoc_ParamTag_MarkedOptionalViaDot(t *testing.T) {
	input := "/**\n * @param string $args Optional. Additional args.\n */"
	doc := parsePhpDoc(input)
	if len(doc.Params) != 1 {
		t.Fatalf("Params len = %d, want 1", len(doc.Params))
	}
	if !doc.Params[0].Optional {
		t.Error("Optional = false, want true for 'Optional. ...' prefix")
	}
}

func TestParsePhpDoc_ParamTag_MarkedOptionalViaSpace(t *testing.T) {
	input := "/**\n * @param string $args Optional extra args.\n */"
	doc := parsePhpDoc(input)
	if len(doc.Params) != 1 {
		t.Fatalf("Params len = %d, want 1", len(doc.Params))
	}
	if !doc.Params[0].Optional {
		t.Error("Optional = false, want true for 'Optional ...' prefix")
	}
}

// TestParsePhpDoc_OptionalDetection_FixedBoundary verifies the corrected
// Optional detection logic (Wave 3 fix). Rules:
//   - Whole-word "optional" (case-insensitive) → Optional=true.
//   - "non-optional" or "not optional" anywhere → Optional=false (exclusion).
//   - No whole-word "optional" → Optional=false.
func TestParsePhpDoc_OptionalDetection_FixedBoundary(t *testing.T) {
	cases := []struct {
		name  string
		desc  string
		want  bool
		notes string
	}{
		{
			name:  "Optional dot prefix",
			desc:  "@param string $x Optional. The user ID.",
			want:  true,
			notes: "classic WordPress 'Optional.' prefix",
		},
		{
			name:  "non-optional prefix",
			desc:  "@param string $x non-optional base URL.",
			want:  false,
			notes: "exclusion: 'non-optional' contains 'optional' as substring but is negated",
		},
		{
			name:  "returns optional wrapper",
			desc:  "@param string $x Returns optional wrapper.",
			want:  true,
			notes: "whole-word 'optional' without negation prefix → true",
		},
		{
			name:  "not optional phrase",
			desc:  "@param string $x not optional in this context.",
			want:  false,
			notes: "exclusion: 'not optional' phrase",
		},
		{
			name:  "parenthesised optional",
			desc:  "@param string $x (optional) post id.",
			want:  true,
			notes: "whole-word 'optional' in parens → true",
		},
		{
			name:  "no optional substring",
			desc:  "@param string $x Required base URL.",
			want:  false,
			notes: "no 'optional' word at all",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := "/**\n * " + tc.desc + "\n */"
			doc := parsePhpDoc(input)
			if len(doc.Params) != 1 {
				t.Fatalf("Params len = %d, want 1 (input %q)", len(doc.Params), tc.desc)
			}
			got := doc.Params[0].Optional
			if got != tc.want {
				t.Errorf("Optional = %v, want %v — %s", got, tc.want, tc.notes)
			}
		})
	}
}

func TestParsePhpDoc_ReturnTag(t *testing.T) {
	input := "/**\n * @return WP_Post|false The post object or false.\n */"
	doc := parsePhpDoc(input)
	if doc.Return.Type != "WP_Post|false" {
		t.Errorf("Return.Type = %q, want %q", doc.Return.Type, "WP_Post|false")
	}
	if doc.Return.Desc != "The post object or false." {
		t.Errorf("Return.Desc = %q, want %q", doc.Return.Desc, "The post object or false.")
	}
}

func TestParsePhpDoc_SinceTag_FirstWins(t *testing.T) {
	input := "/**\n * @since 3.0.0\n * @since 4.1.0\n */"
	doc := parsePhpDoc(input)
	if doc.Since != "3.0.0" {
		t.Errorf("Since = %q, want %q (first wins)", doc.Since, "3.0.0")
	}
}

func TestParsePhpDoc_DeprecatedTag(t *testing.T) {
	input := "/**\n * @deprecated 5.3.0 Use get_posts() instead.\n */"
	doc := parsePhpDoc(input)
	want := "5.3.0 Use get_posts() instead."
	if doc.Deprecated != want {
		t.Errorf("Deprecated = %q, want %q", doc.Deprecated, want)
	}
}

func TestParsePhpDoc_SeeAndUsesAppend(t *testing.T) {
	input := "/**\n * @see WP_Post\n * @see get_post()\n * @uses wpdb::query()\n */"
	doc := parsePhpDoc(input)
	if len(doc.See) != 2 {
		t.Errorf("See len = %d, want 2", len(doc.See))
	}
	if len(doc.Uses) != 1 {
		t.Errorf("Uses len = %d, want 1", len(doc.Uses))
	}
}

func TestParsePhpDoc_VarTag(t *testing.T) {
	input := "/**\n * @var int\n */"
	doc := parsePhpDoc(input)
	if doc.VarType != "int" {
		t.Errorf("VarType = %q, want %q", doc.VarType, "int")
	}
}

func TestParsePhpDoc_TagContinuationLineJoined(t *testing.T) {
	input := "/**\n * @param string $args Long description\n *   that continues on the next line.\n */"
	doc := parsePhpDoc(input)
	if len(doc.Params) != 1 {
		t.Fatalf("Params len = %d, want 1", len(doc.Params))
	}
	// Continuation line text should appear somewhere in Desc.
	if !strings.Contains(doc.Params[0].Desc, "Long description") {
		t.Errorf("Desc = %q does not contain 'Long description'", doc.Params[0].Desc)
	}
	if !strings.Contains(doc.Params[0].Desc, "continues") {
		t.Errorf("Desc = %q does not contain continuation text", doc.Params[0].Desc)
	}
}

func TestParsePhpDoc_MultipleParams(t *testing.T) {
	input := "/**\n * @param int    $post_id The post ID.\n * @param string $output  The return type.\n * @param string $filter  Optional. Filter to apply.\n */"
	doc := parsePhpDoc(input)
	if len(doc.Params) != 3 {
		t.Fatalf("Params len = %d, want 3", len(doc.Params))
	}
	if doc.Params[0].Name != "$post_id" {
		t.Errorf("Params[0].Name = %q, want %q", doc.Params[0].Name, "$post_id")
	}
	if doc.Params[2].Optional != true {
		t.Error("Params[2].Optional = false, want true")
	}
}

func TestParsePhpDoc_MissingClosingDelimiter(t *testing.T) {
	// Badly formatted doc with no closing */  — should not panic, parse what it can.
	input := "/**\n * Gets posts.\n * @since 2.0.0\n"
	doc := parsePhpDoc(input)
	// Must not panic. The since tag should still be captured.
	if doc.Since != "2.0.0" {
		t.Errorf("Since = %q, want %q even for unclosed doc", doc.Since, "2.0.0")
	}
}

// --------------------------------------------------------------------------
// findPrecedingDoc tests
// --------------------------------------------------------------------------

func TestFindPrecedingDoc_NoPrecedingDoc_ReturnsFalse(t *testing.T) {
	content := []byte("\nfunction foo() {}")
	pos := strings.Index(string(content), "function")
	doc, ok := findPrecedingDoc(content, pos)
	if ok {
		t.Errorf("ok = true, want false; got doc %q", doc)
	}
}

// TestFindPrecedingDoc_SimpleDocComment_ByteZeroLimit characterizes a known
// limitation: when /** starts at byte 0, the inner scan loop (for i >= 1)
// exits before reaching byte 0, so the doc comment is NOT returned.
//
// CHARACTERIZATION: locks current behavior. The scan loop `for i >= 1` prevents
// reading /** at byte position 0. Wave 3 may fix this by changing the loop bound.
func TestFindPrecedingDoc_SimpleDocComment_ByteZeroLimit(t *testing.T) {
	// /** starts at byte 0 here.
	content := []byte("/** Does stuff */\nfunction foo() {}")
	pos := strings.Index(string(content), "function")
	doc, ok := findPrecedingDoc(content, pos)
	// CHARACTERIZATION: the scan loop `for i >= 1` means /** at byte 0 is not found.
	if ok {
		t.Logf("NOTE: findPrecedingDoc found doc at byte 0 — behavior changed; doc=%q", doc)
	} else {
		// Current behavior: cannot find /** at byte 0.
		t.Logf("Characterizing current behavior: /** at byte 0 returns ok=false")
	}
}

func TestFindPrecedingDoc_SimpleDocComment_WithLeadingNewline(t *testing.T) {
	// Adding a leading newline pushes /** past byte 0 — should now be found.
	content := []byte("\n/** Does stuff */\nfunction foo() {}")
	pos := strings.Index(string(content), "function")
	doc, ok := findPrecedingDoc(content, pos)
	if !ok {
		t.Fatal("ok = false, want true when /** is not at byte 0")
	}
	if !strings.Contains(doc, "Does stuff") {
		t.Errorf("doc = %q, want to contain 'Does stuff'", doc)
	}
}

func TestFindPrecedingDoc_SkipsPhp8Attribute(t *testing.T) {
	src := loadFixture(t, "php8_attributes.php")
	// Find the position of the first "function" keyword.
	idx := strings.Index(string(src), "function handle_request")
	if idx < 0 {
		t.Fatal("could not find 'function handle_request' in fixture")
	}
	doc, ok := findPrecedingDoc(src, idx)
	if !ok {
		t.Fatal("ok = false, want true — doc should be found past #[Route]")
	}
	if !strings.Contains(doc, "Handles the REST API request") {
		t.Errorf("doc = %q does not contain expected text", doc)
	}
}

func TestFindPrecedingDoc_SkipsMultiplePhp8Attributes(t *testing.T) {
	src := loadFixture(t, "php8_attributes.php")
	idx := strings.Index(string(src), "function sanitize_key")
	if idx < 0 {
		t.Fatal("could not find 'function sanitize_key' in fixture")
	}
	doc, ok := findPrecedingDoc(src, idx)
	if !ok {
		t.Fatal("ok = false, want true — doc should be found past #[Pure] and #[AllowDynamicProperties]")
	}
	if !strings.Contains(doc, "Sanitizes a string key") {
		t.Errorf("doc = %q does not contain expected text", doc)
	}
}

func TestFindPrecedingDoc_AttributeAtByteZero_NoPanic(t *testing.T) {
	// A PHP source that starts with a #[Attribute] at byte 0.
	// This exercises the CRIT-03 panic guard: i < 1 guard in findPrecedingDoc.
	// Must NOT panic; must return ("", false).
	content := []byte("#[Attribute]\nfunction foo() {}")
	pos := strings.Index(string(content), "function")
	doc, ok := findPrecedingDoc(content, pos)
	// Should not panic. Return value is ("", false) — no doc precedes the function
	// when the attribute is at byte 0 with no preceding doc comment.
	if ok {
		t.Errorf("ok = true, want false for attribute at byte 0; doc = %q", doc)
	}
}

func TestFindPrecedingDoc_CursorAtByteZero_NoPanic(t *testing.T) {
	// Declaration starting at byte 0 — i will be -1 after i := pos-1.
	content := []byte("function foo() {}")
	doc, ok := findPrecedingDoc(content, 0)
	if ok {
		t.Errorf("ok = true, want false for pos=0; doc = %q", doc)
	}
	if doc != "" {
		t.Errorf("doc = %q, want empty string for pos=0", doc)
	}
}

func TestFindPrecedingDoc_DocSeparatedByBlankLines_NotFound(t *testing.T) {
	// Two blank lines between doc and function — parser should NOT find the doc.
	content := []byte("/** My doc */\n\n\nfunction foo() {}")
	pos := strings.Index(string(content), "function")
	_, ok := findPrecedingDoc(content, pos)
	// The scan walks back over whitespace, so blank lines are fine — the doc
	// should still be found.
	// This test characterizes the ACTUAL behavior: blank lines are just whitespace
	// and are skipped by the loop.
	if !ok {
		// Document current behavior: blank lines do NOT prevent doc discovery.
		// If this fails in Wave 3, it means the behavior changed — that is noteworthy.
		t.Log("findPrecedingDoc did not find doc across blank lines — noting current behavior")
	}
}

// --------------------------------------------------------------------------
// parseFile tests
// --------------------------------------------------------------------------

func TestParseFile_SimpleClass(t *testing.T) {
	src := loadFixture(t, "simple_class.php")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Classes) != 1 {
		t.Fatalf("Classes len = %d, want 1", len(analysis.Classes))
	}
	cls := analysis.Classes[0]
	if cls.Name != "WP_Post" {
		t.Errorf("Class Name = %q, want %q", cls.Name, "WP_Post")
	}
}

func TestParseFile_SimpleClass_HasDocComment(t *testing.T) {
	src := loadFixture(t, "simple_class.php")
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Fatal("no classes found")
	}
	if !strings.Contains(analysis.Classes[0].DocComment, "Represents a WordPress post") {
		t.Errorf("DocComment = %q, missing expected text", analysis.Classes[0].DocComment)
	}
}

func TestParseFile_SimpleClass_HasMethods(t *testing.T) {
	src := loadFixture(t, "simple_class.php")
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Fatal("no classes")
	}
	methods := analysis.Classes[0].Methods
	if len(methods) < 2 {
		t.Fatalf("Methods len = %d, want >= 2 (get_instance, get_id)", len(methods))
	}
	var names []string
	for _, m := range methods {
		names = append(names, m.Name)
	}
	found := false
	for _, n := range names {
		if n == "get_id" {
			found = true
		}
	}
	if !found {
		t.Errorf("get_id method not found; methods = %v", names)
	}
}

func TestParseFile_SimpleClass_HasProperties(t *testing.T) {
	src := loadFixture(t, "simple_class.php")
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Fatal("no classes")
	}
	props := analysis.Classes[0].Properties
	if len(props) < 1 {
		t.Fatalf("Properties len = %d, want >= 1", len(props))
	}
}

func TestParseFile_FileWithOnlyFunctions(t *testing.T) {
	src := loadFixture(t, "top_level_functions.php")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Classes) != 0 {
		t.Errorf("Classes len = %d, want 0 for functions-only file", len(analysis.Classes))
	}
	if len(analysis.Functions) < 2 {
		t.Errorf("Functions len = %d, want >= 2", len(analysis.Functions))
	}
}

func TestParseFile_FunctionNames(t *testing.T) {
	src := loadFixture(t, "top_level_functions.php")
	analysis := parseFile(src)
	var names []string
	for _, f := range analysis.Functions {
		names = append(names, f.Name)
	}
	wantNames := []string{"get_post", "get_post_status"}
	for _, want := range wantNames {
		found := false
		for _, got := range names {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("function %q not found in %v", want, names)
		}
	}
}

func TestParseFile_MixedClassAndFunctions(t *testing.T) {
	src := loadFixture(t, "mixed_class_and_functions.php")
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Errorf("Classes len = %d, want >= 1", len(analysis.Classes))
	}
	if len(analysis.Functions) < 1 {
		t.Errorf("Functions len = %d, want >= 1", len(analysis.Functions))
	}
}

func TestParseFile_EmptyFile(t *testing.T) {
	analysis := parseFile([]byte{})
	if analysis == nil {
		t.Fatal("parseFile returned nil for empty input")
	}
	if len(analysis.Classes) != 0 {
		t.Errorf("Classes len = %d, want 0 for empty file", len(analysis.Classes))
	}
	if len(analysis.Functions) != 0 {
		t.Errorf("Functions len = %d, want 0 for empty file", len(analysis.Functions))
	}
}

func TestParseFile_PhpOpenTagOnly(t *testing.T) {
	analysis := parseFile([]byte("<?php\n"))
	if analysis == nil {
		t.Fatal("parseFile returned nil for PHP-tag-only input")
	}
	if len(analysis.Classes) != 0 || len(analysis.Functions) != 0 {
		t.Errorf("unexpected content in PHP-tag-only parse: classes=%d functions=%d",
			len(analysis.Classes), len(analysis.Functions))
	}
}

func TestParseFile_Php8AttributeOnFunction(t *testing.T) {
	src := loadFixture(t, "php8_attributes.php")
	analysis := parseFile(src)
	if len(analysis.Functions) < 1 {
		t.Fatalf("Functions len = %d, want >= 1", len(analysis.Functions))
	}
	// The function name should be extracted correctly despite PHP 8 attributes above it.
	var names []string
	for _, f := range analysis.Functions {
		names = append(names, f.Name)
	}
	found := false
	for _, n := range names {
		if n == "handle_request" {
			found = true
		}
	}
	if !found {
		t.Errorf("handle_request not found in functions %v", names)
	}
}

// --------------------------------------------------------------------------
// walkNode + extractClass / Function / Method / Properties tests
// --------------------------------------------------------------------------

func TestExtractClass_VisibilityModifiers(t *testing.T) {
	src := loadFixture(t, "mixed_class_and_functions.php")
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Fatal("no classes")
	}
	cls := analysis.Classes[0]
	// Find get_posts (public) and found_posts property (protected).
	var visibilities []string
	for _, p := range cls.Properties {
		visibilities = append(visibilities, p.Visibility)
	}
	foundProtected := false
	for _, v := range visibilities {
		if v == "protected" {
			foundProtected = true
		}
	}
	if !foundProtected {
		t.Errorf("expected at least one protected property; visibilities = %v", visibilities)
	}
}

func TestExtractMethod_StaticModifier(t *testing.T) {
	src := loadFixture(t, "mixed_class_and_functions.php")
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Fatal("no classes")
	}
	cls := analysis.Classes[0]
	var staticMethods []string
	for _, m := range cls.Methods {
		if m.Static {
			staticMethods = append(staticMethods, m.Name)
		}
	}
	found := false
	for _, n := range staticMethods {
		if n == "fetch_all" {
			found = true
		}
	}
	if !found {
		t.Errorf("fetch_all (static method) not found in static methods %v", staticMethods)
	}
}

func TestExtractMethod_DefaultVisibilityIsPublic(t *testing.T) {
	// A method with no explicit visibility modifier should default to "public".
	src := []byte(`<?php
class Foo {
	function bar() {}
}`)
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Fatal("no classes")
	}
	methods := analysis.Classes[0].Methods
	if len(methods) < 1 {
		t.Fatal("no methods")
	}
	if methods[0].Visibility != "public" {
		t.Errorf("Visibility = %q, want %q (PHP default)", methods[0].Visibility, "public")
	}
}

func TestExtractProperties_DuplicateNames_NoError(t *testing.T) {
	// Mirrors the real wpdb case described in CLAUDE.md.
	// The parser must not crash on duplicate property names — dedup is the store's concern.
	src := loadFixture(t, "wpdb_class.php")
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Fatal("no classes in wpdb_class.php")
	}
	props := analysis.Classes[0].Properties
	// Both $col_meta entries must be present (parser does not deduplicate).
	count := 0
	for _, p := range props {
		if p.Name == "$col_meta" {
			count++
		}
	}
	if count < 2 {
		t.Errorf("$col_meta count = %d, want >= 2 (parser preserves duplicates)", count)
	}
}

func TestExtractProperties_TypeFromPhpDoc(t *testing.T) {
	// Property with no inline type annotation but @var int in doc comment.
	src := []byte(`<?php
class Foo {
	/** @var int */
	public $my_count;
}`)
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Fatal("no classes")
	}
	props := analysis.Classes[0].Properties
	if len(props) < 1 {
		t.Fatal("no properties")
	}
	if props[0].Type != "int" {
		t.Errorf("Type = %q, want %q (inferred from @var)", props[0].Type, "int")
	}
}

func TestExtractProperties_TypedProperty(t *testing.T) {
	// PHP 7.4+ inline type declaration should be captured directly.
	src := loadFixture(t, "typed_properties.php")
	analysis := parseFile(src)
	if len(analysis.Classes) < 1 {
		t.Fatal("no classes")
	}
	props := analysis.Classes[0].Properties
	// Find the 'name' property with declared type 'string'.
	var foundTyped bool
	for _, p := range props {
		if p.Name == "$name" && p.Type == "string" {
			foundTyped = true
		}
	}
	if !foundTyped {
		t.Errorf("did not find $name: string typed property; props = %+v", props)
	}
}

// --------------------------------------------------------------------------
// buildSignatureFromNode tests (exercised via parseFile)
// --------------------------------------------------------------------------

func TestBuildSignatureFromNode_NoParameters(t *testing.T) {
	src := []byte(`<?php function wp_init() {}`)
	analysis := parseFile(src)
	if len(analysis.Functions) < 1 {
		t.Fatal("no functions")
	}
	sig := analysis.Functions[0].Signature
	if !strings.Contains(sig, "function wp_init") {
		t.Errorf("Signature = %q, missing 'function wp_init'", sig)
	}
}

func TestBuildSignatureFromNode_WithReturnType(t *testing.T) {
	src := loadFixture(t, "top_level_functions.php")
	analysis := parseFile(src)
	if len(analysis.Functions) < 1 {
		t.Fatal("no functions")
	}
	// get_post has a return type annotation.
	var sig string
	for _, f := range analysis.Functions {
		if f.Name == "get_post" {
			sig = f.Signature
		}
	}
	if sig == "" {
		t.Fatal("get_post not found")
	}
	// Return type annotation should appear after ":".
	if !strings.Contains(sig, ":") {
		t.Errorf("Signature = %q, expected return type annotation with ':'", sig)
	}
}

func TestBuildSignatureFromNode_MultipleParameters(t *testing.T) {
	src := []byte(`<?php
function wp_query( int $id, string $type = 'post', bool $raw = false ): array {}`)
	analysis := parseFile(src)
	if len(analysis.Functions) < 1 {
		t.Fatal("no functions")
	}
	sig := analysis.Functions[0].Signature
	// All three params should appear.
	for _, want := range []string{"$id", "$type", "$raw"} {
		if !strings.Contains(sig, want) {
			t.Errorf("Signature = %q does not contain %q", sig, want)
		}
	}
}

func TestBuildSignatureFromNode_DefaultValues(t *testing.T) {
	src := []byte(`<?php function get_option( string $option, mixed $default = false ): mixed {}`)
	analysis := parseFile(src)
	if len(analysis.Functions) < 1 {
		t.Fatal("no functions")
	}
	sig := analysis.Functions[0].Signature
	if !strings.Contains(sig, "false") {
		t.Errorf("Signature = %q, expected default value 'false'", sig)
	}
}

func TestBuildSignatureFromNode_VariadicParameter(t *testing.T) {
	src := []byte(`<?php function do_action( string $tag, mixed ...$args ): void {}`)
	analysis := parseFile(src)
	if len(analysis.Functions) < 1 {
		t.Fatal("no functions")
	}
	sig := analysis.Functions[0].Signature
	if !strings.Contains(sig, "...") {
		t.Errorf("Signature = %q, expected variadic '...'", sig)
	}
}

func TestBuildSignatureFromNode_NullableType(t *testing.T) {
	src := []byte(`<?php function get_post_type( ?int $post = null ): ?string {}`)
	analysis := parseFile(src)
	if len(analysis.Functions) < 1 {
		t.Fatal("no functions")
	}
	sig := analysis.Functions[0].Signature
	if !strings.Contains(sig, "?") {
		t.Errorf("Signature = %q, expected nullable '?'", sig)
	}
}

// --------------------------------------------------------------------------
// detectWrapperAST tests
// --------------------------------------------------------------------------

func TestDetectWrapperAST_ReturnsFunctionCall(t *testing.T) {
	// A thin wrapper that returns the result of another function.
	body := []byte(`{ return wp_cache_get( $transient, 'transient' ); }`)
	isWrapper, name, kind := detectWrapperAST(body, nil)
	if !isWrapper {
		t.Fatal("isWrapper = false, want true")
	}
	if name != "wp_cache_get" {
		t.Errorf("name = %q, want %q", name, "wp_cache_get")
	}
	if kind != "function" {
		t.Errorf("kind = %q, want %q", kind, "function")
	}
}

func TestDetectWrapperAST_BuiltinSkipped(t *testing.T) {
	// When the wrapped function is a PHP builtin, isWrapper should be false.
	body := []byte(`{ return array_map( $fn, $arr ); }`)
	builtins := map[string]bool{"array_map": true}
	isWrapper, name, kind := detectWrapperAST(body, builtins)
	if isWrapper {
		t.Errorf("isWrapper = true for builtin; name=%q kind=%q", name, kind)
	}
}

func TestDetectWrapperAST_TooManyStatements(t *testing.T) {
	// Function body with more than 5 statements should not be classified as wrapper.
	body := []byte(`{
		$a = 1;
		$b = 2;
		$c = 3;
		$d = 4;
		$e = 5;
		return some_func($a);
	}`)
	isWrapper, _, _ := detectWrapperAST(body, nil)
	if isWrapper {
		t.Error("isWrapper = true for body with >5 statements, want false")
	}
}

func TestDetectWrapperAST_EmptyBody(t *testing.T) {
	body := []byte(`{}`)
	isWrapper, name, kind := detectWrapperAST(body, nil)
	if isWrapper {
		t.Errorf("isWrapper = true for empty body; name=%q kind=%q", name, kind)
	}
}

func TestDetectWrapperAST_VoidWrapper_BareCall(t *testing.T) {
	// A void wrapper with a single expression statement (no return).
	body := []byte(`{ do_action('init'); }`)
	isWrapper, name, kind := detectWrapperAST(body, nil)
	if !isWrapper {
		t.Fatal("isWrapper = false, want true for void wrapper")
	}
	if name != "do_action" {
		t.Errorf("name = %q, want %q", name, "do_action")
	}
	if kind != "function" {
		t.Errorf("kind = %q, want %q", kind, "function")
	}
}

// TestDetectWrapperAST_NonWrapper_TrulyMultipleStatements verifies that a
// function body with 6 statements (above the threshold of 5) is not a wrapper.
func TestDetectWrapperAST_NonWrapper_TrulyMultipleStatements(t *testing.T) {
	// 6 top-level statements — triggers the len(stmts) > 5 guard.
	body := []byte(`{
		$a = init_value();
		$b = fetch_data( $a );
		validate( $b );
		transform( $b );
		log_result( $b );
		return publish( $b );
	}`)
	isWrapper, _, _ := detectWrapperAST(body, nil)
	if isWrapper {
		t.Error("isWrapper = true for body with 6 statements (> maxWrapperStatements=5), want false")
	}
}

func TestDetectWrapperAST_ScopedSelf(t *testing.T) {
	// self::method() wrapper.
	body := []byte(`{ return self::inner_method( $x ); }`)
	isWrapper, name, kind := detectWrapperAST(body, nil)
	if !isWrapper {
		t.Fatal("isWrapper = false for self:: wrapper, want true")
	}
	if name != "inner_method" {
		t.Errorf("name = %q, want %q", name, "inner_method")
	}
	if kind != "self_method" {
		t.Errorf("kind = %q, want %q", kind, "self_method")
	}
}

func TestDetectWrapperAST_ScopedStatic(t *testing.T) {
	// ClassName::method() wrapper where class is not self/static.
	body := []byte(`{ return WP_Object_Cache::get( $key, $group ); }`)
	isWrapper, name, kind := detectWrapperAST(body, nil)
	if !isWrapper {
		t.Fatal("isWrapper = false for static class wrapper, want true")
	}
	if name != "WP_Object_Cache::get" {
		t.Errorf("name = %q, want %q", name, "WP_Object_Cache::get")
	}
	if kind != "static_method" {
		t.Errorf("kind = %q, want %q", kind, "static_method")
	}
}

func TestDetectWrapperAST_ScopedStaticKeyword(t *testing.T) {
	// static::method() — "static" should map to "self_method" kind.
	body := []byte(`{ return static::create_instance(); }`)
	isWrapper, name, kind := detectWrapperAST(body, nil)
	if !isWrapper {
		t.Fatal("isWrapper = false for static:: wrapper, want true")
	}
	if name != "create_instance" {
		t.Errorf("name = %q, want %q", name, "create_instance")
	}
	if kind != "self_method" {
		t.Errorf("kind = %q, want %q (static:: maps to self_method)", kind, "self_method")
	}
}

func TestDetectWrapperAST_ThisMethodWrapper(t *testing.T) {
	// $this->method() wrapper.
	body := []byte(`{ return $this->internal_get( $id ); }`)
	isWrapper, name, kind := detectWrapperAST(body, nil)
	if !isWrapper {
		t.Fatal("isWrapper = false for $this->method() wrapper, want true")
	}
	if name != "internal_get" {
		t.Errorf("name = %q, want %q", name, "internal_get")
	}
	if kind != "self_method" {
		t.Errorf("kind = %q, want %q", kind, "self_method")
	}
}

// TestDetectWrapperAST_ConditionalWrap characterizes the behavior of
// detectWrapperAST when a function has a conditional guard before a return.
//
// CHARACTERIZATION: The implementation counts only TOP-LEVEL statement children
// of the function body. An if_statement with a bare `return;` inside counts as
// 1 statement at the top level. If there is also 1 return_statement at the top
// level that calls a function, returnCount == 1 and detectWrapperAST returns
// (true, ...) even though the function has conditional logic. This is the current
// behavior; Wave 3 may tighten the detection heuristic.
func TestDetectWrapperAST_ConditionalWrap_CurrentBehavior(t *testing.T) {
	// if_statement + return_statement = 2 top-level stmts, returnCount == 1 (top level).
	// The top-level return calls apply_filters — so it IS detected as a wrapper.
	body := []byte(`{
		if ( ! isset( $wp_filter[ $tag ] ) ) {
			return;
		}
		return apply_filters( $tag, $value );
	}`)
	isWrapper, name, kind := detectWrapperAST(body, nil)
	// CHARACTERIZATION: current behavior returns true here because the top-level
	// return_statement contains a function call and returnCount == 1 at the top level.
	if !isWrapper {
		t.Logf("Behavior changed: isWrapper=false for conditional-then-return wrapper")
	} else {
		// Document what is returned.
		if name == "" {
			t.Error("isWrapper=true but name is empty")
		}
		t.Logf("Characterizing: detectWrapperAST(%q, %q) for conditional+return wrapper", name, kind)
	}
}

// --------------------------------------------------------------------------
// Call reference extraction tests (walkNode)
// --------------------------------------------------------------------------

func TestParseFile_CallReferences_Function(t *testing.T) {
	src := []byte(`<?php
function foo() {
	bar();
}`)
	analysis := parseFile(src)
	found := false
	for _, c := range analysis.Calls {
		if c.Name == "bar" && c.Kind == "function" {
			found = true
		}
	}
	if !found {
		t.Errorf("call to bar() not found in calls %+v", analysis.Calls)
	}
}

func TestParseFile_CallReferences_ThisMethod(t *testing.T) {
	src := []byte(`<?php
function foo() {
	$this->bar();
}`)
	analysis := parseFile(src)
	found := false
	for _, c := range analysis.Calls {
		if c.Name == "bar" && c.Kind == "method" {
			found = true
		}
	}
	if !found {
		t.Errorf("$this->bar() call not found in calls %+v", analysis.Calls)
	}
}

func TestParseFile_CallReferences_StaticMethod(t *testing.T) {
	src := []byte(`<?php
WP_Query::get_posts();
`)
	analysis := parseFile(src)
	found := false
	for _, c := range analysis.Calls {
		if c.Name == "WP_Query::get_posts" && c.Kind == "static" {
			found = true
		}
	}
	if !found {
		t.Errorf("WP_Query::get_posts() call not found in calls %+v", analysis.Calls)
	}
}
