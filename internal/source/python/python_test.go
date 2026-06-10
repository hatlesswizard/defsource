package python

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/docparser/pydoc"
	"github.com/hatlesswizard/defsource/internal/source"
)

// --- Fixture Python code ---

const fixtureClassWithMethods = `
class HttpResponse:
    """An HTTP response class.

    This class represents an HTTP response with status code and headers.

    Args:
        content (str): The response content.
        status (int, optional): The HTTP status code.
    """

    status_code: int
    content: str

    def __init__(self, content: str = "", status: int = 200) -> None:
        """Initialize the response.

        Args:
            content (str): The body content.
            status (int): HTTP status code.
        """
        self.content = content
        self.status_code = status

    def get_status(self) -> int:
        """Return the status code."""
        return self.status_code

    @property
    def charset(self) -> str:
        """The response charset."""
        return self._charset

    def _internal_method(self):
        """This is private."""
        pass
`

const fixtureFunction = `
def get_object_or_404(klass, *args, **kwargs):
    """Get an object or raise Http404.

    Args:
        klass (Model): The model class to query.
        *args: Positional arguments for the queryset filter.
        **kwargs: Keyword arguments for the queryset filter.

    Returns:
        Model: The found object.

    Raises:
        Http404: If no object is found.
    """
    queryset = _get_queryset(klass)
    try:
        return queryset.get(*args, **kwargs)
    except queryset.model.DoesNotExist:
        raise Http404("No %s matches the given query." % klass.__name__)
`

const fixtureAsyncFunction = `
async def fetch_data(url: str, timeout: float = 30.0) -> dict:
    """Fetch data from a URL asynchronously.

    Args:
        url (str): The URL to fetch from.
        timeout (float, optional): Request timeout in seconds.

    Returns:
        dict: The response data.
    """
    async with aiohttp.ClientSession() as session:
        async with session.get(url, timeout=timeout) as response:
            return await response.json()
`

const fixtureDataclass = `
from dataclasses import dataclass

@dataclass
class Config:
    """Application configuration.

    Attributes:
        host (str): The server hostname.
        port (int): The server port.
        debug (bool): Whether debug mode is enabled.
    """
    host: str = "localhost"
    port: int = 8080
    debug: bool = False
`

const fixtureProtocol = `
from typing import Protocol

class Serializer(Protocol):
    """Protocol for serializable objects."""

    def serialize(self) -> bytes:
        """Serialize the object to bytes."""
        ...

    def deserialize(self, data: bytes) -> None:
        """Deserialize from bytes."""
        ...
`

const fixtureEnum = `
from enum import Enum

class Color(Enum):
    """Color enumeration."""
    RED = 1
    GREEN = 2
    BLUE = 3
`

const fixtureAllExports = `
__all__ = [
    "PublicClass",
    "public_function",
]

class PublicClass:
    """This is public."""
    pass

class _PrivateClass:
    """This is private."""
    pass

def public_function():
    """This is public."""
    pass

def _private_function():
    """This is private."""
    pass

def unlisted_function():
    """Not in __all__, so private."""
    pass
`

const fixtureWrapper = `
def get_cache(key, default=None):
    """Get a value from the cache."""
    return cache.get(key, default)

def simple_wrapper(x):
    """A simple wrapper."""
    return actual_function(x)

def not_a_wrapper(x, y):
    """This does real work."""
    result = compute(x)
    return result + y
`

const fixtureDecorators = `
import functools

class MyClass:
    """A class with decorated methods."""

    @classmethod
    def create(cls, name: str) -> "MyClass":
        """Create a new instance."""
        return cls(name)

    @staticmethod
    def validate(value: str) -> bool:
        """Validate a value."""
        return len(value) > 0

    @abstractmethod
    def process(self) -> None:
        """Process the data."""
        ...
`

const fixtureTypeAnnotations = `
from typing import List, Optional, Dict, Tuple, Union

def complex_function(
    items: List[str],
    mapping: Dict[str, int],
    optional_val: Optional[str] = None,
    *args: Tuple[int, ...],
    **kwargs: Dict[str, Any],
) -> Union[str, None]:
    """A function with complex type annotations.

    Args:
        items (List[str]): A list of strings.
        mapping (Dict[str, int]): A string-to-int mapping.
        optional_val (Optional[str], optional): An optional value.
        *args (Tuple[int, ...]): Extra positional args.
        **kwargs (Dict[str, Any]): Extra keyword args.

    Returns:
        Union[str, None]: The result or None.
    """
    return None
`

// --- Tests ---

