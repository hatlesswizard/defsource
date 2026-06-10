package cpp

import (
	"regexp"
	"strings"
)

// Wrapper detection patterns for C++ delegation.
var (
	// Single return statement calling another function:
	// return someFunction(args...);
	reSingleReturnCall = regexp.MustCompile(
		`(?s)\{\s*return\s+([a-zA-Z_]\w*(?:::\w+)*)\s*\(([^)]*)\)\s*;\s*\}`)

	// Method forwarding to member: return member_.method(args);
	// or return impl_->method(args);
	reMemberForward = regexp.MustCompile(
		`(?s)\{\s*return\s+(\w+)[_]?(?:\.|->)\s*([a-zA-Z_]\w*)\s*\(([^)]*)\)\s*;\s*\}`)

	// std::forward pattern: return std::forward<T>(args)...
	reStdForward = regexp.MustCompile(
		`std::forward\s*<[^>]+>\s*\(`)

	// Pimpl forwarding: return pImpl->method(args);
	// or return d_ptr->method(args);
	rePimplForward = regexp.MustCompile(
		`(?s)\{\s*return\s+(?:pImpl|p_impl|d_ptr|m_impl|impl_|m_d)\s*->\s*([a-zA-Z_]\w*)\s*\(([^)]*)\)\s*;\s*\}`)

	// CRTP pattern: static_cast<Derived*>(this)->method(args)
	reCRTP = regexp.MustCompile(
		`static_cast\s*<\s*\w+\s*\*?\s*>\s*\(\s*this\s*\)\s*->\s*(\w+)\s*\(`)

	// Operator forwarding: return lhs op rhs; where op calls another operator
	reOperatorForward = regexp.MustCompile(
		`(?s)\{\s*return\s+([a-zA-Z_]\w*(?:::\w+)*)\s*\(([^)]*)\)\s*;\s*\}`)
)

// detectWrapper analyzes method source code for delegation patterns.
// Returns (isWrapper, targetName, targetKind).
func detectWrapper(sourceCode string, idx *codebaseIndex) (bool, string, string) {
	sourceCode = strings.TrimSpace(sourceCode)
	if sourceCode == "" {
		return false, "", ""
	}

	// Extract body if the source includes the signature
	body := extractBody(sourceCode)
	if body == "" {
		return false, "", ""
	}

	// Count statements — a wrapper should have at most 1-2 statements
	stmts := countStatements(body)
	if stmts > 2 {
		return false, "", ""
	}

	// Check for pimpl forwarding (highest priority)
	if m := rePimplForward.FindStringSubmatch(sourceCode); m != nil {
		targetMethod := m[1]
		return true, targetMethod, "method"
	}

	// Check for CRTP pattern
	if m := reCRTP.FindStringSubmatch(sourceCode); m != nil {
		targetMethod := m[1]
		return true, targetMethod, "method"
	}

	// Check for member forwarding (impl_->method or member.method)
	if m := reMemberForward.FindStringSubmatch(sourceCode); m != nil {
		targetMethod := m[2]
		return true, targetMethod, "method"
	}

	// Check for single-return function call
	if m := reSingleReturnCall.FindStringSubmatch(sourceCode); m != nil {
		targetName := m[1]
		// Skip std:: functions as they are builtins
		if strings.HasPrefix(targetName, "std::") {
			return false, "", ""
		}
		// Determine kind
		kind := "function"
		if strings.Contains(targetName, "::") {
			kind = "method"
		}
		return true, targetName, kind
	}

	return false, "", ""
}

// extractBody extracts the function body (between outermost { and }).
func extractBody(source string) string {
	start := strings.Index(source, "{")
	if start < 0 {
		return ""
	}
	// Find matching closing brace
	depth := 0
	for i := start; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start : i+1]
			}
		}
	}
	return ""
}

// countStatements counts semicolons at the top level of a body block
// (rough statement count).
func countStatements(body string) int {
	if len(body) < 2 {
		return 0
	}
	// Strip outer braces
	inner := body[1 : len(body)-1]
	count := 0
	depth := 0
	for _, ch := range inner {
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
		case ';':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}
