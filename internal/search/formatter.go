package search

import (
	"fmt"
	"strings"
)

// Snippet holds the data needed for formatting a documentation snippet.
type Snippet struct {
	EntityName    string
	MethodName    string
	Signature     string
	Description   string
	Parameters    []SnippetParam
	ReturnType    string
	ReturnDesc    string
	SourceCode    string
	WrappedSource string
	WrappedMethod string
	URL           string
}

// SnippetParam describes a function/method parameter for formatting.
type SnippetParam struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// FormatDocSnippets renders snippets as LLM-friendly markdown text,
// stopping when the token budget is exceeded (always keeps at least 1 snippet).
func FormatDocSnippets(snippets []Snippet, tokenBudget int) string {
	if len(snippets) == 0 {
		return ""
	}

	var parts []string
	totalTokens := 0

	for i, s := range snippets {
		block := formatSnippet(s)
		tokens := ApproxTokenCount(block)

		if i > 0 && totalTokens+tokens > tokenBudget {
			break
		}

		parts = append(parts, block)
		totalTokens += tokens
	}

	return strings.Join(parts, "\n---\n\n")
}

func formatSnippet(s Snippet) string {
	var b strings.Builder

	// Title
	if s.MethodName != "" {
		fmt.Fprintf(&b, "# %s::%s()\n\n", s.EntityName, s.MethodName)
	} else {
		fmt.Fprintf(&b, "# %s\n\n", s.EntityName)
	}

	// Signature
	if s.Signature != "" {
		fmt.Fprintf(&b, "**Signature:** `%s`\n\n", s.Signature)
	}

	// Description
	if s.Description != "" {
		fmt.Fprintf(&b, "## Description\n\n%s\n\n", s.Description)
	}

	// Parameters
	if len(s.Parameters) > 0 {
		b.WriteString("## Parameters\n\n")
		for _, p := range s.Parameters {
			req := "optional"
			if p.Required {
				req = "required"
			}
			fmt.Fprintf(&b, "- `%s` (%s, %s): %s\n", p.Name, p.Type, req, p.Description)
		}
		b.WriteString("\n")
	}

	// Return
	if s.ReturnType != "" {
		if s.ReturnDesc != "" {
			fmt.Fprintf(&b, "## Return\n\n`%s` — %s\n\n", s.ReturnType, s.ReturnDesc)
		} else {
			fmt.Fprintf(&b, "## Return\n\n`%s`\n\n", s.ReturnType)
		}
	}

	// Source Code
	if s.SourceCode != "" {
		fmt.Fprintf(&b, "## Source Code\n\n```php\n%s\n```\n\n", s.SourceCode)
	}

	// Wrapped Method
	if s.WrappedMethod != "" && s.WrappedSource != "" {
		fmt.Fprintf(&b, "## Wrapped Method: %s\n\n```php\n%s\n```\n\n", s.WrappedMethod, s.WrappedSource)
	}

	// Source URL
	if s.URL != "" {
		fmt.Fprintf(&b, "Source: %s\n\n", s.URL)
	}

	return b.String()
}