func TestParseFile_ClassWithMethods(t *testing.T) {
	analysis := parseFile([]byte(fixtureClassWithMethods))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(analysis.Classes))
	}

	cls := analysis.Classes[0]
	if cls.Name != "HttpResponse" {
		t.Errorf("expected class name HttpResponse, got %q", cls.Name)
	}

	if cls.Docstring == "" {
		t.Error("expected non-empty docstring")
	}

	if !strings.Contains(cls.Docstring, "HTTP response class") {
		t.Errorf("docstring should contain 'HTTP response class', got %q", cls.Docstring)
	}

	// Check methods
	if len(cls.Methods) < 3 {
		t.Fatalf("expected at least 3 methods, got %d", len(cls.Methods))
	}

	// Check __init__
	var initMethod *methodDef
	for i := range cls.Methods {
		if cls.Methods[i].Name == "__init__" {
			initMethod = &cls.Methods[i]
			break
		}
	}
	if initMethod == nil {
		t.Fatal("__init__ method not found")
	}

	// __init__ should have self, content, status params
	if len(initMethod.Params) < 3 {
		t.Errorf("expected at least 3 params for __init__, got %d", len(initMethod.Params))
	}

	// Check that self is first param
	if len(initMethod.Params) > 0 && initMethod.Params[0].Name != "self" {
		t.Errorf("first param should be 'self', got %q", initMethod.Params[0].Name)
	}

	// Check content param type
	for _, p := range initMethod.Params {
		if p.Name == "content" {
			if p.Type != "str" {
				t.Errorf("content param type should be 'str', got %q", p.Type)
			}
			if p.Default != `""` {
				t.Errorf("content param default should be empty string, got %q", p.Default)
			}
		}
	}

	// Check return type of get_status
	var getStatus *methodDef
	for i := range cls.Methods {
		if cls.Methods[i].Name == "get_status" {
			getStatus = &cls.Methods[i]
			break
		}
	}
	if getStatus == nil {
		t.Fatal("get_status method not found")
	}
	if getStatus.ReturnType != "int" {
		t.Errorf("get_status return type should be 'int', got %q", getStatus.ReturnType)
	}
}

func TestParseFile_Function(t *testing.T) {
	analysis := parseFile([]byte(fixtureFunction))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(analysis.Functions))
	}

	fn := analysis.Functions[0]
	if fn.Name != "get_object_or_404" {
		t.Errorf("expected function name 'get_object_or_404', got %q", fn.Name)
	}

	if fn.Docstring == "" {
		t.Error("expected non-empty docstring")
	}

	// Check params: klass, *args, **kwargs
	if len(fn.Params) != 3 {
		t.Fatalf("expected 3 params, got %d: %+v", len(fn.Params), fn.Params)
	}

	if fn.Params[0].Name != "klass" {
		t.Errorf("first param should be 'klass', got %q", fn.Params[0].Name)
	}
	if !fn.Params[1].Star {
		t.Errorf("second param should be *args (Star=true)")
	}
	if fn.Params[1].Name != "args" {
		t.Errorf("second param name should be 'args', got %q", fn.Params[1].Name)
	}
	if !fn.Params[2].StarStar {
		t.Errorf("third param should be **kwargs (StarStar=true)")
	}
	if fn.Params[2].Name != "kwargs" {
		t.Errorf("third param name should be 'kwargs', got %q", fn.Params[2].Name)
	}
}

func TestParseFile_AsyncFunction(t *testing.T) {
	analysis := parseFile([]byte(fixtureAsyncFunction))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(analysis.Functions))
	}

	fn := analysis.Functions[0]
	if fn.Name != "fetch_data" {
		t.Errorf("expected function name 'fetch_data', got %q", fn.Name)
	}
	if !fn.Async {
		t.Error("expected async=true for async function")
	}
	if fn.ReturnType != "dict" {
		t.Errorf("expected return type 'dict', got %q", fn.ReturnType)
	}

	// Check params
	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Params))
	}
	if fn.Params[0].Name != "url" || fn.Params[0].Type != "str" {
		t.Errorf("first param should be url: str, got %q: %q", fn.Params[0].Name, fn.Params[0].Type)
	}
	if fn.Params[1].Name != "timeout" || fn.Params[1].Default != "30.0" {
		t.Errorf("second param should be timeout with default 30.0, got %q default %q",
			fn.Params[1].Name, fn.Params[1].Default)
	}
}

