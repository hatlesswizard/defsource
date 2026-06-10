//go:build sqlite_fts5 || fts5

package ruby

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFixture(%q): %v", name, err)
	}
	return data
}

func newTestSource(t *testing.T) *RubySource {
	t.Helper()
	return New(t.TempDir(), Config{
		LibraryID:   "/ruby/test",
		Name:        "Test Ruby Library",
		Description: "A test Ruby library",
		SourceURL:   "https://github.com/test/test",
		Version:     "1.0.0",
		TrustScore:  0.9,
		SourceRoots: []string{"lib"},
	})
}

// ---------------------------------------------------------------------------
// Test 1 — Construction and identity
// ---------------------------------------------------------------------------

func TestNew_ConstructsWithConfig(t *testing.T) {
	s := newTestSource(t)
	if s == nil {
		t.Fatal("New() returned nil")
	}

	if got := s.ID(); got != "/ruby/test" {
		t.Errorf("ID() = %q, want %q", got, "/ruby/test")
	}

	meta := s.Meta()
	if meta.Name != "Test Ruby Library" {
		t.Errorf("Meta().Name = %q, want %q", meta.Name, "Test Ruby Library")
	}
	if meta.Language != "ruby" {
		t.Errorf("Meta().Language = %q, want %q", meta.Language, "ruby")
	}
	if meta.TrustScore != 0.9 {
		t.Errorf("Meta().TrustScore = %v, want 0.9", meta.TrustScore)
	}
}

func TestNew_ImplementsSourceInterface(t *testing.T) {
	var _ source.Source = newTestSource(t)
}

func TestNew_IndexInitializedNonNil(t *testing.T) {
	s := newTestSource(t)
	if s.index == nil {
		t.Fatal("New() left s.index nil; want non-nil empty index")
	}
	if len(s.index.definedClasses) != 0 {
		t.Errorf("empty index has %d classes, want 0", len(s.index.definedClasses))
	}
}

// ---------------------------------------------------------------------------
// Test 2 — Parser: Class extraction
// ---------------------------------------------------------------------------

func TestParseFile_ExtractsClass(t *testing.T) {
	src := readFixture(t, "sample_class.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Classes) < 2 {
		t.Fatalf("expected at least 2 classes, got %d", len(analysis.Classes))
	}

	// Check ActiveRecord::Base
	var base *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].QualifiedName == "ActiveRecord::Base" {
			base = &analysis.Classes[i]
			break
		}
	}
	if base == nil {
		t.Fatal("ActiveRecord::Base not found")
	}

	if base.Name != "Base" {
		t.Errorf("Name = %q, want %q", base.Name, "Base")
	}
	if base.QualifiedName != "ActiveRecord::Base" {
		t.Errorf("QualifiedName = %q, want %q", base.QualifiedName, "ActiveRecord::Base")
	}
}

func TestParseFile_ExtractsSuperclass(t *testing.T) {
	src := []byte(`class Child < Parent
  def hello; end
end
`)
	analysis := parseFile(src)
	if analysis == nil || len(analysis.Classes) == 0 {
		t.Fatal("no classes found")
	}
	if analysis.Classes[0].Superclass != "Parent" {
		t.Errorf("Superclass = %q, want %q", analysis.Classes[0].Superclass, "Parent")
	}
}

// ---------------------------------------------------------------------------
// Test 3 — Parser: Module extraction
// ---------------------------------------------------------------------------

func TestParseFile_ExtractsModule(t *testing.T) {
	src := readFixture(t, "sample_module.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Modules) == 0 {
		t.Fatal("no modules found")
	}

	mod := analysis.Modules[0]
	if mod.Name != "Enumerable" {
		t.Errorf("Module Name = %q, want %q", mod.Name, "Enumerable")
	}
	if mod.QualifiedName != "Enumerable" {
		t.Errorf("Module QualifiedName = %q, want %q", mod.QualifiedName, "Enumerable")
	}
}

