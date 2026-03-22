package wordpress

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hatlesswizard/defsource/internal/source"
)

var (
	reReturnFunc         = regexp.MustCompile(`^\s*return\s+(\w+)\s*\(`)
	reReturnThisMethod   = regexp.MustCompile(`^\s*return\s+\$this->(\w+)\s*\(`)
	reReturnSelfMethod   = regexp.MustCompile(`^\s*return\s+(?:self|static)::(\w+)\s*\(`)
	reReturnStaticMethod = regexp.MustCompile(`^\s*return\s+(\w+)::(\w+)\s*\(`)
	reVoidCall           = regexp.MustCompile(`^\s*(\w+)\s*\(`)
	reVoidThisCall       = regexp.MustCompile(`^\s*\$this->(\w+)\s*\(`)
)

// DetectWrapper analyzes a method's source code to determine if it's a wrapper.
// Returns (isWrapper, targetName, targetKind).
func (w *WordPressSource) DetectWrapper(method *source.Method) (bool, string, string) {
	if method.SourceCode == "" {
		return false, "", ""
	}

	bodyLines := extractBodyLines(method.SourceCode)

	if len(bodyLines) > 5 || len(bodyLines) == 0 {
		return false, "", ""
	}

	// Check each line for delegation patterns
	for _, line := range bodyLines {
		line = strings.TrimSpace(line)

		if matches := reReturnThisMethod.FindStringSubmatch(line); len(matches) > 1 {
			return true, matches[1], "self_method"
		}

		if matches := reReturnSelfMethod.FindStringSubmatch(line); len(matches) > 1 {
			return true, matches[1], "self_method"
		}

		if matches := reReturnStaticMethod.FindStringSubmatch(line); len(matches) > 2 {
			if matches[1] == "parent" {
				continue // parent:: calls are not resolvable wrappers
			}
			return true, matches[1] + "::" + matches[2], "static_method"
		}

		if matches := reReturnFunc.FindStringSubmatch(line); len(matches) > 1 {
			name := matches[1]
			if !phpBuiltins[name] {
				return true, name, "function"
			}
		}
	}

	// Check for void wrappers (single-line body)
	if len(bodyLines) == 1 {
		line := strings.TrimSpace(bodyLines[0])

		if matches := reVoidThisCall.FindStringSubmatch(line); len(matches) > 1 {
			return true, matches[1], "self_method"
		}
		if matches := reVoidCall.FindStringSubmatch(line); len(matches) > 1 {
			if !voidBuiltinCalls[matches[1]] {
				return true, matches[1], "function"
			}
		}
	}

	return false, "", ""
}

var voidBuiltinCalls = map[string]bool{
	"unset": true, "echo": true, "print": true, "die": true, "exit": true,
	"trigger_error": true, "error_log": true, "header": true, "setcookie": true,
	"session_start": true, "session_destroy": true,
}