func TestParseFile_Dataclass(t *testing.T) {
	analysis := parseFile([]byte(fixtureDataclass))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(analysis.Classes))
	}

	cls := analysis.Classes[0]
	if cls.Name != "Config" {
		t.Errorf("expected class name 'Config', got %q", cls.Name)
	}

	// Check decorators
	hasDataclass := false
	for _, dec := range cls.Decorators {
		if dec == "dataclass" {
			hasDataclass = true
			break
		}
	}
	if !hasDataclass {
		t.Errorf("expected @dataclass decorator, got %v", cls.Decorators)
	}

	// Check kind
	kind := classKind(&cls)
	if kind != "dataclass" {
		t.Errorf("expected kind 'dataclass', got %q", kind)
	}
}

func TestParseFile_Protocol(t *testing.T) {
	analysis := parseFile([]byte(fixtureProtocol))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(analysis.Classes))
	}

	cls := analysis.Classes[0]
	if cls.Name != "Serializer" {
		t.Errorf("expected class name 'Serializer', got %q", cls.Name)
	}

	// Check bases
	hasProtocol := false
	for _, base := range cls.Bases {
		if base == "Protocol" {
			hasProtocol = true
			break
		}
	}
	if !hasProtocol {
		t.Errorf("expected Protocol in bases, got %v", cls.Bases)
	}

	kind := classKind(&cls)
	if kind != "protocol" {
		t.Errorf("expected kind 'protocol', got %q", kind)
	}

	// Check methods
	if len(cls.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(cls.Methods))
	}
}

func TestParseFile_Enum(t *testing.T) {
	analysis := parseFile([]byte(fixtureEnum))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(analysis.Classes))
	}

	cls := analysis.Classes[0]
	if cls.Name != "Color" {
		t.Errorf("expected class name 'Color', got %q", cls.Name)
	}

	hasEnum := false
	for _, base := range cls.Bases {
		if base == "Enum" {
			hasEnum = true
			break
		}
	}
	if !hasEnum {
		t.Errorf("expected Enum in bases, got %v", cls.Bases)
	}

	kind := classKind(&cls)
	if kind != "enum" {
		t.Errorf("expected kind 'enum', got %q", kind)
	}
}

func TestAllExports_Filtering(t *testing.T) {
	allNames := parseAllExports([]byte(fixtureAllExports))
	if allNames == nil {
		t.Fatal("parseAllExports returned nil")
	}

	if !allNames["PublicClass"] {
		t.Error("PublicClass should be in __all__")
	}
	if !allNames["public_function"] {
		t.Error("public_function should be in __all__")
	}
	if allNames["_PrivateClass"] {
		t.Error("_PrivateClass should not be in __all__")
	}
	if allNames["unlisted_function"] {
		t.Error("unlisted_function should not be in __all__")
	}
}

func TestDiscovery_PublicFiltering(t *testing.T) {
	idx := emptyIndex()
	idx.allExports["test.py"] = map[string]bool{
		"PublicClass":     true,
		"public_function": true,
	}
	idx.definedClasses["PublicClass"] = "test.py"
	idx.definedClasses["_PrivateClass"] = "test.py"
	idx.definedFunctions["public_function"] = "test.py"
	idx.definedFunctions["_private_function"] = "test.py"
	idx.definedFunctions["unlisted_function"] = "test.py"

	entities := idx.buildEntityList()

	// Should only contain PublicClass and public_function
	found := map[string]bool{}
	for _, id := range entities {
		_, fragment := splitFragment(id)
		found[fragment] = true
	}

	if !found["PublicClass"] {
		t.Error("PublicClass should be in entity list")
	}
	if !found["public_function"] {
		t.Error("public_function should be in entity list")
	}
	if found["_PrivateClass"] {
		t.Error("_PrivateClass should not be in entity list")
	}
	if found["_private_function"] {
		t.Error("_private_function should not be in entity list")
	}
	if found["unlisted_function"] {
		t.Error("unlisted_function should not be in entity list (not in __all__)")
	}
}

func TestWrapperDetection_SimpleWrapper(t *testing.T) {
	src := []byte(`    return actual_function(x)`)
	idx := emptyIndex()
	idx.definedFunctions["actual_function"] = "some_file.py"

	isWrapper, target, kind := detectWrapper(src, idx)
	if !isWrapper {
		t.Error("expected wrapper detection")
	}
	if target != "actual_function" {
		t.Errorf("expected target 'actual_function', got %q", target)
	}
	if kind != "function" {
		t.Errorf("expected kind 'function', got %q", kind)
	}
}

