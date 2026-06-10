package doxygen

import (
	"testing"
)

func TestParseBlockComment(t *testing.T) {
	raw := `/**
 * \brief Opens a file for reading or writing.
 *
 * This function opens the specified file and returns a handle.
 *
 * \param filename The path to the file to open.
 * \param mode The mode string ("r", "w", "a", etc).
 * \return A file handle on success, or NULL on failure.
 * \throws std::runtime_error if the file cannot be opened.
 * \since 2.0
 * \see fclose
 */`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Opens a file for reading or writing." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}
	if doc.Params[0].Name != "filename" {
		t.Errorf("param[0] name: %q", doc.Params[0].Name)
	}
	if doc.Params[1].Name != "mode" {
		t.Errorf("param[1] name: %q", doc.Params[1].Name)
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}

	if len(doc.Throws) != 1 {
		t.Fatalf("expected 1 throw, got %d", len(doc.Throws))
	}
	if doc.Throws[0].Type != "std::runtime_error" {
		t.Errorf("throws type: %q", doc.Throws[0].Type)
	}

	if doc.Since != "2.0" {
		t.Errorf("since: %q", doc.Since)
	}

	if len(doc.See) != 1 || doc.See[0] != "fclose" {
		t.Errorf("see: %v", doc.See)
	}
}

func TestParseAtStyle(t *testing.T) {
	raw := `/// @brief Allocates memory for an array.
/// @param count Number of elements.
/// @param size Size of each element in bytes.
/// @return Pointer to allocated memory.
/// @throws std::bad_alloc if allocation fails.`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Allocates memory for an array." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}
}

func TestParseParamDirection(t *testing.T) {
	raw := `/**
 * Copies data between buffers.
 *
 * \param[out] dest The destination buffer.
 * \param[in] src The source buffer.
 * \param[in] count Number of bytes to copy.
 * \return Number of bytes copied.
 */`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Params) != 3 {
		t.Fatalf("expected 3 params, got %d", len(doc.Params))
	}
	if doc.Params[0].Name != "dest" {
		t.Errorf("param[0] name: %q", doc.Params[0].Name)
	}
	if doc.Params[1].Name != "src" {
		t.Errorf("param[1] name: %q", doc.Params[1].Name)
	}
}

func TestParseTparam(t *testing.T) {
	raw := `/**
 * \brief A generic container.
 *
 * \tparam T The element type stored in the container.
 * \tparam Alloc The allocator type.
 */`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 tparams, got %d", len(doc.Params))
	}
	if doc.Params[0].Name != "T" || doc.Params[0].Type != "template" {
		t.Errorf("tparam[0]: name=%q type=%q", doc.Params[0].Name, doc.Params[0].Type)
	}
}

func TestParseDeprecated(t *testing.T) {
	raw := `/**
 * Old API function.
 *
 * \deprecated Use new_api() instead.
 */`

	p := New()
	doc := p.Parse(raw)

	if doc.Deprecated != "Use new_api() instead." {
		t.Errorf("deprecated: %q", doc.Deprecated)
	}
}
