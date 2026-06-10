package phpdoc

import (
	"testing"
)

func TestParseBasic(t *testing.T) {
	raw := `/**
 * Retrieves post data given a post ID or post object.
 *
 * This function retrieves the full post object from the database.
 *
 * @since 1.5.1
 *
 * @param int|WP_Post $post   Optional. Post ID or post object. Defaults to current post.
 * @param string      $output Optional. The type of output to return.
 * @return WP_Post|null Post data or null on failure.
 */`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Retrieves post data given a post ID or post object." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if doc.Since != "1.5.1" {
		t.Errorf("unexpected since: %q", doc.Since)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}

	if doc.Params[0].Name != "$post" {
		t.Errorf("param[0] name: %q", doc.Params[0].Name)
	}
	if doc.Params[0].Type != "int|WP_Post" {
		t.Errorf("param[0] type: %q", doc.Params[0].Type)
	}
	if !doc.Params[0].Optional {
		t.Error("param[0] should be optional")
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}
	if doc.Returns.Type != "WP_Post|null" {
		t.Errorf("return type: %q", doc.Returns.Type)
	}
}

func TestParseDeprecated(t *testing.T) {
	raw := `/**
 * Old function.
 *
 * @deprecated 4.5.0 Use new_function() instead.
 * @see new_function()
 */`

	p := New()
	doc := p.Parse(raw)

	if doc.Deprecated != "4.5.0 Use new_function() instead." {
		t.Errorf("unexpected deprecated: %q", doc.Deprecated)
	}
	if len(doc.See) != 1 || doc.See[0] != "new_function()" {
		t.Errorf("unexpected see: %v", doc.See)
	}
}

func TestParseThrows(t *testing.T) {
	raw := `/**
 * Processes the request.
 *
 * @param string $input The input data.
 * @throws InvalidArgumentException If input is empty.
 * @throws RuntimeException If processing fails.
 * @return bool True on success.
 */`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Throws) != 2 {
		t.Fatalf("expected 2 throws, got %d", len(doc.Throws))
	}
	if doc.Throws[0].Type != "InvalidArgumentException" {
		t.Errorf("throws[0] type: %q", doc.Throws[0].Type)
	}
	if doc.Throws[1].Type != "RuntimeException" {
		t.Errorf("throws[1] type: %q", doc.Throws[1].Type)
	}
}

func TestNonOptionalParam(t *testing.T) {
	raw := `/**
 * Do something.
 *
 * @param string $url The non-optional base URL.
 */`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(doc.Params))
	}
	if doc.Params[0].Optional {
		t.Error("param should not be optional (contains 'non-optional')")
	}
}