var phpBuiltins = map[string]bool{
	"array": true, "isset": true, "empty": true,
	"is_null": true, "is_array": true, "is_string": true,
	"is_object": true, "is_int": true, "is_float": true,
	"is_bool": true, "is_numeric": true, "is_callable": true,
	"is_resource": true, "is_integer": true, "is_long": true,
	"is_double": true, "is_real": true, "is_finite": true,
	"is_infinite": true, "is_nan": true, "is_a": true,
	"count": true, "strlen": true, "intval": true,
	"floatval": true, "strval": true, "boolval": true,
	"compact": true, "extract": true, "list": true,
	"true": true, "false": true, "null": true,
	"return": true,
	// String functions
	"str_replace": true, "sprintf": true, "substr": true,
	"implode": true, "explode": true, "trim": true,
	"ltrim": true, "rtrim": true,
	// Array functions
	"in_array": true, "array_merge": true, "array_keys": true,
	"array_values": true, "array_map": true, "array_filter": true,
	"array_pop": true, "array_push": true,
	"array_key_exists": true, "array_key_first": true, "array_key_last": true,
	"array_splice": true, "array_slice": true, "array_unique": true,
	"array_column": true, "array_combine": true, "array_diff": true,
	"array_intersect": true, "array_reverse": true, "array_search": true,
	"array_sum": true, "key": true, "current": true, "next": true, "prev": true,
	"reset": true, "end": true, "sizeof": true,
	// String functions (extended)
	"strcmp": true, "strcasecmp": true, "strpos": true, "strrpos": true,
	"strstr": true, "strtolower": true, "strtoupper": true, "ucfirst": true,
	"lcfirst": true, "ucwords": true, "str_pad": true, "str_repeat": true,
	"str_word_count": true, "wordwrap": true, "nl2br": true, "chunk_split": true,
	"number_format": true,
	// HTML/URL functions
	"html_entity_decode": true, "htmlspecialchars": true, "htmlentities": true,
	"htmlspecialchars_decode": true, "strip_tags": true,
	"urlencode": true, "urldecode": true, "rawurlencode": true, "rawurldecode": true,
	"http_build_query": true,
	// Type checking (extended)
	"property_exists": true, "class_exists": true, "method_exists": true,
	"function_exists": true, "defined": true, "gettype": true, "settype": true,
	"get_object_vars": true, "get_class": true, "get_parent_class": true,
	// File system
	"file_exists": true, "file_get_contents": true, "file_put_contents": true,
	"is_readable": true, "is_writable": true, "is_dir": true, "is_file": true,
	"is_link": true, "mkdir": true, "rmdir": true, "unlink": true, "rename": true,
	"copy": true, "chmod": true, "realpath": true, "dirname": true, "basename": true,
	"pathinfo": true, "glob": true, "tempnam": true, "sys_get_temp_dir": true,
	// Date/time
	"date": true, "time": true, "strtotime": true, "mktime": true,
	"microtime": true, "gmdate": true, "date_create": true,
	// Crypto/encoding
	"md5": true, "sha1": true, "hash": true, "crc32": true,
	"base64_encode": true, "base64_decode": true,
	// Output/debug
	"var_export": true, "print_r": true, "var_dump": true,
	"debug_backtrace": true, "debug_print_backtrace": true,
	"trigger_error": true, "error_log": true,
	// Session/header
	"header": true, "setcookie": true, "session_start": true, "session_destroy": true,
	"headers_sent": true, "header_remove": true,
	// Math
	"abs": true, "ceil": true, "floor": true, "round": true, "max": true, "min": true,
	"pow": true, "sqrt": true, "log": true, "rand": true, "mt_rand": true,
	// Output buffering
	"ob_start": true, "ob_end_clean": true,
	// Regex
	"preg_match": true, "preg_replace": true,
	"preg_quote": true,
	// JSON
	"json_encode": true, "json_decode": true,
	// Misc
	"sleep": true, "usleep": true, "call_user_func": true, "call_user_func_array": true,
	"func_get_args": true, "func_num_args": true,
	// WordPress core
	"wp_parse_args": true, "absint": true,
	"esc_html": true, "esc_attr": true, "esc_url": true,
	"sanitize_text_field": true,
	"_deprecated_function": true, "_doing_it_wrong": true,
	"apply_filters": true, "do_action": true,
	"has_filter": true, "add_filter": true, "add_action": true,
	"remove_filter": true, "remove_action": true,
	"current_user_can": true, "wp_die": true,
	"wp_redirect": true, "wp_safe_redirect": true,
	"checked": true, "selected": true, "disabled": true,
}

// extractBodyLines strips the function signature and closing brace,
// returning only the body lines. Also strips comments.
func extractBodyLines(sourceCode string) []string {
	lines := strings.Split(sourceCode, "\n")
	var bodyLines []string
	insideBody := false
	braceDepth := 0
	inBlockComment := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inBlockComment {
			if strings.Contains(trimmed, "*/") {
				inBlockComment = false
			}
			continue
		}
		if strings.Contains(trimmed, "/*") {
			if strings.Contains(trimmed, "*/") {
				// Single-line block comment (e.g. /* comment */), skip the line
				continue
			}
			inBlockComment = true
			continue
		}

		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

		if !insideBody {
			if strings.Contains(trimmed, "{") {
				insideBody = true
				if idx := strings.Index(trimmed, "{"); idx < len(trimmed)-1 {
					after := strings.TrimSpace(trimmed[idx+1:])
					if after != "" && after != "}" {
						bodyLines = append(bodyLines, after)
					}
				}
			}
			continue
		}

		if braceDepth <= 0 {
			break
		}

		if trimmed == "" {
			continue
		}

		bodyLines = append(bodyLines, trimmed)
	}

	return bodyLines
}

// ResolveWrapperURL constructs the URL to fetch the wrapped method's documentation.
func (w *WordPressSource) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	switch targetKind {
	case "function":
		slug := strings.ToLower(targetName)
		return fmt.Sprintf("%s/reference/functions/%s/", BaseURL, slug)

	case "self_method":
		methodSlug := strings.ToLower(targetName)
		return fmt.Sprintf("%s/reference/classes/%s/%s/", BaseURL, entitySlug, methodSlug)

	case "static_method":
		className, methodName, ok := strings.Cut(targetName, "::")
		if !ok {
			return ""
		}
		return fmt.Sprintf("%s/reference/classes/%s/%s/", BaseURL, strings.ToLower(className), strings.ToLower(methodName))

	default:
		return ""
	}
}
