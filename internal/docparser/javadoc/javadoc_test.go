package javadoc

import (
	"testing"
)

func TestParseBasic(t *testing.T) {
	raw := `/**
 * Returns a localized formatted string from the application's package
 * default resource bundle.
 *
 * @param key the key for the desired string
 * @param args optional arguments for the format string
 * @return the formatted string
 * @throws MissingResourceException if no resource bundle has been specified
 * @since 1.5
 * @see java.util.ResourceBundle
 */`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Returns a localized formatted string from the application's package\ndefault resource bundle." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}

	if doc.Params[0].Name != "key" {
		t.Errorf("param[0] name: %q", doc.Params[0].Name)
	}

	if doc.Params[1].Name != "args" {
		t.Errorf("param[1] name: %q", doc.Params[1].Name)
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}
	if doc.Returns.Description != "the formatted string" {
		t.Errorf("return desc: %q", doc.Returns.Description)
	}

	if len(doc.Throws) != 1 {
		t.Fatalf("expected 1 throw, got %d", len(doc.Throws))
	}
	if doc.Throws[0].Type != "MissingResourceException" {
		t.Errorf("throws type: %q", doc.Throws[0].Type)
	}

	if doc.Since != "1.5" {
		t.Errorf("since: %q", doc.Since)
	}

	if len(doc.See) != 1 || doc.See[0] != "java.util.ResourceBundle" {
		t.Errorf("see: %v", doc.See)
	}
}

func TestParseDeprecated(t *testing.T) {
	raw := `/**
 * Old method for creating threads.
 *
 * @deprecated As of JDK 1.1, replaced by {@code stop()}.
 */`

	p := New()
	doc := p.Parse(raw)

	if doc.Deprecated == "" {
		t.Error("expected deprecated tag")
	}
	if doc.Deprecated != "As of JDK 1.1, replaced by {@code stop()}." {
		t.Errorf("deprecated: %q", doc.Deprecated)
	}
}

func TestParseMultipleThrows(t *testing.T) {
	raw := `/**
 * Opens a connection to the server.
 *
 * @param host the hostname to connect to
 * @param port the port number
 * @throws IOException if an I/O error occurs
 * @throws SecurityException if a security manager denies access
 * @throws IllegalArgumentException if port is out of range
 * @return the connection handle
 */`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Throws) != 3 {
		t.Fatalf("expected 3 throws, got %d", len(doc.Throws))
	}
	if doc.Throws[0].Type != "IOException" {
		t.Errorf("throws[0]: %q", doc.Throws[0].Type)
	}
	if doc.Throws[1].Type != "SecurityException" {
		t.Errorf("throws[1]: %q", doc.Throws[1].Type)
	}
	if doc.Throws[2].Type != "IllegalArgumentException" {
		t.Errorf("throws[2]: %q", doc.Throws[2].Type)
	}
}
