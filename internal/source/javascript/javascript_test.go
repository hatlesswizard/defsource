package javascript

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
)

// testRepoDir creates a temporary directory structure for testing.
func testRepoDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestParseFile_ES6Class(t *testing.T) {
	src := []byte(`
/**
 * A sample class for testing.
 * @since 1.0.0
 */
class MyClass {
  /**
   * Create a new instance.
   * @param {string} name - The name.
   */
  constructor(name) {
    this.name = name;
  }

  /**
   * Get the greeting.
   * @returns {string} The greeting message.
   */
  greet() {
    return 'Hello, ' + this.name;
  }

  /**
   * A static factory method.
   * @param {string} name - The name.
   * @returns {MyClass} A new instance.
   */
  static create(name) {
    return new MyClass(name);
  }

  get fullName() {
    return this.name;
  }

  set fullName(value) {
    this.name = value;
  }
}

export default MyClass;
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Classes) == 0 {
		t.Fatal("no classes found")
	}

	cls := analysis.Classes[0]
	if cls.Name != "MyClass" {
		t.Errorf("class name = %q, want %q", cls.Name, "MyClass")
	}

	if cls.DocComment == "" {
		t.Error("class has no doc comment")
	}
	if !strings.Contains(cls.DocComment, "A sample class") {
		t.Errorf("doc comment = %q, want it to contain 'A sample class'", cls.DocComment)
	}

	// Should have: constructor, greet, create (static), get fullName, set fullName
	if len(cls.Methods) < 4 {
		t.Errorf("got %d methods, want at least 4", len(cls.Methods))
	}

	// Verify static method.
	var found bool
	for _, m := range cls.Methods {
		if m.Name == "create" && m.Static {
			found = true
			break
		}
	}
	if !found {
		t.Error("static method 'create' not found")
	}

	// Verify getter.
	found = false
	for _, m := range cls.Methods {
		if m.Name == "fullName" && m.IsGetter {
			found = true
			break
		}
	}
	if !found {
		t.Error("getter 'fullName' not found")
	}
}

func TestParseFile_FunctionDeclarations(t *testing.T) {
	src := []byte(`
/**
 * Adds two numbers.
 * @param {number} a - First number.
 * @param {number} b - Second number.
 * @returns {number} The sum.
 */
export function add(a, b) {
  return a + b;
}

/**
 * Multiplies numbers.
 * @param {...number} nums - Numbers to multiply.
 * @returns {number} The product.
 */
export function multiply(...nums) {
  return nums.reduce((acc, n) => acc * n, 1);
}
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Functions) < 2 {
		t.Fatalf("got %d functions, want at least 2", len(analysis.Functions))
	}

	var addFn *funcDef
	for i := range analysis.Functions {
		if analysis.Functions[i].Name == "add" {
			addFn = &analysis.Functions[i]
			break
		}
	}
	if addFn == nil {
		t.Fatal("function 'add' not found")
	}
	if !addFn.Exported {
		t.Error("function 'add' should be exported")
	}
	if addFn.DocComment == "" {
		t.Error("function 'add' has no doc comment")
	}
}

