package golang

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
)

func testdataPath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "testdata")
}

func readTestFile(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(testdataPath(t), name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// --- Source interface tests ---

func TestSourceID(t *testing.T) {
	s := New("/tmp/fake", Config{LibraryID: "go/stdlib"})
	if got := s.ID(); got != "go/stdlib" {
		t.Errorf("ID() = %q, want %q", got, "go/stdlib")
	}
}

func TestSourceMeta(t *testing.T) {
	s := New("/tmp/fake", Config{
		LibraryID:   "go/gin",
		Name:        "Gin",
		Description: "HTTP framework",
		SourceURL:   "https://github.com/gin-gonic/gin",
		Ref:         "v1.9.1",
	})
	meta := s.Meta()
	if meta.Language != "go" {
		t.Errorf("Language = %q, want %q", meta.Language, "go")
	}
	if meta.Name != "Gin" {
		t.Errorf("Name = %q, want %q", meta.Name, "Gin")
	}
	if meta.Version != "v1.9.1" {
		t.Errorf("Version = %q, want %q", meta.Version, "v1.9.1")
	}
}

func TestDetectWrapperNil(t *testing.T) {
	s := New("/tmp/fake", Config{LibraryID: "go/stdlib"})
	isW, name, kind := s.DetectWrapper(nil)
	if isW || name != "" || kind != "" {
		t.Errorf("DetectWrapper(nil) = (%v, %q, %q), want (false, \"\", \"\")", isW, name, kind)
	}
}

// --- Parser tests ---

func TestParseStructSimple(t *testing.T) {
	content := readTestFile(t, "struct_simple.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(parsed.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(parsed.Structs))
	}

	st := parsed.Structs[0]
	if st.Name != "Server" {
		t.Errorf("struct name = %q, want %q", st.Name, "Server")
	}

	if st.DocComment == "" {
		t.Error("struct doc comment is empty")
	}

	// Should have exported fields only when filtering (5 exported, 1 unexported)
	// Parser extracts all fields; filtering is done in buildStructEntity
	exportedCount := 0
	for _, f := range st.Fields {
		if isExported(f.Name) {
			exportedCount++
		}
	}
	if exportedCount < 5 {
		t.Errorf("expected at least 5 exported fields, got %d", exportedCount)
	}
}

func TestParseStructMethods(t *testing.T) {
	content := readTestFile(t, "struct_simple.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	// Should have methods with receiver type "Server"
	var exportedMethods int
	for _, m := range parsed.Methods {
		if m.ReceiverType == "Server" && isExported(m.Name) {
			exportedMethods++
		}
	}
	if exportedMethods != 2 {
		t.Errorf("expected 2 exported methods on Server, got %d", exportedMethods)
	}
}

func TestParseInterfaceSimple(t *testing.T) {
	content := readTestFile(t, "interface_simple.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(parsed.Interfaces) < 2 {
		t.Fatalf("expected at least 2 interfaces, got %d", len(parsed.Interfaces))
	}

	// Find Handler interface
	var handler *interfaceDef
	for i := range parsed.Interfaces {
		if parsed.Interfaces[i].Name == "Handler" {
			handler = &parsed.Interfaces[i]
			break
		}
	}
	if handler == nil {
		t.Fatal("Handler interface not found")
	}
	if len(handler.Methods) != 1 {
		t.Errorf("Handler should have 1 method, got %d", len(handler.Methods))
	}
	if handler.Methods[0].Name != "ServeHTTP" {
		t.Errorf("method name = %q, want %q", handler.Methods[0].Name, "ServeHTTP")
	}

	// Find ReadWriter with embedded interfaces
	var rw *interfaceDef
	for i := range parsed.Interfaces {
		if parsed.Interfaces[i].Name == "ReadWriter" {
			rw = &parsed.Interfaces[i]
			break
		}
	}
	if rw == nil {
		t.Fatal("ReadWriter interface not found")
	}
	if len(rw.Embedded) != 2 {
		t.Errorf("ReadWriter should embed 2 interfaces, got %d", len(rw.Embedded))
	}
}

func TestParseFunctionSimple(t *testing.T) {
	content := readTestFile(t, "function_simple.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	// Should find exported functions
	exported := 0
	for _, fn := range parsed.Functions {
		if isExported(fn.Name) {
			exported++
		}
	}
	if exported != 3 {
		t.Errorf("expected 3 exported functions, got %d", exported)
	}

	// Find Println with variadic param
	var println *functionDef
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == "Println" {
			println = &parsed.Functions[i]
			break
		}
	}
	if println == nil {
		t.Fatal("Println not found")
	}
	if len(println.Params) == 0 {
		t.Fatal("Println should have parameters")
	}
	if !println.Params[0].Variadic {
		t.Error("Println param should be variadic")
	}
	if println.ReturnType == "" {
		t.Error("Println should have return type")
	}
}

func TestParseFunctionDoc(t *testing.T) {
	content := readTestFile(t, "function_simple.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	var println *functionDef
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == "Println" {
			println = &parsed.Functions[i]
			break
		}
	}
	if println == nil {
		t.Fatal("Println not found")
	}
	if println.DocComment == "" {
		t.Error("Println should have a doc comment")
	}
	if !strings.Contains(println.DocComment, "Println formats") {
		t.Errorf("unexpected doc: %q", println.DocComment)
	}
}

func TestParseConstants(t *testing.T) {
	content := readTestFile(t, "constants.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(parsed.Constants) < 3 {
		t.Fatalf("expected at least 3 constants, got %d", len(parsed.Constants))
	}

	// Find StatusOK
	var found bool
	for _, c := range parsed.Constants {
		if c.Name == "StatusOK" {
			found = true
			if c.Value != "200" {
				t.Errorf("StatusOK value = %q, want %q", c.Value, "200")
			}
			break
		}
	}
	if !found {
		t.Error("StatusOK constant not found")
	}
}

func TestParseTypeAlias(t *testing.T) {
	content := readTestFile(t, "type_alias.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(parsed.TypeAliases) < 2 {
		t.Fatalf("expected at least 2 type aliases, got %d", len(parsed.TypeAliases))
	}

	// Find HandlerFunc
	var hf *typeAliasDef
	for i := range parsed.TypeAliases {
		if parsed.TypeAliases[i].Name == "HandlerFunc" {
			hf = &parsed.TypeAliases[i]
			break
		}
	}
	if hf == nil {
		t.Fatal("HandlerFunc not found")
	}
	if hf.DocComment == "" {
		t.Error("HandlerFunc should have a doc comment")
	}
}

func TestParseGenerics(t *testing.T) {
	content := readTestFile(t, "generics.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	// Should find generic structs
	var setFound bool
	for _, st := range parsed.Structs {
		if st.Name == "Set" {
			setFound = true
			if st.TypeParams == "" {
				t.Error("Set should have type params")
			}
			break
		}
	}
	if !setFound {
		t.Error("Set generic struct not found")
	}

	// Should find generic functions
	var mapFound bool
	for _, fn := range parsed.Functions {
		if fn.Name == "Map" {
			mapFound = true
			if fn.TypeParams == "" {
				t.Error("Map should have type params")
			}
			break
		}
	}
	if !mapFound {
		t.Error("Map generic function not found")
	}
}

func TestParseEmbedding(t *testing.T) {
	content := readTestFile(t, "embedding.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	// Find Client with embedded *Transport
	var client *structDef
	for i := range parsed.Structs {
		if parsed.Structs[i].Name == "Client" {
			client = &parsed.Structs[i]
			break
		}
	}
	if client == nil {
		t.Fatal("Client struct not found")
	}
	if len(client.Embedded) == 0 {
		t.Error("Client should have embedded types")
	}

	// Find Mux with embedded Handler interface
	var mux *structDef
	for i := range parsed.Structs {
		if parsed.Structs[i].Name == "Mux" {
			mux = &parsed.Structs[i]
			break
		}
	}
	if mux == nil {
		t.Fatal("Mux struct not found")
	}
	if len(mux.Embedded) == 0 {
		t.Error("Mux should have embedded types")
	}
}

// --- Wrapper detection tests ---

func TestDetectWrapperSimple(t *testing.T) {
	content := readTestFile(t, "function_wrapper.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	idx := emptyIndex()

	// Test ListenAndServe wrapper
	var listenAndServe *functionDef
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == "ListenAndServe" {
			listenAndServe = &parsed.Functions[i]
			break
		}
	}
	if listenAndServe == nil {
		t.Fatal("ListenAndServe not found")
	}

	src := extractSource(content, listenAndServe.StartPos, listenAndServe.EndPos)
	isW, target, kind := detectWrapper([]byte(src), idx)
	if !isW {
		t.Error("ListenAndServe should be detected as a wrapper")
	}
	if target != "Serve" {
		t.Errorf("wrapper target = %q, want %q", target, "Serve")
	}
	if kind != "function" {
		t.Errorf("wrapper kind = %q, want %q", kind, "function")
	}
}

func TestDetectWrapperMethodDelegation(t *testing.T) {
	content := readTestFile(t, "function_wrapper.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	idx := emptyIndex()

	// Test Get wrapper (DefaultClient.Get)
	var get *functionDef
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == "Get" {
			get = &parsed.Functions[i]
			break
		}
	}
	if get == nil {
		t.Fatal("Get not found")
	}

	src := extractSource(content, get.StartPos, get.EndPos)
	isW, target, kind := detectWrapper([]byte(src), idx)
	if !isW {
		t.Error("Get should be detected as a wrapper")
	}
	if kind != "method" {
		t.Errorf("wrapper kind = %q, want %q", kind, "method")
	}
	_ = target
}

func TestDetectWrapperErrorCheck(t *testing.T) {
	content := readTestFile(t, "function_wrapper.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	idx := emptyIndex()

	// Test CacheGet wrapper
	var cacheGet *functionDef
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == "CacheGet" {
			cacheGet = &parsed.Functions[i]
			break
		}
	}
	if cacheGet == nil {
		t.Fatal("CacheGet not found")
	}

	src := extractSource(content, cacheGet.StartPos, cacheGet.EndPos)
	isW, _, _ := detectWrapper([]byte(src), idx)
	if !isW {
		t.Error("CacheGet should be detected as a wrapper (error-check pattern)")
	}
}

func TestDetectNonWrapper(t *testing.T) {
	content := readTestFile(t, "function_non_wrapper.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	idx := emptyIndex()

	for _, fn := range parsed.Functions {
		if !isExported(fn.Name) {
			continue
		}
		src := extractSource(content, fn.StartPos, fn.EndPos)
		isW, target, _ := detectWrapper([]byte(src), idx)
		if isW {
			t.Errorf("%s should NOT be detected as a wrapper (target=%q)", fn.Name, target)
		}
	}
}

// --- Discovery tests ---

func TestDiscovery(t *testing.T) {
	tdPath := testdataPath(t)

	// Use the testdata dir directly as repoPath, with no root dirs
	// and override the exclude list to NOT skip "testdata" since we ARE testdata.
	s := New(tdPath, Config{
		LibraryID:   "go/test",
		Name:        "Test",
		RootDirs:    []string{""},
		ExcludeDirs: []string{}, // override defaults
	})
	// Override the default excludeDirs for this test by building index directly
	idx, err := buildCodebaseIndexForTest(tdPath)
	if err != nil {
		t.Fatal(err)
	}
	s.index = idx
	ids := idx.buildEntityList()

	if len(ids) == 0 {
		t.Fatal("no entities discovered")
	}

	// Verify only exported entities
	for _, id := range ids {
		_, fragment := splitFragment(id)
		if fragment == "" {
			t.Errorf("entity ID %q has no fragment", id)
			continue
		}
		if !isExported(fragment) {
			t.Errorf("unexported entity in discovery results: %q", id)
		}
	}

	// Verify Server struct is found
	found := false
	for _, id := range ids {
		if strings.HasSuffix(id, "#Server") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Server struct not found in discovery")
	}
}

func TestDiscoverySkipsTestFiles(t *testing.T) {
	tdPath := testdataPath(t)

	idx, err := buildCodebaseIndexForTest(tdPath)
	if err != nil {
		t.Fatal(err)
	}
	ids := idx.buildEntityList()

	for _, id := range ids {
		if strings.Contains(id, "_test.go") {
			t.Errorf("test file in discovery results: %q", id)
		}
	}
}

// --- Export filtering tests ---

func TestIsExported(t *testing.T) {
	tests := []struct {
		name   string
		export bool
	}{
		{"Server", true},
		{"Handler", true},
		{"unexportedFunc", false},
		{"", false},
		{"_internal", false},
		{"A", true},
		{"a", false},
	}

	for _, tt := range tests {
		if got := isExported(tt.name); got != tt.export {
			t.Errorf("isExported(%q) = %v, want %v", tt.name, got, tt.export)
		}
	}
}

// --- ParseEntity integration test ---

func TestParseEntityStruct(t *testing.T) {
	tdPath := testdataPath(t)
	filePath := filepath.Join(tdPath, "struct_simple.go")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	s := New(tdPath, Config{
		LibraryID:       "go/test",
		GitHubOwnerRepo: "test/repo",
		Ref:             "main",
	})

	entity, methodIDs, err := s.ParseEntity(context.Background(), filePath+"#Server", content)
	if err != nil {
		t.Fatal(err)
	}
	if entity == nil {
		t.Fatal("entity is nil")
	}

	if entity.Name != "Server" {
		t.Errorf("Name = %q, want %q", entity.Name, "Server")
	}
	if entity.Kind != source.KindStruct {
		t.Errorf("Kind = %q, want %q", entity.Kind, source.KindStruct)
	}
	if entity.Description == "" {
		t.Error("Description is empty")
	}
	if entity.SourceCode == "" {
		t.Error("SourceCode is empty")
	}
	if entity.URL == "" {
		t.Error("URL is empty")
	}
	if !strings.Contains(entity.URL, "github.com/test/repo") {
		t.Errorf("URL should contain github repo: %q", entity.URL)
	}

	// Should have exported properties only
	for _, p := range entity.Properties {
		if p.Name != "" && !isExported(p.Name) {
			t.Errorf("unexported property in entity: %q", p.Name)
		}
	}

	// Should have method IDs for exported methods
	if len(methodIDs) != 2 {
		t.Errorf("expected 2 method IDs, got %d: %v", len(methodIDs), methodIDs)
	}
}

func TestParseEntityFunction(t *testing.T) {
	tdPath := testdataPath(t)
	filePath := filepath.Join(tdPath, "function_simple.go")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	s := New(tdPath, Config{LibraryID: "go/test"})

	entity, _, err := s.ParseEntity(context.Background(), filePath+"#Println", content)
	if err != nil {
		t.Fatal(err)
	}
	if entity.Kind != source.KindFunction {
		t.Errorf("Kind = %q, want %q", entity.Kind, source.KindFunction)
	}
	if entity.Name != "Println" {
		t.Errorf("Name = %q, want %q", entity.Name, "Println")
	}
}

func TestParseEntityInterface(t *testing.T) {
	tdPath := testdataPath(t)
	filePath := filepath.Join(tdPath, "interface_simple.go")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	s := New(tdPath, Config{LibraryID: "go/test"})

	entity, _, err := s.ParseEntity(context.Background(), filePath+"#Handler", content)
	if err != nil {
		t.Fatal(err)
	}
	if entity.Kind != source.KindInterface {
		t.Errorf("Kind = %q, want %q", entity.Kind, source.KindInterface)
	}
}

// --- ParseMethod test ---

func TestParseMethod(t *testing.T) {
	tdPath := testdataPath(t)
	filePath := filepath.Join(tdPath, "struct_simple.go")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	s := New(tdPath, Config{LibraryID: "go/test"})

	method, err := s.ParseMethod(context.Background(), filePath+"#Server.ListenAndServe", content)
	if err != nil {
		t.Fatal(err)
	}
	if method == nil {
		t.Fatal("method is nil")
	}
	if method.Name != "ListenAndServe" {
		t.Errorf("Name = %q, want %q", method.Name, "ListenAndServe")
	}
	if method.Signature == "" {
		t.Error("Signature is empty")
	}
	if !strings.Contains(method.Signature, "Server") {
		t.Errorf("Signature should contain receiver type: %q", method.Signature)
	}
	if method.ReturnType == "" {
		t.Error("ReturnType is empty")
	}
}

// --- ParseSourceCode test ---

func TestParseSourceCode(t *testing.T) {
	tdPath := testdataPath(t)
	filePath := filepath.Join(tdPath, "struct_simple.go")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	s := New(tdPath, Config{LibraryID: "go/test"})

	src, err := s.ParseSourceCode(filePath+"#Server", content)
	if err != nil {
		t.Fatal(err)
	}
	if src == "" {
		t.Error("source code is empty")
	}
	if !strings.Contains(src, "Server") {
		t.Errorf("source should contain 'Server': %q", src)
	}
}

// --- Doc comment with deprecation ---

func TestDocDeprecated(t *testing.T) {
	content := readTestFile(t, "doc_complex.go")
	parsed := parseFile(content)
	if parsed == nil {
		t.Fatal("parseFile returned nil")
	}

	// Find Title function (deprecated)
	var title *functionDef
	for i := range parsed.Functions {
		if parsed.Functions[i].Name == "Title" {
			title = &parsed.Functions[i]
			break
		}
	}
	if title == nil {
		t.Fatal("Title not found")
	}
	if !strings.Contains(title.DocComment, "Deprecated:") {
		t.Errorf("Title doc should contain 'Deprecated:': %q", title.DocComment)
	}
}

// --- Slug building ---

func TestBuildSlug(t *testing.T) {
	tests := []struct {
		pkg, name, want string
	}{
		{"http", "Server", "http/Server"},
		{"fmt", "Println", "fmt/Println"},
		{"", "Server", "Server"},
	}

	for _, tt := range tests {
		got := buildSlug(tt.pkg, tt.name)
		if got != tt.want {
			t.Errorf("buildSlug(%q, %q) = %q, want %q", tt.pkg, tt.name, got, tt.want)
		}
	}
}

// buildCodebaseIndexForTest builds an index from the testdata directory without
// the default "testdata" exclusion.
func buildCodebaseIndexForTest(tdPath string) (*codebaseIndex, error) {
	cfg := Config{
		RootDirs:    []string{""},
		ExcludeDirs: []string{},
	}
	// Override defaultExcludeDirs by temporarily swapping
	return buildCodebaseIndex(tdPath, cfg)
}

// --- Method signature building ---

func TestBuildMethodSignature(t *testing.T) {
	m := &methodDef{
		Name:            "ListenAndServe",
		ReceiverName:    "srv",
		ReceiverType:    "Server",
		PointerReceiver: true,
		Params:          nil,
		ReturnType:      "error",
	}

	sig := buildMethodSignature(m)
	if !strings.Contains(sig, "*Server") {
		t.Errorf("signature should contain pointer receiver: %q", sig)
	}
	if !strings.Contains(sig, "ListenAndServe") {
		t.Errorf("signature should contain method name: %q", sig)
	}
	if !strings.Contains(sig, "error") {
		t.Errorf("signature should contain return type: %q", sig)
	}
}
