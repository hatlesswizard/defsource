package godoc

import (
	"testing"
)

func TestParseBasic(t *testing.T) {
	raw := `// ListenAndServe listens on the TCP network address addr and then calls
// Serve with handler to handle requests on incoming connections.
// Accepted connections are configured to enable TCP keep-alives.
//
// The handler is typically nil, in which case the DefaultServeMux is used.
//
// ListenAndServe always returns a non-nil error.`

	p := New()
	doc := p.Parse(raw)

	expected := "ListenAndServe listens on the TCP network address addr and then calls\nServe with handler to handle requests on incoming connections.\nAccepted connections are configured to enable TCP keep-alives."
	if doc.Summary != expected {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if doc.Deprecated != "" {
		t.Errorf("unexpected deprecated: %q", doc.Deprecated)
	}
}

func TestParseDeprecated(t *testing.T) {
	raw := `// Title returns a copy of the string s with all Unicode letters that begin
// words mapped to their title case.
//
// Deprecated: The rule Title uses for word boundaries does not handle Unicode
// punctuation properly. Use golang.org/x/text/cases instead.`

	p := New()
	doc := p.Parse(raw)

	if doc.Deprecated == "" {
		t.Error("expected deprecated notice")
	}
	if doc.Deprecated != "The rule Title uses for word boundaries does not handle Unicode" {
		t.Errorf("deprecated: %q", doc.Deprecated)
	}
}

func TestParseSince(t *testing.T) {
	raw := `// Clear removes all entries from the map, added in Go 1.21.`

	p := New()
	doc := p.Parse(raw)

	if doc.Since != "1.21" {
		t.Errorf("since: %q", doc.Since)
	}
}

func TestParseBlockComment(t *testing.T) {
	raw := `/* Package http provides HTTP client and server implementations.

Get, Head, Post, and PostForm make HTTP (or HTTPS) requests.

The caller must close the response body when finished with it. */`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Package http provides HTTP client and server implementations." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}
}

func TestParseEmpty(t *testing.T) {
	p := New()
	doc := p.Parse("")

	if doc.Summary != "" {
		t.Errorf("expected empty summary, got: %q", doc.Summary)
	}
}
