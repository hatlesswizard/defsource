package cpp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
)

// testHeader is a realistic C++ header for testing the parser.
const testHeader = `
#pragma once

#include <string>
#include <vector>
#include <memory>

namespace mylib {
namespace detail {
void internal_helper();
}

/**
 * \brief A sample class demonstrating documentation.
 *
 * This class provides basic container functionality.
 *
 * \tparam T The element type
 * \since 1.0
 */
template<typename T>
class Container {
public:
    /**
     * \brief Construct a container with initial capacity.
     * \param capacity Initial capacity
     */
    explicit Container(size_t capacity) : data_(capacity) {}

    /**
     * \brief Add an element to the container.
     * \param value The value to add
     */
    void push_back(const T& value) {
        data_.push_back(value);
    }

    /**
     * \brief Get element at index.
     * \param index The position
     * \return Reference to the element
     * \throws std::out_of_range if index is invalid
     */
    const T& at(size_t index) const {
        return data_.at(index);
    }

    /// \brief Get the size.
    /// \return Number of elements
    size_t size() const noexcept {
        return data_.size();
    }

    /// Check if empty.
    bool empty() const noexcept { return data_.empty(); }

    virtual ~Container() = default;

protected:
    std::vector<T> data_;

private:
    size_t max_size_ = 1024;
};

/**
 * \brief A derived container with extra features.
 * \deprecated Use Container directly
 */
class ExtendedContainer : public Container<int> {
public:
    void clear() { data_.clear(); }

    /// \brief Get sum of all elements.
    int sum() const;

    // deleted copy
    ExtendedContainer(const ExtendedContainer&) = delete;
    ExtendedContainer& operator=(const ExtendedContainer&) = delete;

    // defaulted move
    ExtendedContainer(ExtendedContainer&&) = default;
    ExtendedContainer& operator=(ExtendedContainer&&) = default;
};

/// \brief Color enumeration.
enum class Color : uint8_t {
    /// Red color
    Red = 0,
    /// Green color
    Green = 1,
    /// Blue color
    Blue = 2,
};

/// Type alias for string vector.
using StringList = std::vector<std::string>;

/**
 * \brief A simple struct for configuration.
 */
struct Config {
    std::string name;
    int value = 0;
    bool enabled = true;

    /// Validate the configuration.
    bool validate() const { return !name.empty(); }
};

/**
 * \brief Free function to create a container.
 * \param size Initial size
 * \return A new container
 */
template<typename T>
Container<T> make_container(size_t size) {
    return Container<T>(size);
}

/// \brief Compute factorial.
/// \param n Input number
/// \return n!
constexpr int factorial(int n) {
    return n <= 1 ? 1 : n * factorial(n - 1);
}

/**
 * \brief Wrapper that delegates to another function.
 */
int wrapper_func(int x) {
    return factorial(x);
}

} // namespace mylib
`

// testConcept is a C++20 concept example.
const testConcept = `
namespace concepts {

template<typename T>
concept Hashable = requires(T a) {
    { std::hash<T>{}(a) } -> std::convertible_to<std::size_t>;
};

} // namespace concepts
`