func TestWrapperDetection_DelegateMethod(t *testing.T) {
	src := []byte(`    return self.delegate.get(key, default)`)
	idx := emptyIndex()

	isWrapper, target, kind := detectWrapper(src, idx)
	if !isWrapper {
		t.Error("expected wrapper detection for delegate method")
	}
	if target != "delegate.get" {
		t.Errorf("expected target 'delegate.get', got %q", target)
	}
	if kind != "delegate_method" {
		t.Errorf("expected kind 'delegate_method', got %q", kind)
	}
}

func TestWrapperDetection_NotAWrapper(t *testing.T) {
	src := []byte("    result = compute(x)\n    return result + y")
	idx := emptyIndex()

	isWrapper, _, _ := detectWrapper(src, idx)
	if isWrapper {
		t.Error("multi-statement body should not be detected as wrapper")
	}
}

func TestWrapperDetection_BuiltinNotWrapper(t *testing.T) {
	src := []byte(`    return len(items)`)
	idx := emptyIndex()

	isWrapper, _, _ := detectWrapper(src, idx)
	if isWrapper {
		t.Error("call to builtin 'len' should not be detected as wrapper")
	}
}

func TestParseFile_Decorators(t *testing.T) {
	analysis := parseFile([]byte(fixtureDecorators))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(analysis.Classes))
	}

	cls := analysis.Classes[0]
	if len(cls.Methods) != 3 {
		t.Fatalf("expected 3 methods, got %d", len(cls.Methods))
	}

	// Check classmethod
	var createMethod *methodDef
	for i := range cls.Methods {
		if cls.Methods[i].Name == "create" {
			createMethod = &cls.Methods[i]
			break
		}
	}
	if createMethod == nil {
		t.Fatal("create method not found")
	}
	hasClassmethod := false
	for _, dec := range createMethod.Decorators {
		if dec == "classmethod" {
			hasClassmethod = true
			break
		}
	}
	if !hasClassmethod {
		t.Errorf("create should have @classmethod decorator, got %v", createMethod.Decorators)
	}
}

func TestParseFile_TypeAnnotations(t *testing.T) {
	analysis := parseFile([]byte(fixtureTypeAnnotations))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(analysis.Functions))
	}

	fn := analysis.Functions[0]
	if fn.Name != "complex_function" {
		t.Errorf("expected function name 'complex_function', got %q", fn.Name)
	}

	// Should have items, mapping, optional_val, *args, **kwargs
	if len(fn.Params) < 5 {
		t.Fatalf("expected at least 5 params, got %d: %+v", len(fn.Params), fn.Params)
	}

	// Check items param type
	if fn.Params[0].Name != "items" {
		t.Errorf("first param should be 'items', got %q", fn.Params[0].Name)
	}
	if fn.Params[0].Type != "List[str]" {
		t.Errorf("items type should be 'List[str]', got %q", fn.Params[0].Type)
	}

	// Check return type
	if fn.ReturnType != "Union[str, None]" {
		t.Errorf("return type should be 'Union[str, None]', got %q", fn.ReturnType)
	}
}

func TestParseEntity_Class(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		LibraryID:   "/python/test",
		Name:        "Test",
		Description: "Test library",
		SourceURL:   "https://github.com/test/test",
		TrustScore:  0.9,
	}
	src := New(tmpDir, cfg)

	filePath := filepath.Join(tmpDir, "views.py")
	entityID := filePath + "#HttpResponse"

	entity, methods, err := src.ParseEntity(context.Background(), entityID, []byte(fixtureClassWithMethods))
	if err != nil {
		t.Fatalf("ParseEntity failed: %v", err)
	}

	if entity.Name != "HttpResponse" {
		t.Errorf("expected name 'HttpResponse', got %q", entity.Name)
	}
	if entity.Kind != source.KindClass {
		t.Errorf("expected kind %q, got %q", source.KindClass, entity.Kind)
	}
	if entity.Description == "" {
		t.Error("expected non-empty description")
	}

	// Should have method IDs for public methods (not _internal_method)
	hasInit := false
	hasInternal := false
	for _, m := range methods {
		if strings.Contains(m, "__init__") {
			hasInit = true
		}
		if strings.Contains(m, "_internal_method") {
			hasInternal = true
		}
	}
	if !hasInit {
		t.Error("expected __init__ in method IDs")
	}
	if hasInternal {
		t.Error("_internal_method should be filtered out")
	}
}