func TestParseFile_ModuleMethods(t *testing.T) {
	src := readFixture(t, "sample_module.rb")
	analysis := parseFile(src)
	if analysis == nil || len(analysis.Modules) == 0 {
		t.Fatal("no modules found")
	}

	mod := analysis.Modules[0]
	methodNames := make(map[string]bool)
	for _, m := range mod.Methods {
		methodNames[m.Name] = true
	}

	expected := []string{"first", "map", "select"}
	for _, name := range expected {
		if !methodNames[name] {
			t.Errorf("method %q not found in module", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 4 — Parser: Method parameters
// ---------------------------------------------------------------------------

func TestParseFile_MethodParams(t *testing.T) {
	src := readFixture(t, "sample_class.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	// Find ActiveRecord::Base
	var base *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].QualifiedName == "ActiveRecord::Base" {
			base = &analysis.Classes[i]
			break
		}
	}
	if base == nil {
		t.Fatal("ActiveRecord::Base not found")
	}

	// Find initialize method
	var initMethod *methodDef
	for i := range base.Methods {
		if base.Methods[i].Name == "initialize" {
			initMethod = &base.Methods[i]
			break
		}
	}
	if initMethod == nil {
		t.Fatal("initialize method not found")
	}

	if len(initMethod.Params) < 2 {
		t.Fatalf("expected at least 2 params for initialize, got %d", len(initMethod.Params))
	}

	// Check optional parameter
	found := false
	for _, p := range initMethod.Params {
		if p.Name == "attributes" && p.Default == "{}" {
			found = true
			break
		}
	}
	if !found {
		t.Error("optional parameter 'attributes' with default '{}' not found")
	}

	// Check block parameter
	foundBlock := false
	for _, p := range initMethod.Params {
		if p.Name == "block" && p.Block {
			foundBlock = true
			break
		}
	}
	if !foundBlock {
		t.Error("block parameter not found")
	}
}

func TestParseFile_SplatParams(t *testing.T) {
	src := readFixture(t, "sample_wrapper.rb")
	analysis := parseFile(src)
	if analysis == nil || len(analysis.Classes) == 0 {
		t.Fatal("no classes found")
	}

	cls := analysis.Classes[0]
	// Find forward method with splat
	var forwardMethod *methodDef
	for i := range cls.Methods {
		if cls.Methods[i].Name == "forward" {
			forwardMethod = &cls.Methods[i]
			break
		}
	}
	if forwardMethod == nil {
		t.Fatal("forward method not found")
	}

	if len(forwardMethod.Params) == 0 {
		t.Fatal("expected at least 1 param for forward")
	}
	if !forwardMethod.Params[0].Splat {
		t.Error("expected first param to be splat")
	}
}

// ---------------------------------------------------------------------------
// Test 5 — Parser: Visibility handling
// ---------------------------------------------------------------------------

func TestParseFile_Visibility(t *testing.T) {
	src := readFixture(t, "sample_class.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var base *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].QualifiedName == "ActiveRecord::Base" {
			base = &analysis.Classes[i]
			break
		}
	}
	if base == nil {
		t.Fatal("ActiveRecord::Base not found")
	}

	visibilityMap := make(map[string]string)
	for _, m := range base.Methods {
		visibilityMap[m.Name] = m.Visibility
	}

	// Public methods (before any visibility modifier)
	if v := visibilityMap["initialize"]; v != "public" {
		t.Errorf("initialize visibility = %q, want %q", v, "public")
	}
	if v := visibilityMap["save"]; v != "public" {
		t.Errorf("save visibility = %q, want %q", v, "public")
	}

	// Protected methods
	if v := visibilityMap["validate!"]; v != "protected" {
		t.Errorf("validate! visibility = %q, want %q", v, "protected")
	}

	// Private methods
	if v := visibilityMap["persist"]; v != "private" {
		t.Errorf("persist visibility = %q, want %q", v, "private")
	}
	if v := visibilityMap["assign_attributes"]; v != "private" {
		t.Errorf("assign_attributes visibility = %q, want %q", v, "private")
	}
}

// ---------------------------------------------------------------------------
// Test 6 — Parser: Attr accessor extraction
// ---------------------------------------------------------------------------

func TestParseFile_AttrAccessors(t *testing.T) {
	src := readFixture(t, "sample_class.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var base *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].QualifiedName == "ActiveRecord::Base" {
			base = &analysis.Classes[i]
			break
		}
	}
	if base == nil {
		t.Fatal("ActiveRecord::Base not found")
	}

	attrNames := make(map[string]string) // name -> kind
	for _, attr := range base.Attributes {
		attrNames[attr.Name] = attr.Kind
	}

	// attr_reader
	if kind, ok := attrNames["id"]; !ok {
		t.Error("attr_reader :id not found")
	} else if kind != "reader" {
		t.Errorf("id kind = %q, want %q", kind, "reader")
	}
	if kind, ok := attrNames["created_at"]; !ok {
		t.Error("attr_reader :created_at not found")
	} else if kind != "reader" {
		t.Errorf("created_at kind = %q, want %q", kind, "reader")
	}

	// attr_writer
	if kind, ok := attrNames["updated_at"]; !ok {
		t.Error("attr_writer :updated_at not found")
	} else if kind != "writer" {
		t.Errorf("updated_at kind = %q, want %q", kind, "writer")
	}

	// attr_accessor
	if kind, ok := attrNames["name"]; !ok {
		t.Error("attr_accessor :name not found")
	} else if kind != "accessor" {
		t.Errorf("name kind = %q, want %q", kind, "accessor")
	}
	if kind, ok := attrNames["email"]; !ok {
		t.Error("attr_accessor :email not found")
	} else if kind != "accessor" {
		t.Errorf("email kind = %q, want %q", kind, "accessor")
	}
}

// ---------------------------------------------------------------------------
// Test 7 — Parser: Module mixin tracking
// ---------------------------------------------------------------------------

func TestParseFile_ModuleMixins(t *testing.T) {
	src := readFixture(t, "sample_class.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var base *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].QualifiedName == "ActiveRecord::Base" {
			base = &analysis.Classes[i]
			break
		}
	}
	if base == nil {
		t.Fatal("ActiveRecord::Base not found")
	}

	// Check includes
	includeFound := false
	for _, inc := range base.Includes {
		if inc == "Comparable" {
			includeFound = true
			break
		}
	}
	if !includeFound {
		t.Errorf("include Comparable not found, includes = %v", base.Includes)
	}

	// Check extends
	extendFound := false
	for _, ext := range base.Extends {
		if ext == "ClassMethods" {
			extendFound = true
			break
		}
	}
	if !extendFound {
		t.Errorf("extend ClassMethods not found, extends = %v", base.Extends)
	}

	// Check prepends
	prependFound := false
	for _, pre := range base.Prepends {
		if pre == "Callbacks" {
			prependFound = true
			break
		}
	}
	if !prependFound {
		t.Errorf("prepend Callbacks not found, prepends = %v", base.Prepends)
	}
}

