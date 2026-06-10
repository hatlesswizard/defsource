package clang

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test fixture: a sample C header file with various entity types.
const testHeader = `
#ifndef TEST_H
#define TEST_H

/**
 * @brief A point in 2D space.
 *
 * Represents a coordinate pair.
 * @since 1.0
 */
struct Point {
    int x;     /**< X coordinate */
    int y;     /**< Y coordinate */
};

/**
 * @brief Color values.
 */
enum Color {
    RED = 0,
    GREEN = 1,
    BLUE = 2
};

/**
 * @brief A simple tagged union.
 */
union Value {
    int i;
    float f;
    char *s;
};

/**
 * @brief Callback function type.
 * @param data User data pointer.
 * @param size Size in bytes.
 * @return Status code.
 */
typedef int (*callback_fn)(void *data, size_t size);

/// Type alias for unsigned integer.
typedef unsigned int uint32;

/**
 * @brief Compute the distance between two points.
 * @param a First point.
 * @param b Second point.
 * @return The Euclidean distance.
 */
double point_distance(struct Point a, struct Point b);

/**
 * @brief Create a new point.
 * @param x X coordinate.
 * @param y Y coordinate.
 * @return A new Point struct.
 */
struct Point point_new(int x, int y);

/// @brief A variadic logging function.
/// @param fmt Format string.
void log_message(const char *fmt, ...);

/**
 * @brief Simple inline wrapper.
 */
static inline double fast_distance(struct Point a, struct Point b) {
    return point_distance(a, b);
}

/* Forward declaration of opaque type */
struct OpaqueHandle;

/**
 * @brief Maximum buffer size.
 * @param n Number of elements.
 */
#define MAX_BUF_SIZE(n) ((n) * 1024)

/// @brief Wrapper macro for point_new.
#define POINT(x, y) point_new((x), (y))

extern int global_counter;

/**
 * @brief Bit field example.
 */
struct Flags {
    unsigned int readable : 1;
    unsigned int writable : 1;
    unsigned int executable : 1;
};

#endif /* TEST_H */
`