func TestParseFile_Classes(t *testing.T) {
	analysis := parseFile([]byte(testHeader))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	// Should find Container and ExtendedContainer classes
	if len(analysis.Classes) < 1 {
		t.Fatalf("expected at least 1 class, got %d", len(analysis.Classes))
	}

	var container *classDef
	var extended *classDef
	for i := range analysis.Classes {
		switch analysis.Classes[i].Name {
		case "Container":
			container = &analysis.Classes[i]
		case "ExtendedContainer":
			extended = &analysis.Classes[i]
		}
	}

	if container == nil {
		t.Fatal("Container class not found")
	}
	if container.QualifiedName != "mylib::Container" {
		t.Errorf("expected QualifiedName 'mylib::Container', got %q", container.QualifiedName)
	}
	if container.TemplateParams == "" {
		t.Error("expected template params on Container")
	}

	// Check methods exist
	if len(container.Methods) == 0 {
		t.Fatal("expected methods on Container")
	}

	var pushBack *methodDef
	var atMethod *methodDef
	var sizeMethod *methodDef
	for i := range container.Methods {
		switch container.Methods[i].Name {
		case "push_back":
			pushBack = &container.Methods[i]
		case "at":
			atMethod = &container.Methods[i]
		case "size":
			sizeMethod = &container.Methods[i]
		}
	}

	if pushBack == nil {
		t.Error("push_back method not found")
	} else if pushBack.Visibility != "public" {
		t.Errorf("push_back visibility: expected 'public', got %q", pushBack.Visibility)
	}

	if atMethod == nil {
		t.Error("at method not found")
	} else if !atMethod.Const {
		t.Error("at method should be const")
	}

	if sizeMethod == nil {
		t.Error("size method not found")
	} else {
		if !sizeMethod.Const {
			t.Error("size method should be const")
		}
		if !sizeMethod.Noexcept {
			t.Error("size method should be noexcept")
		}
	}

	// Check fields
	var dataField *fieldDef
	for i := range container.Fields {
		if container.Fields[i].Name == "data_" {
			dataField = &container.Fields[i]
		}
	}
	if dataField == nil {
		t.Error("data_ field not found")
	} else if dataField.Visibility != "protected" {
		t.Errorf("data_ visibility: expected 'protected', got %q", dataField.Visibility)
	}

	// ExtendedContainer
	if extended == nil {
		t.Fatal("ExtendedContainer class not found")
	}
	if len(extended.Bases) == 0 {
		t.Error("expected base classes on ExtendedContainer")
	}
}

func TestParseFile_Structs(t *testing.T) {
	analysis := parseFile([]byte(testHeader))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var config *structDef
	for i := range analysis.Structs {
		if analysis.Structs[i].Name == "Config" {
			config = &analysis.Structs[i]
			break
		}
	}

	if config == nil {
		t.Fatal("Config struct not found")
	}
	if config.QualifiedName != "mylib::Config" {
		t.Errorf("expected QualifiedName 'mylib::Config', got %q", config.QualifiedName)
	}

	// Check fields
	if len(config.Fields) < 2 {
		t.Errorf("expected at least 2 fields on Config, got %d", len(config.Fields))
	}

	// Check validate method
	var validate *methodDef
	for i := range config.Methods {
		if config.Methods[i].Name == "validate" {
			validate = &config.Methods[i]
			break
		}
	}
	if validate == nil {
		t.Error("validate method not found on Config")
	}
}

func TestParseFile_Functions(t *testing.T) {
	analysis := parseFile([]byte(testHeader))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var factorial *functionDef
	var makeContainer *functionDef
	var wrapperFunc *functionDef
	for i := range analysis.Functions {
		switch analysis.Functions[i].Name {
		case "factorial":
			factorial = &analysis.Functions[i]
		case "make_container":
			makeContainer = &analysis.Functions[i]
		case "wrapper_func":
			wrapperFunc = &analysis.Functions[i]
		}
	}

	if factorial == nil {
		t.Fatal("factorial function not found")
	}
	if factorial.QualifiedName != "mylib::factorial" {
		t.Errorf("expected QualifiedName 'mylib::factorial', got %q", factorial.QualifiedName)
	}
	if !factorial.Constexpr {
		t.Error("factorial should be constexpr")
	}

	if makeContainer == nil {
		t.Fatal("make_container function not found")
	}
	if makeContainer.TemplateParams == "" {
		t.Error("make_container should have template params")
	}

	if wrapperFunc == nil {
		t.Fatal("wrapper_func not found")
	}
}