func TestParseFile_ArrowFunctions(t *testing.T) {
	src := []byte(`
/**
 * Squares a number.
 * @param {number} x - The number.
 * @returns {number} The square.
 */
export const square = (x) => x * x;

/**
 * Identity function.
 */
export const identity = x => x;
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Functions) < 2 {
		t.Fatalf("got %d functions, want at least 2", len(analysis.Functions))
	}

	var squareFn *funcDef
	for i := range analysis.Functions {
		if analysis.Functions[i].Name == "square" {
			squareFn = &analysis.Functions[i]
			break
		}
	}
	if squareFn == nil {
		t.Fatal("function 'square' not found")
	}
	if !squareFn.IsArrow {
		t.Error("function 'square' should be marked as arrow")
	}
	if !squareFn.Exported {
		t.Error("function 'square' should be exported")
	}
}

func TestParseFile_CommonJSExports(t *testing.T) {
	src := []byte(`
/**
 * A utility module.
 */
module.exports = {
  /**
   * Format a name.
   * @param {string} first - First name.
   * @param {string} last - Last name.
   * @returns {string} Formatted name.
   */
  formatName(first, last) {
    return first + ' ' + last;
  },

  /**
   * Parse a name.
   * @param {string} fullName - The full name.
   * @returns {Object} Parsed result.
   */
  parseName(fullName) {
    const parts = fullName.split(' ');
    return { first: parts[0], last: parts[1] };
  }
};
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Modules) == 0 {
		t.Fatal("no modules found")
	}

	mod := analysis.Modules[0]
	if !mod.Exported {
		t.Error("module should be exported")
	}
	if len(mod.Methods) < 2 {
		t.Errorf("got %d methods, want at least 2", len(mod.Methods))
	}
}

func TestParseFile_ExportsProperty(t *testing.T) {
	src := []byte(`/**
 * Helper function.
 * @param {string} input - The input.
 * @returns {string} The processed output.
 */
exports.helper = function(input) {
  return input.trim();
};

/**
 * Another helper.
 */
exports.another = (x) => x + 1;
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Functions) < 2 {
		t.Fatalf("got %d functions, want at least 2", len(analysis.Functions))
	}

	var helperFn *funcDef
	for i := range analysis.Functions {
		if analysis.Functions[i].Name == "helper" {
			helperFn = &analysis.Functions[i]
			break
		}
	}
	if helperFn == nil {
		t.Fatal("function 'helper' not found")
	}
	if !helperFn.Exported {
		t.Error("function 'helper' should be exported")
	}
}

func TestParseFile_PrototypeMethods(t *testing.T) {
	src := []byte(`
function EventEmitter() {
  this.listeners = {};
}

/**
 * Register an event listener.
 * @param {string} event - Event name.
 * @param {Function} handler - The handler.
 */
EventEmitter.prototype.on = function(event, handler) {
  if (!this.listeners[event]) {
    this.listeners[event] = [];
  }
  this.listeners[event].push(handler);
};

/**
 * Emit an event.
 * @param {string} event - Event name.
 * @param {...*} args - Arguments.
 */
EventEmitter.prototype.emit = function(event, ...args) {
  const handlers = this.listeners[event] || [];
  handlers.forEach(h => h(...args));
};

module.exports = EventEmitter;
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	// Should have EventEmitter class with prototype methods.
	var emitter *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].Name == "EventEmitter" {
			emitter = &analysis.Classes[i]
			break
		}
	}
	if emitter == nil {
		t.Fatal("EventEmitter class not found")
	}

	if len(emitter.Methods) < 2 {
		t.Errorf("got %d methods, want at least 2", len(emitter.Methods))
	}

	var onMethod *methodDef
	for i := range emitter.Methods {
		if emitter.Methods[i].Name == "on" {
			onMethod = &emitter.Methods[i]
			break
		}
	}
	if onMethod == nil {
		t.Fatal("method 'on' not found")
	}
	if onMethod.DocComment == "" {
		t.Error("method 'on' should have a doc comment")
	}
}

