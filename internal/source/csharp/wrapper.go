package csharp

import (
	"regexp"
	"strings"
)

// Wrapper detection patterns for C# delegation patterns.
//
// Patterns detected:
// 1. Direct delegation: return _inner.Method(args);
// 2. Extension method wrapping: return obj.Method(args);
// 3. Interface forwarding: return _implementation.Method(args);
// 4. Decorator pattern: base.Method(args);
// 5. Static method delegation: return ClassName.Method(args);

var (
	// Matches: return _field.Method(args...);
	reDelegation = regexp.MustCompile(
		`(?m)^\s*(?:return\s+)?(?:_\w+|this\.\w+)\s*\.\s*(\w+)\s*\(([^)]*)\)\s*;?\s*$`)

	// Matches: return ClassName.StaticMethod(args...);
	reStaticDelegation = regexp.MustCompile(
		`(?m)^\s*return\s+([A-Z]\w+)\s*\.\s*(\w+)\s*\(([^)]*)\)\s*;?\s*$`)

	// Matches: return base.Method(args...);
	reBaseDelegation = regexp.MustCompile(
		`(?m)^\s*return\s+base\s*\.\s*(\w+)\s*\(([^)]*)\)\s*;?\s*$`)

	// Matches expression-bodied: => _field.Method(args);
	reExpressionDelegation = regexp.MustCompile(
		`(?m)=>\s*(?:_\w+|this\.\w+)\s*\.\s*(\w+)\s*\(([^)]*)\)\s*;?\s*$`)

	// Matches expression-bodied static: => ClassName.Method(args);
	reExpressionStaticDelegation = regexp.MustCompile(
		`(?m)=>\s*([A-Z]\w+)\s*\.\s*(\w+)\s*\(([^)]*)\)\s*;?\s*$`)
)

// detectWrapper analyzes method source code for delegation patterns.
// Returns (isWrapper, targetName, targetKind).
func detectWrapper(sourceCode string, idx *codebaseIndex) (bool, string, string) {
	// Strip XML doc comments and blank lines to get the method body
	body := extractMethodBody(sourceCode)
	if body == "" {
		return false, "", ""
	}

	// Count non-trivial statements (excluding null checks, type checks)
	stmts := countStatements(body)
	if stmts > 3 {
		// Too many statements to be a simple wrapper
		return false, "", ""
	}

	// Check expression-bodied delegation: => _field.Method(args)
	if m := reExpressionDelegation.FindStringSubmatch(body); m != nil {
		methodName := m[1]
		return true, methodName, "self_method"
	}

	// Check expression-bodied static delegation: => ClassName.Method(args)
	if m := reExpressionStaticDelegation.FindStringSubmatch(body); m != nil {
		className := m[1]
		methodName := m[2]
		if idx != nil && idx.HasType(className) {
			return true, className + "." + methodName, "function"
		}
		// Check if it's a known type in the index
		if idx != nil {
			if _, ok := idx.FileForType(className); ok {
				return true, className + "." + methodName, "function"
			}
		}
		return true, className + "." + methodName, "function"
	}

	// Check base delegation: return base.Method(args)
	if m := reBaseDelegation.FindStringSubmatch(body); m != nil {
		methodName := m[1]
		return true, methodName, "self_method"
	}

	// Check field delegation: return _field.Method(args)
	if m := reDelegation.FindStringSubmatch(body); m != nil {
		methodName := m[1]
		// This is delegation to an inner field (interface forwarding / decorator)
		return true, methodName, "method"
	}

	// Check static delegation: return ClassName.StaticMethod(args)
	if m := reStaticDelegation.FindStringSubmatch(body); m != nil {
		className := m[1]
		methodName := m[2]
		return true, className + "." + methodName, "function"
	}

	return false, "", ""
}

// extractMethodBody extracts the method body from full source code,
// stripping the signature and outer braces.
func extractMethodBody(sourceCode string) string {
	// Look for opening brace or => for expression-bodied
	lines := strings.Split(sourceCode, "\n")
	var bodyLines []string
	inBody := false
	braceDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip doc comments
		if strings.HasPrefix(trimmed, "///") {
			continue
		}

		// Look for expression-bodied member
		if strings.Contains(trimmed, "=>") && !inBody {
			return trimmed
		}

		if !inBody {
			if strings.Contains(trimmed, "{") {
				inBody = true
				braceDepth = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
				// Get content after first {
				if idx := strings.Index(trimmed, "{"); idx >= 0 {
					rest := strings.TrimSpace(trimmed[idx+1:])
					if rest != "" && rest != "}" {
						bodyLines = append(bodyLines, rest)
					}
				}
				continue
			}
			continue
		}

		// Inside body
		braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
		if braceDepth <= 0 {
			// Remove trailing }
			if idx := strings.LastIndex(trimmed, "}"); idx >= 0 {
				rest := strings.TrimSpace(trimmed[:idx])
				if rest != "" {
					bodyLines = append(bodyLines, rest)
				}
			}
			break
		}
		bodyLines = append(bodyLines, trimmed)
	}

	return strings.Join(bodyLines, "\n")
}

// countStatements counts meaningful statements in a method body.
// Skips null checks, argument validation, and comments.
func countStatements(body string) int {
	lines := strings.Split(body, "\n")
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "{" || trimmed == "}" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Skip null/argument checks
		if strings.Contains(trimmed, "ArgumentNullException") ||
			strings.Contains(trimmed, "ArgumentException") ||
			strings.Contains(trimmed, "?? throw") ||
			strings.HasPrefix(trimmed, "if (") && strings.Contains(trimmed, "null") {
			continue
		}
		count++
	}
	return count
}