func TestParseFile_Enums(t *testing.T) {
	analysis := parseFile([]byte(testHeader))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var color *enumDef
	for i := range analysis.Enums {
		if analysis.Enums[i].Name == "Color" {
			color = &analysis.Enums[i]
			break
		}
	}

	if color == nil {
		t.Fatal("Color enum not found")
	}
	if color.QualifiedName != "mylib::Color" {
		t.Errorf("expected QualifiedName 'mylib::Color', got %q", color.QualifiedName)
	}
	if !color.Scoped {
		t.Error("Color should be a scoped enum (enum class)")
	}
	if len(color.Values) < 3 {
		t.Errorf("expected at least 3 enum values, got %d", len(color.Values))
	}
}

func TestParseFile_TypeAliases(t *testing.T) {
	analysis := parseFile([]byte(testHeader))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var stringList *typeAliasDef
	for i := range analysis.TypeAliases {
		if analysis.TypeAliases[i].Name == "StringList" {
			stringList = &analysis.TypeAliases[i]
			break
		}
	}

	if stringList == nil {
		t.Fatal("StringList type alias not found")
	}
	if stringList.QualifiedName != "mylib::StringList" {
		t.Errorf("expected QualifiedName 'mylib::StringList', got %q", stringList.QualifiedName)
	}
}

func TestParseFile_Namespaces(t *testing.T) {
	analysis := parseFile([]byte(testHeader))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	// Should find mylib namespace
	found := false
	for _, ns := range analysis.Namespaces {
		if ns.QualifiedName == "mylib" {
			found = true
			break
		}
	}
	if !found {
		t.Error("mylib namespace not found")
	}
}

func TestParseFile_Concepts(t *testing.T) {
	analysis := parseFile([]byte(testConcept))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Concepts) == 0 {
		t.Fatal("no concepts found")
	}

	var hashable *conceptDef
	for i := range analysis.Concepts {
		if analysis.Concepts[i].Name == "Hashable" {
			hashable = &analysis.Concepts[i]
			break
		}
	}
	if hashable == nil {
		t.Fatal("Hashable concept not found")
	}
	if hashable.QualifiedName != "concepts::Hashable" {
		t.Errorf("expected QualifiedName 'concepts::Hashable', got %q", hashable.QualifiedName)
	}
}

func TestParseFile_DocComments(t *testing.T) {
	analysis := parseFile([]byte(testHeader))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	// Container should have a doc comment
	var container *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].Name == "Container" {
			container = &analysis.Classes[i]
			break
		}
	}
	if container == nil {
		t.Fatal("Container not found")
	}
	if container.DocComment == "" {
		t.Error("Container should have a doc comment")
	}
	if !strings.Contains(container.DocComment, "sample class") {
		t.Errorf("Container doc should mention 'sample class', got %q", container.DocComment)
	}
}

func TestParseFile_AccessSpecifiers(t *testing.T) {
	src := `
class MyClass {
public:
    void publicMethod() {}
    int publicField;

protected:
    void protectedMethod() {}

private:
    void privateMethod() {}
    int privateField_;
};
`
	analysis := parseFile([]byte(src))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Classes) == 0 {
		t.Fatal("no classes found")
	}

	cls := &analysis.Classes[0]

	// Check method visibilities
	for _, m := range cls.Methods {
		switch m.Name {
		case "publicMethod":
			if m.Visibility != "public" {
				t.Errorf("publicMethod visibility: expected 'public', got %q", m.Visibility)
			}
		case "protectedMethod":
			if m.Visibility != "protected" {
				t.Errorf("protectedMethod visibility: expected 'protected', got %q", m.Visibility)
			}
		case "privateMethod":
			if m.Visibility != "private" {
				t.Errorf("privateMethod visibility: expected 'private', got %q", m.Visibility)
			}
		}
	}
}