func TestDiscoveryFromHeaderFiles(t *testing.T) {
	// Create a temp dir with test header files.
	dir := t.TempDir()
	includeDir := filepath.Join(dir, "include")
	if err := os.MkdirAll(includeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(includeDir, "test.h"), []byte(testHeader), 0o644); err != nil {
		t.Fatal(err)
	}

	// Also create an internal header that should be excluded.
	internalDir := filepath.Join(dir, "include", "internal")
	if err := os.MkdirAll(internalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(internalDir, "private.h"), []byte("int secret_func(void);"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := New(dir, Config{
		LibraryID:  "/c/test",
		Name:       "Test Library",
		HeaderDirs: []string{"include"},
	})

	ids, err := src.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities failed: %v", err)
	}

	if len(ids) == 0 {
		t.Fatal("Expected entities to be discovered, got none")
	}

	// Verify some expected entities are present.
	found := map[string]bool{}
	for _, id := range ids {
		_, frag := splitFragment(id)
		found[frag] = true
	}

	expected := []string{"Point", "Color", "Value", "point_distance", "point_new", "log_message", "MAX_BUF_SIZE", "POINT"}
	for _, name := range expected {
		if !found[name] {
			t.Errorf("Expected entity %q not found in discovery results", name)
		}
	}

	// Internal header entities should not appear.
	if found["secret_func"] {
		t.Error("Internal function secret_func should not be discovered")
	}
}

func TestFunctionDeclarationParsing(t *testing.T) {
	content := []byte(testHeader)
	analysis := parseFileC(content)

	// Find point_distance.
	var fn *functionDef
	for i := range analysis.Functions {
		if analysis.Functions[i].Name == "point_distance" {
			fn = &analysis.Functions[i]
			break
		}
	}

	if fn == nil {
		t.Fatal("Function point_distance not found")
	}

	if fn.ReturnType != "double" {
		t.Errorf("Expected return type 'double', got %q", fn.ReturnType)
	}

	if len(fn.Params) != 2 {
		t.Fatalf("Expected 2 params, got %d", len(fn.Params))
	}

	if fn.Params[0].Name != "a" {
		t.Errorf("Expected first param name 'a', got %q", fn.Params[0].Name)
	}

	if !strings.Contains(fn.Signature, "point_distance") {
		t.Errorf("Signature should contain function name, got %q", fn.Signature)
	}
}

func TestStructWithFields(t *testing.T) {
	content := []byte(testHeader)
	analysis := parseFileC(content)

	var st *structDef
	for i := range analysis.Structs {
		if analysis.Structs[i].Name == "Point" {
			st = &analysis.Structs[i]
			break
		}
	}

	if st == nil {
		t.Fatal("Struct Point not found")
	}

	if len(st.Fields) < 2 {
		t.Fatalf("Expected at least 2 fields in Point, got %d", len(st.Fields))
	}

	fieldNames := map[string]bool{}
	for _, f := range st.Fields {
		fieldNames[f.Name] = true
	}

	if !fieldNames["x"] {
		t.Error("Expected field 'x' in Point struct")
	}
	if !fieldNames["y"] {
		t.Error("Expected field 'y' in Point struct")
	}
}

func TestEnumParsing(t *testing.T) {
	content := []byte(testHeader)
	analysis := parseFileC(content)

	var en *enumDef
	for i := range analysis.Enums {
		if analysis.Enums[i].Name == "Color" {
			en = &analysis.Enums[i]
			break
		}
	}

	if en == nil {
		t.Fatal("Enum Color not found")
	}

	if len(en.Constants) != 3 {
		t.Fatalf("Expected 3 enum constants, got %d", len(en.Constants))
	}

	constNames := map[string]string{}
	for _, c := range en.Constants {
		constNames[c.Name] = c.Value
	}

	if constNames["RED"] != "0" {
		t.Errorf("Expected RED=0, got RED=%q", constNames["RED"])
	}
	if constNames["GREEN"] != "1" {
		t.Errorf("Expected GREEN=1, got GREEN=%q", constNames["GREEN"])
	}
	if constNames["BLUE"] != "2" {
		t.Errorf("Expected BLUE=2, got BLUE=%q", constNames["BLUE"])
	}
}

func TestTypedefParsing(t *testing.T) {
	content := []byte(testHeader)
	analysis := parseFileC(content)

	foundCallback := false
	foundUint32 := false
	for _, td := range analysis.Typedefs {
		if td.Name == "callback_fn" {
			foundCallback = true
			if !td.IsFuncPtr {
				t.Error("Expected callback_fn to be marked as function pointer typedef")
			}
		}
		if td.Name == "uint32" {
			foundUint32 = true
		}
	}

	if !foundCallback {
		t.Error("Typedef callback_fn not found")
	}
	if !foundUint32 {
		t.Error("Typedef uint32 not found")
	}
}

func TestMacroExtraction(t *testing.T) {
	content := []byte(testHeader)
	macros := extractMacros(content)

	macroNames := map[string]*macroDef{}
	for i := range macros {
		macroNames[macros[i].Name] = &macros[i]
	}

	maxBuf, ok := macroNames["MAX_BUF_SIZE"]
	if !ok {
		t.Fatal("Macro MAX_BUF_SIZE not found")
	}
	if len(maxBuf.Params) != 1 || maxBuf.Params[0] != "n" {
		t.Errorf("Expected MAX_BUF_SIZE params=[n], got %v", maxBuf.Params)
	}

	point, ok := macroNames["POINT"]
	if !ok {
		t.Fatal("Macro POINT not found")
	}
	if len(point.Params) != 2 {
		t.Errorf("Expected POINT to have 2 params, got %d", len(point.Params))
	}
}

func TestDoxygenCommentExtraction(t *testing.T) {
	content := []byte(testHeader)
	analysis := parseFileC(content)

	// Check that point_distance has a doc comment.
	for _, fn := range analysis.Functions {
		if fn.Name == "point_distance" {
			if fn.DocComment == "" {
				t.Error("Expected point_distance to have a doc comment")
			}
			if !strings.Contains(fn.DocComment, "distance") {
				t.Errorf("Expected doc to mention 'distance', got %q", fn.DocComment)
			}
			return
		}
	}
	t.Error("point_distance not found for doc comment check")
}

func TestWrapperDetection(t *testing.T) {
	idx := emptyIndex()
	idx.functions["point_distance"] = "/test.h"
	idx.functions["point_new"] = "/test.h"

	// Inline wrapper function.
	wrapperSrc := `{
    return point_distance(a, b);
}`
	isWrapper, target, kind := detectWrapper(wrapperSrc, idx)
	if !isWrapper {
		t.Error("Expected fast_distance to be detected as wrapper")
	}
	if target != "point_distance" {
		t.Errorf("Expected target 'point_distance', got %q", target)
	}
	if kind != "function" {
		t.Errorf("Expected kind 'function', got %q", kind)
	}

	// Macro wrapper.
	macroSrc := `point_new((x), (y))`
	isWrapper, target, kind = detectWrapper(macroSrc, idx)
	if !isWrapper {
		t.Error("Expected POINT macro to be detected as wrapper")
	}
	if target != "point_new" {
		t.Errorf("Expected target 'point_new', got %q", target)
	}
	if kind != "macro" {
		t.Errorf("Expected kind 'macro', got %q", kind)
	}

	// Non-wrapper (calls stdlib function).
	nonWrapperSrc := `{
    return malloc(size);
}`
	isWrapper, _, _ = detectWrapper(nonWrapperSrc, idx)
	if isWrapper {
		t.Error("malloc call should not be detected as wrapper (stdlib function)")
	}
}

func TestForwardDeclarationHandling(t *testing.T) {
	content := []byte(testHeader)
	analysis := parseFileC(content)

	// OpaqueHandle should be found as a forward-declared struct.
	found := false
	for _, st := range analysis.Structs {
		if st.Name == "OpaqueHandle" {
			found = true
			if !st.IsForward {
				t.Error("Expected OpaqueHandle to be marked as forward declaration")
			}
			if len(st.Fields) != 0 {
				t.Error("Forward-declared struct should have no fields")
			}
			break
		}
	}

	if !found {
		t.Error("Forward-declared struct OpaqueHandle not found")
	}
}

func TestFunctionPointerParams(t *testing.T) {
	src := []byte(`
/**
 * @brief Register a callback.
 */
void register_callback(int (*handler)(void *data, int size), void *ctx);
`)
	analysis := parseFileC(src)

	if len(analysis.Functions) == 0 {
		t.Fatal("Expected register_callback function")
	}

	fn := analysis.Functions[0]
	if fn.Name != "register_callback" {
		t.Errorf("Expected name 'register_callback', got %q", fn.Name)
	}
}

func TestVariadicFunction(t *testing.T) {
	content := []byte(testHeader)
	analysis := parseFileC(content)

	var fn *functionDef
	for i := range analysis.Functions {
		if analysis.Functions[i].Name == "log_message" {
			fn = &analysis.Functions[i]
			break
		}
	}

	if fn == nil {
		t.Fatal("Function log_message not found")
	}

	// Should have at least 2 params: const char *fmt and ...
	if len(fn.Params) < 2 {
		t.Fatalf("Expected at least 2 params for variadic function, got %d", len(fn.Params))
	}

	lastParam := fn.Params[len(fn.Params)-1]
	if lastParam.Type != "..." {
		t.Errorf("Expected last param to be variadic '...', got type=%q name=%q", lastParam.Type, lastParam.Name)
	}
}

func TestParseEntity(t *testing.T) {
	dir := t.TempDir()
	headerPath := filepath.Join(dir, "test.h")
	if err := os.WriteFile(headerPath, []byte(testHeader), 0o644); err != nil {
		t.Fatal(err)
	}

	src := New(dir, Config{
		LibraryID: "/c/test",
		Name:      "Test",
	})

	// Parse a function entity.
	entityID := headerPath + "#point_distance"
	entity, _, err := src.ParseEntity(context.Background(), entityID, []byte(testHeader))
	if err != nil {
		t.Fatalf("ParseEntity failed: %v", err)
	}

	if entity.Name != "point_distance" {
		t.Errorf("Expected entity name 'point_distance', got %q", entity.Name)
	}
	if entity.Kind != "function" {
		t.Errorf("Expected entity kind 'function', got %q", entity.Kind)
	}
	if entity.Description == "" {
		t.Error("Expected entity to have a description from doxygen comment")
	}

	// Parse a struct entity.
	entityID = headerPath + "#Point"
	entity, _, err = src.ParseEntity(context.Background(), entityID, []byte(testHeader))
	if err != nil {
		t.Fatalf("ParseEntity for struct failed: %v", err)
	}

	if entity.Name != "Point" {
		t.Errorf("Expected entity name 'Point', got %q", entity.Name)
	}
	if entity.Kind != "struct" {
		t.Errorf("Expected entity kind 'struct', got %q", entity.Kind)
	}
	if len(entity.Properties) < 2 {
		t.Errorf("Expected at least 2 properties for Point, got %d", len(entity.Properties))
	}
}

func TestParseSourceCode(t *testing.T) {
	dir := t.TempDir()
	headerPath := filepath.Join(dir, "test.h")
	if err := os.WriteFile(headerPath, []byte(testHeader), 0o644); err != nil {
		t.Fatal(err)
	}

	src := New(dir, Config{
		LibraryID: "/c/test",
		Name:      "Test",
	})

	// Extract source for a function.
	entityID := headerPath + "#point_distance"
	code, err := src.ParseSourceCode(entityID, []byte(testHeader))
	if err != nil {
		t.Fatalf("ParseSourceCode failed: %v", err)
	}

	if !strings.Contains(code, "point_distance") {
		t.Errorf("Expected source to contain 'point_distance', got %q", code)
	}
}

func TestUnionParsing(t *testing.T) {
	content := []byte(testHeader)
	analysis := parseFileC(content)

	var un *unionDef
	for i := range analysis.Enums {
		_ = analysis.Enums[i] // consume to avoid unused warning in range
	}
	for i := range analysis.Unions {
		if analysis.Unions[i].Name == "Value" {
			un = &analysis.Unions[i]
			break
		}
	}

	if un == nil {
		t.Fatal("Union Value not found")
	}

	if len(un.Fields) < 3 {
		t.Fatalf("Expected at least 3 fields in union Value, got %d", len(un.Fields))
	}
}

func TestMultilineMacro(t *testing.T) {
	src := []byte(`
/** @brief Multi-line do-while macro. */
#define SAFE_FREE(ptr) do { \
    free(ptr);             \
    (ptr) = NULL;          \
} while(0)
`)
	macros := extractMacros(src)
	if len(macros) == 0 {
		t.Fatal("Expected SAFE_FREE macro to be extracted")
	}
	if macros[0].Name != "SAFE_FREE" {
		t.Errorf("Expected macro name 'SAFE_FREE', got %q", macros[0].Name)
	}
	if !strings.Contains(macros[0].Body, "free") {
		t.Errorf("Expected macro body to contain 'free', got %q", macros[0].Body)
	}
}

func TestIDAndMeta(t *testing.T) {
	src := New("/tmp/test", Config{
		LibraryID:   "/c/sqlite",
		Name:        "SQLite",
		Description: "SQLite database engine",
		SourceURL:   "https://github.com/sqlite/sqlite",
		Version:     "3.45.0",
		TrustScore:  0.95,
	})

	if src.ID() != "/c/sqlite" {
		t.Errorf("Expected ID '/c/sqlite', got %q", src.ID())
	}

	meta := src.Meta()
	if meta.Language != "c" {
		t.Errorf("Expected language 'c', got %q", meta.Language)
	}
	if meta.Name != "SQLite" {
		t.Errorf("Expected name 'SQLite', got %q", meta.Name)
	}
	if meta.Version != "3.45.0" {
		t.Errorf("Expected version '3.45.0', got %q", meta.Version)
	}
}

func TestBitFields(t *testing.T) {
	content := []byte(testHeader)
	analysis := parseFileC(content)

	var flags *structDef
	for i := range analysis.Structs {
		if analysis.Structs[i].Name == "Flags" {
			flags = &analysis.Structs[i]
			break
		}
	}

	if flags == nil {
		t.Fatal("Struct Flags not found")
	}

	if len(flags.Fields) < 3 {
		t.Fatalf("Expected at least 3 fields in Flags, got %d", len(flags.Fields))
	}
}

func TestExternDeclaration(t *testing.T) {
	src := []byte(`
extern int init_library(void);
extern void shutdown_library(void);
`)
	analysis := parseFileC(src)

	foundInit := false
	foundShutdown := false
	for _, fn := range analysis.Functions {
		if fn.Name == "init_library" {
			foundInit = true
			if !fn.IsExtern {
				t.Error("Expected init_library to be marked extern")
			}
		}
		if fn.Name == "shutdown_library" {
			foundShutdown = true
			if !fn.IsExtern {
				t.Error("Expected shutdown_library to be marked extern")
			}
		}
	}

	if !foundInit {
		t.Error("extern function init_library not found")
	}
	if !foundShutdown {
		t.Error("extern function shutdown_library not found")
	}
}