func TestParseMethod_Init(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		LibraryID:   "/python/test",
		Name:        "Test",
		Description: "Test library",
		SourceURL:   "https://github.com/test/test",
		TrustScore:  0.9,
	}
	src := New(tmpDir, cfg)

	filePath := filepath.Join(tmpDir, "views.py")
	methodID := filePath + "#HttpResponse.__init__"

	method, err := src.ParseMethod(context.Background(), methodID, []byte(fixtureClassWithMethods))
	if err != nil {
		t.Fatalf("ParseMethod failed: %v", err)
	}

	if method.Name != "__init__" {
		t.Errorf("expected name '__init__', got %q", method.Name)
	}

	// Should not include 'self' in parameters
	for _, p := range method.Parameters {
		if p.Name == "self" {
			t.Error("'self' should be filtered from parameters")
		}
	}

	if len(method.Parameters) != 2 {
		t.Errorf("expected 2 parameters (content, status), got %d", len(method.Parameters))
	}
}

func TestParseSourceCode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		LibraryID:   "/python/test",
		Name:        "Test",
		Description: "Test library",
		SourceURL:   "https://github.com/test/test",
		TrustScore:  0.9,
	}
	src := New(tmpDir, cfg)

	content := []byte(fixtureClassWithMethods)
	filePath := filepath.Join(tmpDir, "views.py")

	// Extract class source
	code, err := src.ParseSourceCode(filePath+"#HttpResponse", content)
	if err != nil {
		t.Fatalf("ParseSourceCode for class failed: %v", err)
	}
	if !strings.Contains(code, "class HttpResponse") {
		t.Error("expected source code to contain class definition")
	}

	// Extract method source
	code, err = src.ParseSourceCode(filePath+"#HttpResponse.get_status", content)
	if err != nil {
		t.Fatalf("ParseSourceCode for method failed: %v", err)
	}
	if !strings.Contains(code, "get_status") {
		t.Error("expected source code to contain method name")
	}
}

func TestDiscovery_Integration(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "mylib")
	os.MkdirAll(srcDir, 0o755)

	// Write a Python file
	pyContent := []byte(`
__all__ = ["MyClass", "my_function"]

class MyClass:
    """A public class."""
    def method(self):
        pass

class _InternalClass:
    """An internal class."""
    pass

def my_function():
    """A public function."""
    pass

def _helper():
    """An internal helper."""
    pass
`)
	os.WriteFile(filepath.Join(srcDir, "core.py"), pyContent, 0o644)

	// Write a test file (should be skipped)
	testContent := []byte(`
def test_something():
    pass
`)
	os.WriteFile(filepath.Join(srcDir, "test_core.py"), testContent, 0o644)

	// Build index
	idx, err := buildCodebaseIndex(tmpDir, []string{srcDir})
	if err != nil {
		t.Fatalf("buildCodebaseIndex failed: %v", err)
	}

	// Check that classes and functions were indexed
	if !idx.HasClass("MyClass") {
		t.Error("MyClass should be indexed")
	}
	if !idx.HasFunction("my_function") {
		t.Error("my_function should be indexed")
	}

	// Build entity list (should respect __all__)
	entities := idx.buildEntityList()

	found := map[string]bool{}
	for _, id := range entities {
		_, fragment := splitFragment(id)
		found[fragment] = true
	}

	if !found["MyClass"] {
		t.Error("MyClass should be in entity list")
	}
	if !found["my_function"] {
		t.Error("my_function should be in entity list")
	}
	if found["_InternalClass"] {
		t.Error("_InternalClass should not be in entity list")
	}
	if found["_helper"] {
		t.Error("_helper should not be in entity list")
	}
}

func TestSkipFiles(t *testing.T) {
	tests := []struct {
		name string
		skip bool
	}{
		{"core.py", false},
		{"views.py", false},
		{"test_views.py", true},
		{"views_test.py", true},
		{"conftest.py", true},
		{"setup.py", true},
		{"0001_initial.py", true},
		{"models.py", false},
		{"README.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipFile(tt.name)
			if got != tt.skip {
				t.Errorf("shouldSkipFile(%q) = %v, want %v", tt.name, got, tt.skip)
			}
		})
	}
}

func TestSkipDirs(t *testing.T) {
	tests := []struct {
		name string
		skip bool
	}{
		{"__pycache__", true},
		{".git", true},
		{"tests", true},
		{"migrations", true},
		{"views", false},
		{"models", false},
		{"venv", true},
		{".venv", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipDir(tt.name)
			if got != tt.skip {
				t.Errorf("shouldSkipDir(%q) = %v, want %v", tt.name, got, tt.skip)
			}
		})
	}
}

// Ensure the imports are used.
var _ = pydoc.New
var _ = source.KindClass