func TestDiscovery_SkipPatterns(t *testing.T) {
	files := map[string]string{
		"lib/utils.js":           "export function helper() { return 1; }",
		"lib/core.mjs":          "export class Core {}",
		"node_modules/dep/x.js": "export function dep() {}",
		"test/utils.test.js":    "export function testHelper() {}",
		"__tests__/foo.js":      "export function testFoo() {}",
		"lib/thing.spec.js":     "export function specThing() {}",
	}
	dir := testRepoDir(t, files)

	roots := []string{dir}
	idx, err := buildCodebaseIndex(dir, roots)
	if err != nil {
		t.Fatal(err)
	}

	// Should find helper and Core.
	if _, ok := idx.definedFunctions["helper"]; !ok {
		t.Error("function 'helper' not found in index")
	}
	if _, ok := idx.definedClasses["Core"]; !ok {
		t.Error("class 'Core' not found in index")
	}

	// Should NOT find things in node_modules, test dirs, or test files.
	if _, ok := idx.definedFunctions["dep"]; ok {
		t.Error("function 'dep' from node_modules should not be indexed")
	}
	if _, ok := idx.definedFunctions["testHelper"]; ok {
		t.Error("function 'testHelper' from test dir should not be indexed")
	}
	if _, ok := idx.definedFunctions["testFoo"]; ok {
		t.Error("function 'testFoo' from __tests__ dir should not be indexed")
	}
	if _, ok := idx.definedFunctions["specThing"]; ok {
		t.Error("function 'specThing' from .spec.js should not be indexed")
	}
}

func TestWrapperDetection_SimpleReturn(t *testing.T) {
	src := []byte(`{
  return otherFunction(a, b);
}`)

	isWrapper, target, kind := detectWrapper(src, nil)
	if !isWrapper {
		t.Fatal("expected wrapper detection")
	}
	if target != "otherFunction" {
		t.Errorf("target = %q, want %q", target, "otherFunction")
	}
	if kind != "function" {
		t.Errorf("kind = %q, want %q", kind, "function")
	}
}

func TestWrapperDetection_MethodDelegation(t *testing.T) {
	src := []byte(`{
  return this.inner.doWork(args);
}`)

	isWrapper, target, kind := detectWrapper(src, nil)
	if !isWrapper {
		t.Fatal("expected wrapper detection")
	}
	if target != "this.inner.doWork" {
		t.Errorf("target = %q, want %q", target, "this.inner.doWork")
	}
	if kind != "method" {
		t.Errorf("kind = %q, want %q", kind, "method")
	}
}

func TestWrapperDetection_BuiltinSkip(t *testing.T) {
	src := []byte(`{
  return JSON.parse(str);
}`)

	builtins := map[string]bool{"JSON.parse": true}
	isWrapper, _, _ := detectWrapper(src, builtins)
	if isWrapper {
		t.Error("should not detect builtin as wrapper")
	}
}

func TestWrapperDetection_VoidWrapper(t *testing.T) {
	src := []byte(`{
  console.log(message);
}`)

	isWrapper, target, kind := detectWrapper(src, nil)
	if !isWrapper {
		t.Fatal("expected void wrapper detection")
	}
	if target != "console.log" {
		t.Errorf("target = %q, want %q", target, "console.log")
	}
	if kind != "module_method" {
		t.Errorf("kind = %q, want %q", kind, "module_method")
	}
}

func TestWrapperDetection_NotWrapper(t *testing.T) {
	src := []byte(`{
  const x = compute(a);
  const y = transform(x);
  return combine(x, y);
}`)

	isWrapper, _, _ := detectWrapper(src, nil)
	if isWrapper {
		t.Error("multi-statement body should not be a simple wrapper")
	}
}

