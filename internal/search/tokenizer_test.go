//go:build sqlite_fts5 || fts5

package search

import (
	"testing"
)

// TestApproxTokenCount_Empty verifies that an empty string returns 0 tokens.
func TestApproxTokenCount_Empty(t *testing.T) {
	if got := ApproxTokenCount(""); got != 0 {
		t.Errorf("ApproxTokenCount(\"\") = %d, want 0", got)
	}
}

// TestApproxTokenCount_PlainWords verifies that word counts are scaled by 1.5.
// The function multiplies whitespace-delimited word count by 1.5, truncating
// to int. For N words the result must equal int(float64(N) * 1.5).
func TestApproxTokenCount_PlainWords(t *testing.T) {
	cases := []struct {
		name  string
		input string
		words int // whitespace-delimited word count
	}{
		{"one word", "hello", 1},
		{"two words", "hello world", 2},
		{"four words", "the quick brown fox", 4},
		{"ten words", "one two three four five six seven eight nine ten", 10},
		{"only whitespace between words", "  foo   bar  baz  ", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := int(float64(tc.words) * 1.5)
			got := ApproxTokenCount(tc.input)
			if got != want {
				t.Errorf("ApproxTokenCount(%q) = %d, want %d (words=%d × 1.5)",
					tc.input, got, want, tc.words)
			}
		})
	}
}

// TestApproxTokenCount_CodeHeavy verifies that PHP-style signature text
// produces a token count proportional to the word count as documented (× 1.5).
// The approximation is not calibrated to any specific LLM tokeniser; the
// test confirms the contract stated in the function doc comment.
func TestApproxTokenCount_CodeHeavy(t *testing.T) {
	// A realistic PHP method signature with type annotations.
	// strings.Fields splits on any whitespace run; the tokens below are:
	// "public", "function", "get_results(", "string", "$query,", "array", "$args", "=", "array()", ")"
	// That is 10 tokens (strings.Fields skips empty). Verify the 1.5 multiplier.
	sig := "public function get_results( string $query, array $args = array() )"
	words := 10 // pre-counted via strings.Fields
	want := int(float64(words) * 1.5)
	got := ApproxTokenCount(sig)
	if got != want {
		t.Errorf("ApproxTokenCount(phpSig) = %d, want %d (words=%d, ratio=1.5)",
			got, want, words)
	}

	// A multi-line source snippet. Verify that newlines do not break the
	// word-count heuristic (strings.Fields treats all whitespace as delimiters).
	snippet := "function wp_query( $args ) {\n\treturn new WP_Query( $args );\n}"
	// Fields: "function", "wp_query(", "$args", ")", "{", "return", "new", "WP_Query(", "$args", ");", "}"
	// = 11 words
	snippetWordCount := 11
	wantSnippet := int(float64(snippetWordCount) * 1.5)
	gotSnippet := ApproxTokenCount(snippet)
	if gotSnippet != wantSnippet {
		t.Errorf("ApproxTokenCount(phpSnippet) = %d, want %d", gotSnippet, wantSnippet)
	}
}
