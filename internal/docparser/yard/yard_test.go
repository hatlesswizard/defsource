package yard

import (
	"testing"
)

func TestParseBasic(t *testing.T) {
	raw := `# Converts the object to its JSON representation.
#
# This method serializes the object attributes into a JSON string.
#
# @param [Hash] opts the options to create the message with
# @option opts [Boolean] :pretty whether to pretty-print
# @return [String] the JSON representation
# @raise [EncodingError] if object cannot be encoded
# @since 1.2.0
# @see #from_json`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Converts the object to its JSON representation." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 2 { // param + option
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}
	if doc.Params[0].Name != "opts" || doc.Params[0].Type != "Hash" {
		t.Errorf("param[0]: name=%q type=%q", doc.Params[0].Name, doc.Params[0].Type)
	}
	if doc.Params[1].Name != "opts.pretty" || !doc.Params[1].Optional {
		t.Errorf("param[1]: name=%q optional=%v", doc.Params[1].Name, doc.Params[1].Optional)
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}
	if doc.Returns.Type != "String" {
		t.Errorf("return type: %q", doc.Returns.Type)
	}

	if len(doc.Throws) != 1 || doc.Throws[0].Type != "EncodingError" {
		t.Errorf("throws: %v", doc.Throws)
	}

	if doc.Since != "1.2.0" {
		t.Errorf("since: %q", doc.Since)
	}

	if len(doc.See) != 1 || doc.See[0] != "#from_json" {
		t.Errorf("see: %v", doc.See)
	}
}

func TestParseDeprecated(t *testing.T) {
	raw := `# Old method.
#
# @deprecated Use {#new_method} instead.`

	p := New()
	doc := p.Parse(raw)

	if doc.Deprecated != "Use {#new_method} instead." {
		t.Errorf("deprecated: %q", doc.Deprecated)
	}
}

func TestParseExample(t *testing.T) {
	raw := `# Creates a new connection.
#
# @param [String] host the hostname
# @example Basic usage
#   conn = Connection.new("localhost")
#   conn.connect`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Examples) != 1 {
		t.Fatalf("expected 1 example, got %d", len(doc.Examples))
	}
	if doc.Examples[0] == "" {
		t.Error("example should not be empty")
	}
}

func TestParseParamNameFirst(t *testing.T) {
	raw := `# Sets the value.
#
# @param name [String] the attribute name
# @return [Object] the set value`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(doc.Params))
	}
	if doc.Params[0].Name != "name" || doc.Params[0].Type != "String" {
		t.Errorf("param: name=%q type=%q", doc.Params[0].Name, doc.Params[0].Type)
	}
}
