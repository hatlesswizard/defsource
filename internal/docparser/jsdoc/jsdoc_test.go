package jsdoc

import (
	"testing"
)

func TestParseBasic(t *testing.T) {
	raw := `/**
 * Creates a new element with the given tag name.
 *
 * @param {string} tagName - The HTML tag name.
 * @param {Object} [options] - Optional configuration.
 * @param {string} [options.className=""] - CSS class name.
 * @returns {HTMLElement} The created element.
 */`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Creates a new element with the given tag name." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 3 {
		t.Fatalf("expected 3 params, got %d", len(doc.Params))
	}

	if doc.Params[0].Name != "tagName" || doc.Params[0].Type != "string" {
		t.Errorf("param[0]: name=%q type=%q", doc.Params[0].Name, doc.Params[0].Type)
	}
	if doc.Params[0].Optional {
		t.Error("param[0] should not be optional")
	}

	if doc.Params[1].Name != "options" || !doc.Params[1].Optional {
		t.Errorf("param[1]: name=%q optional=%v", doc.Params[1].Name, doc.Params[1].Optional)
	}

	if doc.Params[2].Default != "\"\"" {
		t.Errorf("param[2] default: %q", doc.Params[2].Default)
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}
	if doc.Returns.Type != "HTMLElement" {
		t.Errorf("return type: %q", doc.Returns.Type)
	}
}

func TestParseThrows(t *testing.T) {
	raw := `/**
 * Validates the input data.
 *
 * @param {Object} data - The data to validate.
 * @throws {TypeError} If data is not an object.
 * @throws {RangeError} If data values are out of range.
 * @returns {boolean} True if valid.
 */`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Throws) != 2 {
		t.Fatalf("expected 2 throws, got %d", len(doc.Throws))
	}
	if doc.Throws[0].Type != "TypeError" {
		t.Errorf("throws[0] type: %q", doc.Throws[0].Type)
	}
	if doc.Throws[1].Type != "RangeError" {
		t.Errorf("throws[1] type: %q", doc.Throws[1].Type)
	}
}

func TestParseExample(t *testing.T) {
	raw := `/**
 * Adds two numbers.
 *
 * @param {number} a - First number.
 * @param {number} b - Second number.
 * @returns {number} The sum.
 * @example
 * const result = add(1, 2);
 * console.log(result); // 3
 */`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Examples) != 1 {
		t.Fatalf("expected 1 example, got %d", len(doc.Examples))
	}
	if !contains(doc.Examples[0], "const result = add(1, 2)") {
		t.Errorf("unexpected example: %q", doc.Examples[0])
	}
}

func TestParseDeprecated(t *testing.T) {
	raw := `/**
 * Old method.
 *
 * @deprecated Use newMethod() instead.
 * @since 1.0.0
 * @see newMethod
 */`

	p := New()
	doc := p.Parse(raw)

	if doc.Deprecated != "Use newMethod() instead." {
		t.Errorf("unexpected deprecated: %q", doc.Deprecated)
	}
	if doc.Since != "1.0.0" {
		t.Errorf("unexpected since: %q", doc.Since)
	}
	if len(doc.See) != 1 || doc.See[0] != "newMethod" {
		t.Errorf("unexpected see: %v", doc.See)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