// ---------------------------------------------------------------------------
// Test 8 — Parser: YARD doc extraction
// ---------------------------------------------------------------------------

func TestParseFile_YARDDocs(t *testing.T) {
	src := readFixture(t, "sample_class.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var base *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].QualifiedName == "ActiveRecord::Base" {
			base = &analysis.Classes[i]
			break
		}
	}
	if base == nil {
		t.Fatal("ActiveRecord::Base not found")
	}

	// Check class doc
	if base.DocComment == "" {
		t.Error("class doc comment is empty")
	}
	if !strings.Contains(base.DocComment, "Base class for all models") {
		t.Errorf("class doc does not contain expected text, got: %q", base.DocComment)
	}

	// Find the save method and check its doc
	for _, m := range base.Methods {
		if m.Name == "save" {
			if m.DocComment == "" {
				t.Error("save method doc comment is empty")
			}
			if !strings.Contains(m.DocComment, "@return [Boolean]") {
				t.Errorf("save doc does not contain @return, got: %q", m.DocComment)
			}
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Test 9 — Wrapper detection
// ---------------------------------------------------------------------------

func TestDetectWrapper_SingleDelegation(t *testing.T) {
	src := []byte(`def fetch(key)
  @target.fetch(key)
end
`)
	idx := emptyIndex()
	isWrapper, target, kind := detectWrapper(src, idx)
	if !isWrapper {
		t.Error("expected wrapper detection for single delegation")
	}
	if !strings.Contains(target, "target") || !strings.Contains(target, "fetch") {
		t.Errorf("target = %q, expected to contain 'target' and 'fetch'", target)
	}
	if kind != "delegate" {
		t.Errorf("kind = %q, want %q", kind, "delegate")
	}
}

func TestDetectWrapper_SelfDelegation(t *testing.T) {
	src := []byte(`def process(data)
  self.internal_process(data)
end
`)
	idx := emptyIndex()
	isWrapper, target, kind := detectWrapper(src, idx)
	if !isWrapper {
		t.Error("expected wrapper detection for self delegation")
	}
	if target != "internal_process" {
		t.Errorf("target = %q, want %q", target, "internal_process")
	}
	if kind != "method" {
		t.Errorf("kind = %q, want %q", kind, "method")
	}
}

func TestDetectWrapper_NotWrapper_MultipleStatements(t *testing.T) {
	src := []byte(`def complex_method(x)
  y = transform(x)
  finalize(y)
end
`)
	idx := emptyIndex()
	isWrapper, _, _ := detectWrapper(src, idx)
	if isWrapper {
		t.Error("multi-statement method should NOT be detected as wrapper")
	}
}

func TestDetectWrapper_SplatForwarding(t *testing.T) {
	src := []byte(`def forward(*args)
  target(*args)
end
`)
	idx := emptyIndex()
	isWrapper, target, kind := detectWrapper(src, idx)
	if !isWrapper {
		t.Error("expected wrapper detection for splat forwarding")
	}
	if target != "target" {
		t.Errorf("target = %q, want %q", target, "target")
	}
	if kind != "function" {
		t.Errorf("kind = %q, want %q", kind, "function")
	}
}

// ---------------------------------------------------------------------------
// Test 10 — Singleton method handling
// ---------------------------------------------------------------------------

func TestParseFile_SingletonMethods(t *testing.T) {
	src := readFixture(t, "sample_singleton.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Classes) == 0 {
		t.Fatal("no classes found")
	}

	// Find Registry class
	var registry *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].Name == "Registry" {
			registry = &analysis.Classes[i]
			break
		}
	}
	if registry == nil {
		t.Fatal("Registry class not found")
	}

	// Check singleton methods from class << self
	singletonNames := make(map[string]bool)
	for _, m := range registry.SingletonMethods {
		singletonNames[m.Name] = true
	}

	if !singletonNames["instance"] {
		t.Error("singleton method 'instance' not found")
	}
	if !singletonNames["reset!"] {
		t.Error("singleton method 'reset!' not found")
	}
	if !singletonNames["configure"] {
		t.Error("singleton method 'configure' not found (def self.configure)")
	}

	// Check instance methods
	instanceNames := make(map[string]bool)
	for _, m := range registry.Methods {
		instanceNames[m.Name] = true
	}
	if !instanceNames["register"] {
		t.Error("instance method 'register' not found")
	}
}

