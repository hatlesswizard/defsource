//go:build sqlite_fts5 || fts5

package search

import (
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
)

// TestFormatDocSnippets_EmptyInput verifies that an empty snippet slice
// returns an empty string, not a separator or header line.
func TestFormatDocSnippets_EmptyInput(t *testing.T) {
	got := FormatDocSnippets([]source.DocSnippet{}, 8000)
	if got != "" {
		t.Errorf("FormatDocSnippets(empty) = %q, want \"\"", got)
	}
}

// TestFormatDocSnippets_SingleSnippetWithinBudget verifies that a single
// snippet whose token count fits within the budget is fully rendered.
func TestFormatDocSnippets_SingleSnippetWithinBudget(t *testing.T) {
	s := source.DocSnippet{
		EntityName:  "WP_Query",
		Description: "Retrieves query results.",
	}
	got := FormatDocSnippets([]source.DocSnippet{s}, 8000)
	if !strings.Contains(got, "WP_Query") {
		t.Errorf("FormatDocSnippets output missing entity name: %q", got)
	}
	if !strings.Contains(got, "Retrieves query results.") {
		t.Errorf("FormatDocSnippets output missing description: %q", got)
	}
	// A single snippet must never contain a separator.
	if strings.Contains(got, "---") {
		t.Errorf("FormatDocSnippets single snippet should have no separator, got: %q", got)
	}
}

// TestFormatDocSnippets_AtLeastOneInvariant is the critical regression test
// for F-CRITICAL-004 (ch09-unit-tests.md). The first snippet must always be
// included even when tokenBudget=0, which is smaller than any non-empty
// snippet's token count.
//
// The guard in formatter.go:
//
//	if i > 0 && totalTokens+tokens > tokenBudget { break }
//
// ensures the first snippet (i==0) is never subject to the budget check.
// A refactor that changes "i > 0" to "i >= 0" would silently break this
// guarantee and cause this test to fail.
func TestFormatDocSnippets_AtLeastOneInvariant(t *testing.T) {
	s := source.DocSnippet{
		EntityName:  "WP_Query",
		Description: "Detailed description that will have many words and therefore many tokens.",
	}
	// tokenBudget=0 is smaller than any non-empty snippet's token count.
	got := FormatDocSnippets([]source.DocSnippet{s}, 0)
	if got == "" {
		t.Fatal("FormatDocSnippets(tokenBudget=0) returned \"\", violates at-least-1 guarantee")
	}
	if !strings.Contains(got, "WP_Query") {
		t.Errorf("FormatDocSnippets(tokenBudget=0) missing entity name, got: %q", got)
	}
}

// TestFormatDocSnippets_BudgetCutsOffMidList verifies that when a budget is
// set, snippets that would push the running total over the limit are dropped,
// while those that fit (starting from the first) are included.
func TestFormatDocSnippets_BudgetCutsOffMidList(t *testing.T) {
	// Each snippet has a short description — roughly 3-4 words, ≈5 tokens each
	// after the 1.5× multiplier plus the header words.
	small := func(name string) source.DocSnippet {
		return source.DocSnippet{EntityName: name, Description: "Short description here."}
	}

	snippets := []source.DocSnippet{
		small("Alpha"),
		small("Beta"),
		small("Gamma"),
		small("Delta"),
		small("Epsilon"),
	}

	// Compute the exact token count of the first rendered snippet so we can
	// set a budget that allows exactly 2 snippets through.
	firstBlock := formatSnippet(snippets[0])
	firstTokens := ApproxTokenCount(firstBlock)
	secondBlock := formatSnippet(snippets[1])
	secondTokens := ApproxTokenCount(secondBlock)
	thirdBlock := formatSnippet(snippets[2])
	thirdTokens := ApproxTokenCount(thirdBlock)

	// Budget that fits the first two snippets but not the third.
	budget := firstTokens + secondTokens + thirdTokens - 1

	got := FormatDocSnippets(snippets, budget)

	if !strings.Contains(got, "Alpha") {
		t.Error("FormatDocSnippets: expected Alpha (first snippet always included)")
	}
	if !strings.Contains(got, "Beta") {
		t.Errorf("FormatDocSnippets: expected Beta within budget (budget=%d, first=%d, second=%d)",
			budget, firstTokens, secondTokens)
	}
	// Gamma's tokens would push total over the budget; it must be absent.
	if strings.Contains(got, "Gamma") {
		t.Errorf("FormatDocSnippets: Gamma should have been cut off by budget (budget=%d, first=%d, second=%d, third=%d)",
			budget, firstTokens, secondTokens, thirdTokens)
	}
}

