package clang

import (
	"regexp"
	"strings"
)

// Wrapper detection patterns for C code.
//
// In C, a "wrapper" is typically:
// 1. A static inline function that just calls another function (thin wrapper)
// 2. A macro that expands to a function call
// 3. A function pointer typedef that aliases another function

var (
	// Matches a single-statement function body that is just a return of a function call.
	// e.g., "return some_func(arg1, arg2);"
	reReturnCall = regexp.MustCompile(`(?s)\{\s*return\s+([A-Za-z_]\w*)\s*\([^;]*\);\s*\}`)

	// Matches a single-statement function body that is just a function call (void return).
	// e.g., "{ some_func(arg1, arg2); }"
	reVoidCall = regexp.MustCompile(`(?s)\{\s*([A-Za-z_]\w*)\s*\([^;]*\);\s*\}`)

	// Matches a macro body that is just a function call.
	// e.g., "#define FOO(x) bar(x)"
	reMacroCall = regexp.MustCompile(`^\s*([A-Za-z_]\w*)\s*\(`)
)

// detectWrapper analyzes source code and returns whether it is a wrapper,
// the target function name, and the wrapper kind.
func detectWrapper(sourceCode string, idx *codebaseIndex) (bool, string, string) {
	sourceCode = strings.TrimSpace(sourceCode)
	if sourceCode == "" {
		return false, "", ""
	}

	// Check for return-call pattern (most common wrapper pattern).
	if m := reReturnCall.FindStringSubmatch(sourceCode); m != nil {
		target := m[1]
		if isValidWrapperTarget(target, idx) {
			return true, target, "function"
		}
	}

	// Check for void-call pattern (single call statement).
	if m := reVoidCall.FindStringSubmatch(sourceCode); m != nil {
		target := m[1]
		if isValidWrapperTarget(target, idx) {
			return true, target, "function"
		}
	}

	// Check for macro wrapper (body is just a function call).
	// This handles cases where the "source code" is a macro body.
	if !strings.HasPrefix(sourceCode, "{") {
		if m := reMacroCall.FindStringSubmatch(sourceCode); m != nil {
			target := m[1]
			if isValidWrapperTarget(target, idx) {
				return true, target, "macro"
			}
		}
	}

	return false, "", ""
}

// isValidWrapperTarget checks whether a target name is a valid wrapper
// destination (exists in the index and is not a C standard library call
// that we should not resolve further).
func isValidWrapperTarget(name string, idx *codebaseIndex) bool {
	if name == "" {
		return false
	}

	// Skip C standard library functions — these are terminal.
	if cStdLibFunctions[name] {
		return false
	}

	// Must be a known function or macro in the index.
	if idx.HasFunction(name) {
		return true
	}
	if _, ok := idx.macros[name]; ok {
		return true
	}

	return false
}

// cStdLibFunctions is the set of common C standard library functions that
// should terminate wrapper chain resolution. These represent the "leaves"
// of the call graph from a documentation perspective.
var cStdLibFunctions = map[string]bool{
	// stdio.h
	"printf": true, "fprintf": true, "sprintf": true, "snprintf": true,
	"scanf": true, "fscanf": true, "sscanf": true,
	"fopen": true, "fclose": true, "fread": true, "fwrite": true,
	"fgets": true, "fputs": true, "fputc": true, "fgetc": true,
	"fseek": true, "ftell": true, "rewind": true, "fflush": true,
	"puts": true, "putchar": true, "getchar": true,
	"perror": true, "remove": true, "rename": true, "tmpfile": true,

	// stdlib.h
	"malloc": true, "calloc": true, "realloc": true, "free": true,
	"exit": true, "abort": true, "atexit": true,
	"atoi": true, "atol": true, "atof": true,
	"strtol": true, "strtoul": true, "strtod": true, "strtof": true,
	"rand": true, "srand": true,
	"qsort": true, "bsearch": true,
	"abs": true, "labs": true,
	"getenv": true, "system": true,

	// string.h
	"memcpy": true, "memmove": true, "memset": true, "memcmp": true, "memchr": true,
	"strcpy": true, "strncpy": true, "strcat": true, "strncat": true,
	"strcmp": true, "strncmp": true, "strlen": true,
	"strchr": true, "strrchr": true, "strstr": true, "strtok": true,
	"strdup": true, "strerror": true,

	// math.h
	"sin": true, "cos": true, "tan": true,
	"asin": true, "acos": true, "atan": true, "atan2": true,
	"sqrt": true, "pow": true, "exp": true, "log": true, "log10": true,
	"ceil": true, "floor": true, "fabs": true, "fmod": true,

	// ctype.h
	"isalpha": true, "isdigit": true, "isalnum": true, "isspace": true,
	"toupper": true, "tolower": true, "isupper": true, "islower": true,

	// assert.h
	"assert": true,

	// errno.h - not functions but often referenced
	// signal.h
	"signal": true, "raise": true,

	// time.h
	"time": true, "clock": true, "difftime": true, "mktime": true,
	"strftime": true, "localtime": true, "gmtime": true,

	// unistd.h (POSIX)
	"read": true, "write": true, "open": true, "close": true,
	"fork": true, "exec": true, "execv": true, "execve": true,
	"getpid": true, "getppid": true, "sleep": true, "usleep": true,

	// pthread.h
	"pthread_create": true, "pthread_join": true, "pthread_exit": true,
	"pthread_mutex_lock": true, "pthread_mutex_unlock": true,
	"pthread_mutex_init": true, "pthread_mutex_destroy": true,
}
