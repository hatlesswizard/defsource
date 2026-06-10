package xmldoc

import (
	"testing"
)

func TestParseBasic(t *testing.T) {
	raw := `/// <summary>
/// Sends an HTTP request and returns an HTTP response.
/// </summary>
/// <param name="request">The HTTP request message to send.</param>
/// <param name="cancellationToken">The cancellation token to cancel operation.</param>
/// <returns>The task object representing the asynchronous operation.</returns>
/// <exception cref="ArgumentNullException">The request is null.</exception>
/// <exception cref="HttpRequestException">The request failed due to network error.</exception>`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Sends an HTTP request and returns an HTTP response." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(doc.Params))
	}
	if doc.Params[0].Name != "request" {
		t.Errorf("param[0] name: %q", doc.Params[0].Name)
	}
	if doc.Params[1].Name != "cancellationToken" {
		t.Errorf("param[1] name: %q", doc.Params[1].Name)
	}

	if doc.Returns == nil {
		t.Fatal("expected return doc")
	}
	if doc.Returns.Description != "The task object representing the asynchronous operation." {
		t.Errorf("return desc: %q", doc.Returns.Description)
	}

	if len(doc.Throws) != 2 {
		t.Fatalf("expected 2 exceptions, got %d", len(doc.Throws))
	}
	if doc.Throws[0].Type != "ArgumentNullException" {
		t.Errorf("throws[0] type: %q", doc.Throws[0].Type)
	}
}

func TestParseWithRemarks(t *testing.T) {
	raw := `/// <summary>
/// Initializes a new instance of the HttpClient class.
/// </summary>
/// <remarks>
/// The default instance uses the default handler and disposes
/// the handler when this instance is disposed.
/// </remarks>`

	p := New()
	doc := p.Parse(raw)

	if doc.Summary != "Initializes a new instance of the HttpClient class." {
		t.Errorf("unexpected summary: %q", doc.Summary)
	}

	if doc.Description == doc.Summary {
		t.Error("expected description to include remarks")
	}
}

func TestParseWithSeeRef(t *testing.T) {
	raw := `/// <summary>
/// Creates a new scope for dependency injection.
/// </summary>
/// <param name="provider">The <see cref="IServiceProvider"/> to create scope from.</param>
/// <returns>The new <see cref="IServiceScope"/>.</returns>
/// <see cref="IServiceProvider"/>
/// <see cref="IServiceScope"/>`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(doc.Params))
	}
	// The param description should have the cref resolved inline
	if doc.Params[0].Description != "The IServiceProvider to create scope from." {
		t.Errorf("param desc: %q", doc.Params[0].Description)
	}

	if len(doc.See) < 2 {
		t.Errorf("expected at least 2 see refs, got %d", len(doc.See))
	}
}

func TestParseWithExample(t *testing.T) {
	raw := `/// <summary>
/// Reads content from a stream.
/// </summary>
/// <example>
/// var content = await stream.ReadAsStringAsync();
/// Console.WriteLine(content);
/// </example>`

	p := New()
	doc := p.Parse(raw)

	if len(doc.Examples) != 1 {
		t.Fatalf("expected 1 example, got %d", len(doc.Examples))
	}
	if doc.Examples[0] == "" {
		t.Error("example should not be empty")
	}
}
