package search

import (
	"fmt"
	"strings"

	"github.com/hatlesswizard/defsource/internal/source"
)

// FormatDocSnippets renders snippets as LLM-friendly Markdown text and joins
// them with "---" separators. It respects tokenBudget by stopping before
// appending a snippet that would push the running token total over the limit.
//
// Token-budget contract:
//   - The first snippet is always included regardless of its size. A caller
//     that passes tokenBudget=0 (or any value smaller than the first snippet's
//     token count) will still receive one snippet, never an empty string.
//     This at-least-1 guarantee exists so callers always have something
//     meaningful to return even when the budget is very tight.
//   - Subsequent snippets are added only when totalTokens+snippetTokens <=
//     tokenBudget. Once the budget is exhausted the remaining snippets are
//     silently dropped.
//   - An empty snippets slice returns "".
//
// Token counts are estimated by ApproxTokenCount (a 1.5× word-count heuristic).
func FormatDocSnippets(snippets []source.DocSnippet, tokenBudget int) string {
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

// formatSnippet renders a single DocSnippet as a Markdown document section.
// Sections are emitted only when the corresponding field is non-empty, so
// callers may safely pass partially-populated DocSnippets.
func formatSnippet(s source.DocSnippet) string {
	var b strings.Builder

	if s.MethodName != "" {
		fmt.Fprintf(&b, "# %s::%s()\n\n", s.EntityName, s.MethodName)
	} else {
		fmt.Fprintf(&b, "# %s\n\n", s.EntityName)
	}

	if s.Signature != "" {
		fmt.Fprintf(&b, "**Signature:** `%s`\n\n", s.Signature)
	}

	if s.Description != "" {
		fmt.Fprintf(&b, "## Description\n\n%s\n\n", s.Description)
	}

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

	if s.ReturnType != "" {
		if s.ReturnDesc != "" {
			fmt.Fprintf(&b, "## Return\n\n`%s` — %s\n\n", s.ReturnType, s.ReturnDesc)
		} else {
			fmt.Fprintf(&b, "## Return\n\n`%s`\n\n", s.ReturnType)
		}
	}

	if s.SourceCode != "" {
		fmt.Fprintf(&b, "## Source Code\n\n```php\n%s\n```\n\n", s.SourceCode)
	}

	if s.WrappedMethod != "" && s.WrappedSource != "" {
		fmt.Fprintf(&b, "## Wrapped Method: %s\n\n```php\n%s\n```\n\n", s.WrappedMethod, s.WrappedSource)
	}

	if s.URL != "" {
		fmt.Fprintf(&b, "Source: %s\n\n", s.URL)
	}

	return b.String()
}