// TestFormatDocSnippets_AllOptionalFieldsPopulated verifies that when all
// optional fields are present the output contains each expected section in
// the correct markdown order: Signature, Description, Parameters, Return,
// Source Code, Wrapped Method, Source URL.
func TestFormatDocSnippets_AllOptionalFieldsPopulated(t *testing.T) {
	s := source.DocSnippet{
		EntityName:  "WP_Query",
		MethodName:  "get_posts",
		Signature:   "public function get_posts(): array",
		Description: "Returns an array of post objects.",
		Parameters: []source.Parameter{
			{Name: "$args", Type: "array", Required: true, Description: "Query arguments."},
			{Name: "$limit", Type: "int", Required: false, Description: "Maximum results."},
		},
		ReturnType:    "array",
		ReturnDesc:    "Array of WP_Post objects.",
		SourceCode:    "return $this->posts;",
		WrappedSource: "return wp_query_posts($args);",
		WrappedMethod: "wp_query_posts",
		URL:           "https://github.com/WordPress/WordPress/blob/trunk/wp-includes/class-wp-query.php#L1234",
	}

	got := FormatDocSnippets([]source.DocSnippet{s}, 8000)

	checks := []struct {
		name    string
		pattern string
	}{
		{"method title", "# WP_Query::get_posts()"},
		{"signature", "**Signature:**"},
		{"description header", "## Description"},
		{"description body", "Returns an array of post objects."},
		{"parameters header", "## Parameters"},
		{"required param", "required"},
		{"optional param", "optional"},
		{"return header", "## Return"},
		{"return type", "`array`"},
		{"return desc", "Array of WP_Post objects."},
		{"source code header", "## Source Code"},
		{"source code body", "return $this->posts;"},
		{"wrapped method header", "## Wrapped Method: wp_query_posts"},
		{"wrapped source body", "return wp_query_posts($args);"},
		{"url", "https://github.com/WordPress"},
	}
	for _, c := range checks {
		if !strings.Contains(got, c.pattern) {
			t.Errorf("TestFormatDocSnippets_AllOptionalFieldsPopulated: output missing %s (%q)\nOutput:\n%s",
				c.name, c.pattern, got)
		}
	}

	// Verify ordering: Signature appears before Description, Description before
	// Parameters, Parameters before Return, Return before Source Code.
	sigIdx := strings.Index(got, "**Signature:**")
	descIdx := strings.Index(got, "## Description")
	paramIdx := strings.Index(got, "## Parameters")
	retIdx := strings.Index(got, "## Return")
	srcIdx := strings.Index(got, "## Source Code")
	wrapIdx := strings.Index(got, "## Wrapped Method:")
	urlIdx := strings.Index(got, "Source:")

	if !(sigIdx < descIdx && descIdx < paramIdx && paramIdx < retIdx &&
		retIdx < srcIdx && srcIdx < wrapIdx && wrapIdx < urlIdx) {
		t.Errorf("Section ordering is wrong.\n  Signature@%d, Description@%d, Parameters@%d, Return@%d, SourceCode@%d, WrappedMethod@%d, URL@%d",
			sigIdx, descIdx, paramIdx, retIdx, srcIdx, wrapIdx, urlIdx)
	}
}

// TestFormatDocSnippets_MissingOptionalFieldsOmitted verifies that sections
// for empty optional fields are not rendered — no stray headers or blank lines
// caused by missing Signature, Description, Parameters, ReturnType,
// SourceCode, WrappedMethod, or URL.
func TestFormatDocSnippets_MissingOptionalFieldsOmitted(t *testing.T) {
	// Only EntityName is populated; all optional fields are zero-value.
	s := source.DocSnippet{
		EntityName: "WP_Hook",
	}

	got := FormatDocSnippets([]source.DocSnippet{s}, 8000)

	// The entity name header must still appear.
	if !strings.Contains(got, "# WP_Hook") {
		t.Errorf("FormatDocSnippets: entity name header missing, got: %q", got)
	}

	// None of the optional section headers should appear.
	forbidden := []string{
		"**Signature:**",
		"## Description",
		"## Parameters",
		"## Return",
		"## Source Code",
		"## Wrapped Method",
		"Source:",
	}
	for _, f := range forbidden {
		if strings.Contains(got, f) {
			t.Errorf("FormatDocSnippets: unexpected section %q in output for empty snippet, got:\n%s", f, got)
		}
	}
}

// TestFormatDocSnippets_MethodNameTitleFormat verifies that when MethodName is
// set, the title is rendered as "EntityName::MethodName()" rather than just
// "EntityName". When MethodName is empty, the title is just "EntityName".
func TestFormatDocSnippets_MethodNameTitleFormat(t *testing.T) {
	withMethod := source.DocSnippet{EntityName: "WP_Query", MethodName: "get_posts"}
	withoutMethod := source.DocSnippet{EntityName: "WP_Query"}

	gotWith := formatSnippet(withMethod)
	gotWithout := formatSnippet(withoutMethod)

	if !strings.Contains(gotWith, "# WP_Query::get_posts()") {
		t.Errorf("formatSnippet with MethodName: title not formatted as Entity::Method(), got:\n%s", gotWith)
	}
	if strings.Contains(gotWith, "# WP_Query\n") {
		t.Errorf("formatSnippet with MethodName: plain entity title unexpectedly present: %q", gotWith)
	}

	if !strings.Contains(gotWithout, "# WP_Query\n") {
		t.Errorf("formatSnippet without MethodName: title not formatted as plain Entity, got:\n%s", gotWithout)
	}
	if strings.Contains(gotWithout, "::") {
		t.Errorf("formatSnippet without MethodName: unexpected '::' in output: %q", gotWithout)
	}
}

// TestFormatDocSnippets_MultipleSeparatedBySeparator verifies that when two
// snippets both fit within the budget they are joined with the "\n---\n\n"
// separator and the separator appears exactly once.
func TestFormatDocSnippets_MultipleSeparatedBySeparator(t *testing.T) {
	snippets := []source.DocSnippet{
		{EntityName: "Alpha", Description: "First."},
		{EntityName: "Beta", Description: "Second."},
	}
	got := FormatDocSnippets(snippets, 8000)

	// Both entities must appear.
	if !strings.Contains(got, "Alpha") {
		t.Error("FormatDocSnippets: Alpha missing from output")
	}
	if !strings.Contains(got, "Beta") {
		t.Error("FormatDocSnippets: Beta missing from output")
	}

	// The separator must appear exactly once between the two snippets.
	count := strings.Count(got, "\n---\n\n")
	if count != 1 {
		t.Errorf("FormatDocSnippets: expected 1 separator, got %d in output:\n%s", count, got)
	}
}
