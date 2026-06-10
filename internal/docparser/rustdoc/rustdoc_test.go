package rustdoc

import (
	"strings"
	"testing"
)

func TestParseBasic(t *testing.T) {
	raw := `/// Creates a new runtime with the default configuration.
///
/// This is a convenience function that creates a multi-threaded runtime
/// with all features enabled.
///
/// # Arguments
///
/// * ` + "`worker_threads`" + ` - Number of worker threads
/// * ` + "`name`" + ` - Name prefix for worker threads
///
/// # Returns
///
/// A new Runtime instance configured with the specified settings.
///
/// # Errors
///
/// Returns an error if the runtime cannot be created.
///
/// # Examples
///
/// ` + "```" + `
/// let rt = Runtime::new(4, "my-app").unwrap();
/// ` + "```"

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Creates a new runtime with the default configuration." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}
	if doc.Params[0].Name != "worker_threads" {
		t.Errorf("param[0] name: %q", doc.Params[0].Name)
	}
	if doc.Params[1].Name != "name" {
		t.Errorf("param[1] name: %q", doc.Params[1].Name)
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}
	if !strings.Contains(doc.Returns.Description, "Runtime instance") {
		t.Errorf("return desc: %q", doc.Returns.Description)
	}

	if len(doc.Throws) < 1 {
		t.Fatal("expected at least 1 error entry")
	}

	if len(doc.Examples) != 1 {
		t.Fatalf("expected 1 example, got %d", len(doc.Examples))
	}
}

func TestParsePanics(t *testing.T) {
	raw := `/// Divides two numbers.
///
/// # Panics
///
/// Panics if the divisor is zero.`

	p := New()
	doc := p.Parse(raw)

	found := false
	for _, th := range doc.Throws {
		if th.Type == "panic" {
			found = true
			if !strings.Contains(th.Description, "divisor is zero") {
				t.Errorf("panic desc: %q", th.Description)
			}
		}
	}
	if !found {
		t.Error("expected panic entry in throws")
	}
}

func TestParseSafety(t *testing.T) {
	raw := `/// Dereferences a raw pointer.
///
/// # Safety
///
/// The pointer must be valid and properly aligned.`

	p := New()
	doc := p.Parse(raw)

	if !strings.Contains(doc.Description, "Safety:") {
		t.Errorf("expected safety info in description: %q", doc.Description)
	}
}

func TestParseModuleDoc(t *testing.T) {
	raw := `//! This module provides async I/O primitives.
//!
//! It wraps the platform-specific event loop.`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "This module provides async I/O primitives." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}
}

func TestParseDeprecated(t *testing.T) {
	raw := `/// Old function.
///
/// Deprecated since 1.5.0: Use new_function instead.`

	p := New()
	doc := p.Parse(raw)

	if doc.Deprecated != "1.5.0" {
		t.Errorf("deprecated: %q", doc.Deprecated)
	}
}