func TestSourceInterface_FullFlow(t *testing.T) {
	files := map[string]string{
		"lib/router.js": `
/**
 * Router class for handling HTTP routes.
 * @since 2.0.0
 */
export class Router {
  /**
   * Create a new Router.
   */
  constructor() {
    this.routes = [];
  }

  /**
   * Add a GET route.
   * @param {string} path - The route path.
   * @param {Function} handler - The route handler.
   * @returns {Router} This router for chaining.
   */
  get(path, handler) {
    this.routes.push({ method: 'GET', path, handler });
    return this;
  }

  /**
   * Add a POST route.
   * @param {string} path - The route path.
   * @param {Function} handler - The route handler.
   * @returns {Router} This router for chaining.
   */
  post(path, handler) {
    this.routes.push({ method: 'POST', path, handler });
    return this;
  }
}
`,
		"lib/helpers.js": `
/**
 * Format a URL path.
 * @param {string} base - The base path.
 * @param {string} path - The relative path.
 * @returns {string} The formatted URL.
 */
export function formatURL(base, path) {
  return base.replace(/\/$/, '') + '/' + path.replace(/^\//, '');
}
`,
	}
	dir := testRepoDir(t, files)

	s := New(dir, WithConfig(LibraryConfig{
		ID:         "/javascript/test-router",
		Name:       "Test Router",
		SourceURL:  "https://github.com/test/router",
		Version:    "2.0.0",
		BlobRef:    "v2.0.0",
		SourceDirs: []string{"lib"},
	}))

	// Verify interface compliance.
	var _ source.Source = s

	if s.ID() != "/javascript/test-router" {
		t.Errorf("ID() = %q, want %q", s.ID(), "/javascript/test-router")
	}

	meta := s.Meta()
	if meta.Language != "javascript" {
		t.Errorf("Meta().Language = %q, want %q", meta.Language, "javascript")
	}
	if meta.Version != "2.0.0" {
		t.Errorf("Meta().Version = %q, want %q", meta.Version, "2.0.0")
	}

	// Discover entities.
	ctx := context.Background()
	ids, err := s.DiscoverEntities(ctx, nil)
	if err != nil {
		t.Fatalf("DiscoverEntities: %v", err)
	}

	if len(ids) < 2 {
		t.Fatalf("got %d entities, want at least 2 (Router class + formatURL function)", len(ids))
	}

	// Find the Router entity.
	var routerID string
	for _, id := range ids {
		if strings.HasSuffix(id, "#Router") {
			routerID = id
			break
		}
	}
	if routerID == "" {
		t.Fatal("Router entity not found in discovered IDs")
	}

	// Parse the Router entity.
	content, err := os.ReadFile(entityIDToPath(routerID))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	entity, methodURLs, err := s.ParseEntity(ctx, routerID, content)
	if err != nil {
		t.Fatalf("ParseEntity: %v", err)
	}

	if entity.Name != "Router" {
		t.Errorf("entity.Name = %q, want %q", entity.Name, "Router")
	}
	if entity.Kind != source.KindClass {
		t.Errorf("entity.Kind = %q, want %q", entity.Kind, source.KindClass)
	}
	if entity.Description == "" {
		t.Error("entity.Description is empty")
	}
	if !strings.Contains(entity.Description, "Router class") {
		t.Errorf("entity.Description = %q, want it to contain 'Router class'", entity.Description)
	}

	// Should have method URLs for constructor, get, post.
	if len(methodURLs) < 3 {
		t.Errorf("got %d method URLs, want at least 3", len(methodURLs))
	}

	// Parse a method.
	var getMethodID string
	for _, mu := range methodURLs {
		if strings.HasSuffix(mu, ".get") {
			getMethodID = mu
			break
		}
	}
	if getMethodID == "" {
		t.Fatal("method URL for 'get' not found")
	}

	method, err := s.ParseMethod(ctx, getMethodID, content)
	if err != nil {
		t.Fatalf("ParseMethod: %v", err)
	}

	if method.Name != "get" {
		t.Errorf("method.Name = %q, want %q", method.Name, "get")
	}
	if len(method.Parameters) < 2 {
		t.Errorf("got %d parameters, want at least 2", len(method.Parameters))
	}
	if method.ReturnType != "Router" {
		t.Errorf("method.ReturnType = %q, want %q", method.ReturnType, "Router")
	}
}