func TestWrapperDetection(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		isWrapper  bool
		targetName string
		targetKind string
	}{
		{
			name:       "simple forwarding",
			source:     `int wrapper(int x) { return delegate(x); }`,
			isWrapper:  true,
			targetName: "delegate",
			targetKind: "function",
		},
		{
			name:       "pimpl forwarding",
			source:     `void Widget::show() { return pImpl->show(); }`,
			isWrapper:  true,
			targetName: "show",
			targetKind: "method",
		},
		{
			name:       "member forwarding",
			source:     `int Proxy::get() { return impl_.get(); }`,
			isWrapper:  true,
			targetName: "get",
			targetKind: "method",
		},
		{
			name:       "not a wrapper - multiple statements",
			source:     `int process(int x) { int y = transform(x); return finalize(y); }`,
			isWrapper:  false,
			targetName: "",
			targetKind: "",
		},
		{
			name:       "std function not a wrapper",
			source:     `size_t size() { return std::distance(begin_, end_); }`,
			isWrapper:  false,
			targetName: "",
			targetKind: "",
		},
		{
			name:       "CRTP forwarding",
			source:     `void impl() { return static_cast<Derived*>(this)->do_impl(); }`,
			isWrapper:  true,
			targetName: "do_impl",
			targetKind: "method",
		},
	}

	idx := emptyIndex()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isW, target, kind := detectWrapper(tt.source, idx)
			if isW != tt.isWrapper {
				t.Errorf("isWrapper: expected %v, got %v", tt.isWrapper, isW)
			}
			if target != tt.targetName {
				t.Errorf("targetName: expected %q, got %q", tt.targetName, target)
			}
			if kind != tt.targetKind {
				t.Errorf("targetKind: expected %q, got %q", tt.targetKind, kind)
			}
		})
	}
}