// ---------------------------------------------------------------------------
// Test 11 — Namespaced class resolution
// ---------------------------------------------------------------------------

func TestParseFile_NamespacedClass(t *testing.T) {
	src := readFixture(t, "sample_class.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	// Check that we get properly qualified names
	foundBase := false
	foundRelation := false
	for _, cls := range analysis.Classes {
		if cls.QualifiedName == "ActiveRecord::Base" {
			foundBase = true
		}
		if cls.QualifiedName == "ActiveRecord::Relation" {
			foundRelation = true
		}
	}
	if !foundBase {
		t.Error("ActiveRecord::Base not found as qualified name")
	}
	if !foundRelation {
		t.Error("ActiveRecord::Relation not found as qualified name")
	}
}

// ---------------------------------------------------------------------------
// Test 12 — Discovery
// ---------------------------------------------------------------------------

func TestDiscovery_FindsEntities(t *testing.T) {
	// Set up a mini repo with lib/ directory
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a simple Ruby file
	content := []byte(`module MyLib
  class Client
    def initialize(url)
      @url = url
    end

    def get(path)
      request(:get, path)
    end
  end
end
`)
	if err := os.WriteFile(filepath.Join(libDir, "client.rb"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(tmpDir, Config{
		LibraryID:   "/ruby/mylib",
		Name:        "MyLib",
		Description: "Test",
		SourceURL:   "https://github.com/test/mylib",
		Version:     "1.0.0",
		TrustScore:  0.9,
		SourceRoots: []string{"lib"},
	})

	ids, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities error: %v", err)
	}

	if len(ids) == 0 {
		t.Fatal("DiscoverEntities returned 0 entities")
	}

	// Should find the module and the class
	foundModule := false
	foundClass := false
	for _, id := range ids {
		if strings.Contains(id, "MyLib") && !strings.Contains(id, "::") {
			foundModule = true
		}
		if strings.Contains(id, "MyLib::Client") {
			foundClass = true
		}
	}
	if !foundModule {
		t.Errorf("module MyLib not found in entities: %v", ids)
	}
	if !foundClass {
		t.Errorf("class MyLib::Client not found in entities: %v", ids)
	}
}

func TestDiscovery_SkipsTestDirs(t *testing.T) {
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "lib")
	specDir := filepath.Join(tmpDir, "lib", "spec")
	testDir := filepath.Join(tmpDir, "lib", "test")

	for _, d := range []string{libDir, specDir, testDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Real code
	if err := os.WriteFile(filepath.Join(libDir, "real.rb"), []byte(`class Real; end`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Test code (should be skipped)
	if err := os.WriteFile(filepath.Join(specDir, "real_spec.rb"), []byte(`class RealSpec; end`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "real_test.rb"), []byte(`class RealTest; end`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(tmpDir, Config{
		LibraryID:   "/ruby/test",
		Name:        "Test",
		Description: "Test",
		SourceURL:   "https://github.com/test/test",
		Version:     "1.0.0",
		TrustScore:  0.9,
		SourceRoots: []string{"lib"},
	})

	ids, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverEntities error: %v", err)
	}

	for _, id := range ids {
		if strings.Contains(id, "RealSpec") || strings.Contains(id, "RealTest") {
			t.Errorf("test entity should not be discovered: %s", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 13 — ParseEntity integration
// ---------------------------------------------------------------------------

func TestParseEntity_Class(t *testing.T) {
	src := readFixture(t, "sample_class.rb")
	s := newTestSource(t)

	entityID := filepath.Join("testdata", "sample_class.rb") + "#ActiveRecord::Base"
	entity, methods, err := s.ParseEntity(context.Background(), entityID, src)
	if err != nil {
		t.Fatalf("ParseEntity error: %v", err)
	}
	if entity == nil {
		t.Fatal("ParseEntity returned nil entity")
	}

	if entity.Kind != source.KindClass {
		t.Errorf("entity.Kind = %q, want %q", entity.Kind, source.KindClass)
	}
	if entity.Name != "ActiveRecord::Base" {
		t.Errorf("entity.Name = %q, want %q", entity.Name, "ActiveRecord::Base")
	}
	if len(methods) == 0 {
		t.Error("expected method IDs to be returned")
	}
	if len(entity.Properties) == 0 {
		t.Error("expected properties (from attr_*) to be populated")
	}
}

func TestParseEntity_Module(t *testing.T) {
	src := readFixture(t, "sample_module.rb")
	s := newTestSource(t)

	entityID := filepath.Join("testdata", "sample_module.rb") + "#Enumerable"
	entity, methods, err := s.ParseEntity(context.Background(), entityID, src)
	if err != nil {
		t.Fatalf("ParseEntity error: %v", err)
	}
	if entity == nil {
		t.Fatal("ParseEntity returned nil entity")
	}

	if entity.Kind != source.KindModule {
		t.Errorf("entity.Kind = %q, want %q", entity.Kind, source.KindModule)
	}
	if entity.Name != "Enumerable" {
		t.Errorf("entity.Name = %q, want %q", entity.Name, "Enumerable")
	}
	if len(methods) == 0 {
		t.Error("expected method IDs to be returned")
	}
}

// ---------------------------------------------------------------------------
// Test 14 — ParseMethod integration
// ---------------------------------------------------------------------------

func TestParseMethod(t *testing.T) {
	src := readFixture(t, "sample_module.rb")
	s := newTestSource(t)

	methodID := filepath.Join("testdata", "sample_module.rb") + "#Enumerable#first"
	method, err := s.ParseMethod(context.Background(), methodID, src)
	if err != nil {
		t.Fatalf("ParseMethod error: %v", err)
	}
	if method == nil {
		t.Fatal("ParseMethod returned nil")
	}

	if method.Name != "first" {
		t.Errorf("method.Name = %q, want %q", method.Name, "first")
	}
	if method.Signature == "" {
		t.Error("method.Signature is empty")
	}
	if !strings.Contains(method.Signature, "first") {
		t.Errorf("signature should contain method name, got: %q", method.Signature)
	}
}

// ---------------------------------------------------------------------------
// Test 15 — Constants
// ---------------------------------------------------------------------------

func TestParseFile_Constants(t *testing.T) {
	src := readFixture(t, "sample_class.rb")
	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	var base *classDef
	for i := range analysis.Classes {
		if analysis.Classes[i].QualifiedName == "ActiveRecord::Base" {
			base = &analysis.Classes[i]
			break
		}
	}
	if base == nil {
		t.Fatal("ActiveRecord::Base not found")
	}

	foundConst := false
	for _, c := range base.Constants {
		if c.Name == "POOL_SIZE" {
			foundConst = true
			if c.Value != "5" {
				t.Errorf("POOL_SIZE value = %q, want %q", c.Value, "5")
			}
			break
		}
	}
	if !foundConst {
		t.Error("constant POOL_SIZE not found")
	}
}