func TestParseFile_DestructuredParams(t *testing.T) {
	src := []byte(`
/**
 * Configure the app.
 * @param {Object} options - Configuration options.
 * @param {number} options.port - The port number.
 * @param {string} options.host - The hostname.
 */
export function configure({ port, host }) {
  return { port, host };
}
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Functions) == 0 {
		t.Fatal("no functions found")
	}

	fn := analysis.Functions[0]
	if fn.Name != "configure" {
		t.Errorf("function name = %q, want %q", fn.Name, "configure")
	}

	// The signature should contain the destructured parameter.
	if !strings.Contains(fn.Signature, "{") {
		t.Errorf("signature %q should contain destructured params", fn.Signature)
	}
}

func TestParseFile_DefaultParams(t *testing.T) {
	src := []byte(`
/**
 * Create a config.
 * @param {number} [port=3000] - The port.
 * @param {string} [host='localhost'] - The host.
 */
export function createConfig(port = 3000, host = 'localhost') {
  return { port, host };
}
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Functions) == 0 {
		t.Fatal("no functions found")
	}

	fn := analysis.Functions[0]
	if fn.Name != "createConfig" {
		t.Errorf("function name = %q, want %q", fn.Name, "createConfig")
	}

	// Signature should contain default values.
	if !strings.Contains(fn.Signature, "3000") {
		t.Errorf("signature %q should contain default value 3000", fn.Signature)
	}
}

func TestJSDocIntegration(t *testing.T) {
	files := map[string]string{
		"lib/math.js": `
/**
 * Calculate the hypotenuse of a right triangle.
 *
 * Uses the Pythagorean theorem to compute the result.
 *
 * @param {number} a - Length of side a.
 * @param {number} b - Length of side b.
 * @returns {number} The length of the hypotenuse.
 * @since 1.2.0
 * @see Math.sqrt
 *
 * @example
 * hypotenuse(3, 4) // => 5
 */
export function hypotenuse(a, b) {
  return Math.sqrt(a * a + b * b);
}
`,
	}
	dir := testRepoDir(t, files)

	s := New(dir, WithConfig(LibraryConfig{
		ID:         "/javascript/test-math",
		Name:       "Test Math",
		SourceURL:  "https://github.com/test/math",
		Version:    "1.2.0",
		BlobRef:    "v1.2.0",
		SourceDirs: []string{"lib"},
	}))

	ctx := context.Background()
	ids, err := s.DiscoverEntities(ctx, nil)
	if err != nil {
		t.Fatalf("DiscoverEntities: %v", err)
	}

	if len(ids) == 0 {
		t.Fatal("no entities discovered")
	}

	// The function is a top-level export, so ParseEntity should work.
	content, err := os.ReadFile(entityIDToPath(ids[0]))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	entity, _, err := s.ParseEntity(ctx, ids[0], content)
	if err != nil {
		t.Fatalf("ParseEntity: %v", err)
	}

	if entity.Name != "hypotenuse" {
		t.Errorf("entity.Name = %q, want %q", entity.Name, "hypotenuse")
	}
	if !strings.Contains(entity.Description, "Pythagorean theorem") {
		t.Errorf("description should mention Pythagorean theorem, got: %s", entity.Description)
	}
}

func TestParseMethodFragment(t *testing.T) {
	tests := []struct {
		fragment string
		class    string
		method   string
		ok       bool
	}{
		{"Router.get", "Router", "get", true},
		{"EventEmitter.prototype.on", "EventEmitter", "on", true},
		{"utils.formatName", "utils", "formatName", true},
		{"singleName", "", "", false},
		{"", "", "", false},
	}

	for _, tc := range tests {
		cls, meth, ok := parseMethodFragment(tc.fragment)
		if ok != tc.ok {
			t.Errorf("parseMethodFragment(%q): ok = %v, want %v", tc.fragment, ok, tc.ok)
			continue
		}
		if cls != tc.class {
			t.Errorf("parseMethodFragment(%q): class = %q, want %q", tc.fragment, cls, tc.class)
		}
		if meth != tc.method {
			t.Errorf("parseMethodFragment(%q): method = %q, want %q", tc.fragment, meth, tc.method)
		}
	}
}
