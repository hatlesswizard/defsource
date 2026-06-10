package pydoc

import (
	"strings"
	"testing"
)

func TestParseGoogleStyle(t *testing.T) {
	raw := `Fetches rows from a database table.

Retrieves rows pertaining to the given keys from the Table instance.

Args:
    keys (list[str]): Primary keys to look up.
    timeout (int, optional): Timeout in seconds.

Returns:
    list[dict]: A list of dicts mapping keys to row data.

Raises:
    IOError: An error occurred accessing the database.
    ValueError: If keys is empty.`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Fetches rows from a database table." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}

	if doc.Params[0].Name != "keys" || doc.Params[0].Type != "list[str]" {
		t.Errorf("param[0]: name=%q type=%q", doc.Params[0].Name, doc.Params[0].Type)
	}
	if doc.Params[0].Optional {
		t.Error("param[0] should not be optional")
	}

	if doc.Params[1].Name != "timeout" || !doc.Params[1].Optional {
		t.Errorf("param[1]: name=%q optional=%v", doc.Params[1].Name, doc.Params[1].Optional)
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}

	if len(doc.Throws) != 2 {
		t.Fatalf("expected 2 raises, got %d", len(doc.Throws))
	}
	if doc.Throws[0].Type != "IOError" {
		t.Errorf("throws[0] type: %q", doc.Throws[0].Type)
	}
}

func TestParseSphinxStyle(t *testing.T) {
	raw := `Send a message to a recipient.

:param str name: The name of the recipient.
:param str message: The message to send.
:type message: str
:returns: Whether the message was sent.
:rtype: bool
:raises ConnectionError: If unable to connect.`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Send a message to a recipient." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}
	if doc.Returns.Type != "bool" {
		t.Errorf("return type: %q", doc.Returns.Type)
	}

	if len(doc.Throws) != 1 {
		t.Fatalf("expected 1 raise, got %d", len(doc.Throws))
	}
	if doc.Throws[0].Type != "ConnectionError" {
		t.Errorf("throws[0] type: %q", doc.Throws[0].Type)
	}
}

func TestParseNumpyStyle(t *testing.T) {
	raw := `Compute the dot product of two vectors.

Parameters
----------
  a : ndarray
      First input vector.
  b : ndarray
      Second input vector.

Returns
----------
  result : float
      The dot product of a and b.

Raises
----------
  ValueError : If vectors have different lengths.`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Compute the dot product of two vectors." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}
	if doc.Params[0].Name != "a" || doc.Params[0].Type != "ndarray" {
		t.Errorf("param[0]: name=%q type=%q", doc.Params[0].Name, doc.Params[0].Type)
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}
	if doc.Returns.Type != "float" {
		t.Errorf("return type: %q", doc.Returns.Type)
	}

	if len(doc.Throws) != 1 {
		t.Fatalf("expected 1 raise, got %d", len(doc.Throws))
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect Format
	}{
		{"sphinx", ":param x: value", FormatSphinx},
		{"google", "Args:\n    x (int): value", FormatGoogle},
		{"numpy", "Parameters\n----------\n  x : int", FormatNumPy},
		{"plain", "Just a description.", FormatPlain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFormat(tt.input)
			if got != tt.expect {
				t.Errorf("detectFormat returned %d, want %d", got, tt.expect)
			}
		})
	}
}

func TestParsePlainDeprecated(t *testing.T) {
	raw := `Deprecated: Use new_function() instead.

This function will be removed in a future version.`

	p := New()
	doc := p.Parse(raw)

	if !strings.Contains(doc.Deprecated, "Use new_function() instead.") {
		t.Errorf("unexpected deprecated: %q", doc.Deprecated)
	}
}