func TestDiscovery(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	includeDir := filepath.Join(tmpDir, "include", "mylib")
	if err := os.MkdirAll(includeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a test header
	headerContent := `
#pragma once
namespace mylib {
class Widget {
public:
    void show();
    void hide();
};

struct Point {
    double x, y;
};

void initialize();
} // namespace mylib
`
	headerPath := filepath.Join(includeDir, "widget.hpp")
	if err := os.WriteFile(headerPath, []byte(headerContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Also create an internal dir that should be skipped
	internalDir := filepath.Join(includeDir, "detail")
	if err := os.MkdirAll(internalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	internalHeader := filepath.Join(internalDir, "impl.hpp")
	if err := os.WriteFile(internalHeader, []byte("class InternalThing {};"), 0o644); err != nil {
		t.Fatal(err)
	}

	idx, err := buildCodebaseIndex(tmpDir, []string{"include"}, nil)
	if err != nil {
		t.Fatalf("buildCodebaseIndex failed: %v", err)
	}

	// Widget should be found
	if _, ok := idx.definedClasses["mylib::Widget"]; !ok {
		t.Error("mylib::Widget not found in index")
	}

	// Point should be found
	if _, ok := idx.definedStructs["mylib::Point"]; !ok {
		t.Error("mylib::Point not found in index")
	}

	// initialize function should be found
	if _, ok := idx.definedFunctions["mylib::initialize"]; !ok {
		t.Error("mylib::initialize not found in index")
	}

	// InternalThing should NOT be found (in detail/ directory)
	if _, ok := idx.definedClasses["InternalThing"]; ok {
		t.Error("InternalThing should not be indexed (it's in detail/ directory)")
	}

	// Check entity list
	entities := idx.buildEntityList()
	if len(entities) == 0 {
		t.Error("entity list is empty")
	}
	// Each entity should have path#name format
	for _, e := range entities {
		if !strings.Contains(e, "#") {
			t.Errorf("entity %q missing fragment separator", e)
		}
	}
}

func TestSource_Interface(t *testing.T) {
	// Create temp dir with a header
	tmpDir := t.TempDir()
	includeDir := filepath.Join(tmpDir, "include")
	if err := os.MkdirAll(includeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	headerContent := `
namespace testlib {
/**
 * \brief A test class.
 * \since 2.0
 */
class Foo {
public:
    /// \brief Do something.
    /// \param x The input value
    /// \return The result
    int bar(int x) { return x * 2; }
};
} // namespace testlib
`
	headerPath := filepath.Join(includeDir, "foo.hpp")
	if err := os.WriteFile(headerPath, []byte(headerContent), 0o644); err != nil {
		t.Fatal(err)
	}

	src := New(Config{
		RepoPath:    tmpDir,
		LibraryID:   "cpp/testlib",
		Name:        "TestLib",
		Description: "A test library",
		SourceURL:   "https://github.com/test/testlib",
		Version:     "1.0.0",
		IncludeDirs: []string{"include"},
		Ref:         "v1.0.0",
	})

	// Test ID
	if src.ID() != "cpp/testlib" {
		t.Errorf("ID: expected 'cpp/testlib', got %q", src.ID())
	}

	// Test Meta
	meta := src.Meta()
	if meta.Language != "cpp" {
		t.Errorf("Meta.Language: expected 'cpp', got %q", meta.Language)
	}
	if meta.Name != "TestLib" {
		t.Errorf("Meta.Name: expected 'TestLib', got %q", meta.Name)
	}

	// Test DiscoverEntities
	ctx := context.Background()
	entities, err := src.DiscoverEntities(ctx, nil)
	if err != nil {
		t.Fatalf("DiscoverEntities failed: %v", err)
	}
	if len(entities) == 0 {
		t.Fatal("DiscoverEntities returned empty list")
	}

	// Find the Foo entity
	var fooID string
	for _, e := range entities {
		if strings.HasSuffix(e, "#testlib::Foo") {
			fooID = e
			break
		}
	}
	if fooID == "" {
		t.Fatalf("testlib::Foo not found in entities: %v", entities)
	}

	// Test ParseEntity
	content, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatal(err)
	}

	entity, methodIDs, err := src.ParseEntity(ctx, fooID, content)
	if err != nil {
		t.Fatalf("ParseEntity failed: %v", err)
	}
	if entity == nil {
		t.Fatal("ParseEntity returned nil entity")
	}
	if entity.Name != "testlib::Foo" {
		t.Errorf("entity.Name: expected 'testlib::Foo', got %q", entity.Name)
	}
	if entity.Kind != "class" {
		t.Errorf("entity.Kind: expected 'class', got %q", entity.Kind)
	}
	if !strings.Contains(entity.Description, "test class") {
		t.Errorf("entity.Description should contain 'test class', got %q", entity.Description)
	}
	if len(methodIDs) == 0 {
		t.Error("expected method IDs for Foo class")
	}

	// Test ParseMethod
	var barID string
	for _, mid := range methodIDs {
		if strings.Contains(mid, "bar") {
			barID = mid
			break
		}
	}
	if barID == "" {
		t.Fatalf("bar method ID not found in %v", methodIDs)
	}

	method, err := src.ParseMethod(ctx, barID, content)
	if err != nil {
		t.Fatalf("ParseMethod failed: %v", err)
	}
	if method == nil {
		t.Fatal("ParseMethod returned nil")
	}
	if method.Name != "bar" {
		t.Errorf("method.Name: expected 'bar', got %q", method.Name)
	}

	// Test ParseSourceCode
	code, err := src.ParseSourceCode(fooID, content)
	if err != nil {
		t.Fatalf("ParseSourceCode failed: %v", err)
	}
	if code == "" {
		t.Error("ParseSourceCode returned empty string")
	}
	if !strings.Contains(code, "class Foo") {
		t.Errorf("ParseSourceCode should contain 'class Foo', got %q", code)
	}
}

func TestSource_DetectWrapper(t *testing.T) {
	s := New(Config{
		RepoPath:  t.TempDir(),
		LibraryID: "cpp/test",
	})

	method := &source.Method{
		SourceCode: `int wrapper(int x) { return target(x); }`,
	}
	isW, target, kind := s.DetectWrapper(method)
	if !isW {
		t.Error("expected wrapper detection")
	}
	if target != "target" {
		t.Errorf("expected target 'target', got %q", target)
	}
	if kind != "function" {
		t.Errorf("expected kind 'function', got %q", kind)
	}
}

func TestSlugFromQualifiedName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"std::vector", "std-vector"},
		{"boost::asio::io_context", "boost-asio-io_context"},
		{"MyClass", "myclass"},
		{"ns::Container<T>", "ns-container-t"},
	}

	for _, tt := range tests {
		got := slugFromQualifiedName(tt.input)
		if got != tt.expected {
			t.Errorf("slugFromQualifiedName(%q): expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestFindPrecedingDoc(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		pos      int // position of declaration
		wantDoc  bool
		contains string
	}{
		{
			name:     "block comment",
			src:      "/** A doc comment */\nclass Foo {};",
			pos:      21, // start of 'class'
			wantDoc:  true,
			contains: "A doc comment",
		},
		{
			name:     "triple slash",
			src:      "/// A line comment\nclass Foo {};",
			pos:      19, // start of 'class'
			wantDoc:  true,
			contains: "A line comment",
		},
		{
			name:    "no comment",
			src:     "class Foo {};",
			pos:     0,
			wantDoc: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := findPrecedingDoc([]byte(tt.src), tt.pos)
			if tt.wantDoc && doc == "" {
				t.Error("expected doc comment but got empty string")
			}
			if !tt.wantDoc && doc != "" {
				t.Errorf("expected no doc comment but got %q", doc)
			}
			if tt.contains != "" && !strings.Contains(doc, tt.contains) {
				t.Errorf("doc should contain %q, got %q", tt.contains, doc)
			}
		})
	}
}

func TestOperatorOverloads(t *testing.T) {
	src := `
class Vec2 {
public:
    double x, y;
    Vec2 operator+(const Vec2& rhs) const {
        return Vec2{x + rhs.x, y + rhs.y};
    }
    bool operator==(const Vec2& rhs) const {
        return x == rhs.x && y == rhs.y;
    }
};
`
	analysis := parseFile([]byte(src))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Classes) == 0 {
		t.Fatal("no classes found")
	}

	cls := &analysis.Classes[0]
	if len(cls.Methods) < 2 {
		t.Fatalf("expected at least 2 operator methods, got %d", len(cls.Methods))
	}

	// Check that operator names are extracted
	foundPlus := false
	foundEq := false
	for _, m := range cls.Methods {
		if strings.Contains(m.Name, "operator+") {
			foundPlus = true
		}
		if strings.Contains(m.Name, "operator==") {
			foundEq = true
		}
	}
	if !foundPlus {
		t.Error("operator+ not found")
	}
	if !foundEq {
		t.Error("operator== not found")
	}
}

func TestMultipleInheritance(t *testing.T) {
	src := `
class Base1 {};
class Base2 {};

class Derived : public Base1, protected Base2 {
};
`
	analysis := parseFile([]byte(src))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var derived *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].Name == "Derived" {
			derived = &analysis.Classes[i]
			break
		}
	}
	if derived == nil {
		t.Fatal("Derived class not found")
	}
	if len(derived.Bases) < 2 {
		t.Errorf("expected at least 2 base classes, got %d", len(derived.Bases))
	}
}

func TestNestedNamespace(t *testing.T) {
	src := `
namespace outer {
namespace inner {
class Thing {};
} // namespace inner
} // namespace outer
`
	analysis := parseFile([]byte(src))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var thing *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].Name == "Thing" {
			thing = &analysis.Classes[i]
			break
		}
	}
	if thing == nil {
		t.Fatal("Thing class not found")
	}
	if thing.QualifiedName != "outer::inner::Thing" {
		t.Errorf("expected 'outer::inner::Thing', got %q", thing.QualifiedName)
	}
}
