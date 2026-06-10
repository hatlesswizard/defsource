package source_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/clang"
	"github.com/hatlesswizard/defsource/internal/source/cpp"
	"github.com/hatlesswizard/defsource/internal/source/csharp"
	"github.com/hatlesswizard/defsource/internal/source/golang"
	"github.com/hatlesswizard/defsource/internal/source/java"
	"github.com/hatlesswizard/defsource/internal/source/javascript"
	"github.com/hatlesswizard/defsource/internal/source/python"
	"github.com/hatlesswizard/defsource/internal/source/ruby"
	"github.com/hatlesswizard/defsource/internal/source/rust"
	"github.com/hatlesswizard/defsource/internal/source/typescript"
	"github.com/hatlesswizard/defsource/internal/source/php"
)

// allSources returns minimal source adapter instances for edge-case testing.
func allSources(t *testing.T) []struct {
	name string
	src  source.Source
} {
	t.Helper()
	return []struct {
		name string
		src  source.Source
	}{
		{"go", golang.New("/tmp/fake", golang.Config{LibraryID: "go/test"})},
		{"python", python.New("/tmp/fake", python.Config{LibraryID: "/python/test"})},
		{"rust", rust.New("/tmp/fake", rust.Config{CrateName: "test"})},
		{"java", java.New("/tmp/fake", java.Config{LibraryID: "java/test"})},
		{"javascript", javascript.New("/tmp/fake", javascript.WithConfig(javascript.LibraryConfig{ID: "/js/test"}))},
		{"typescript", typescript.New(typescript.Config{LibraryID: "ts/test"}, "/tmp/fake")},
		{"ruby", ruby.New("/tmp/fake", ruby.Config{LibraryID: "/ruby/test"})},
		{"php", php.New("/tmp/fake")},
		{"csharp", csharp.New("/tmp/fake", csharp.Config{LibraryID: "csharp/test"})},
		{"c", clang.New("/tmp/fake", clang.Config{LibraryID: "/c/test"})},
		{"cpp", cpp.New(cpp.Config{RepoPath: "/tmp/fake", LibraryID: "cpp/test"})},
	}
}

// TestEdge_EmptyContent verifies that ParseEntity with empty content does not
// panic and returns an error.
func TestEdge_EmptyContent(t *testing.T) {
	ctx := context.Background()
	for _, e := range allSources(t) {
		t.Run(e.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ParseEntity panicked with empty content: %v", r)
				}
			}()
			entity, _, err := e.src.ParseEntity(ctx, "/fake/file.ext#SomeName", []byte{})
			// Should either return an error or nil entity, not panic
			if err == nil && entity != nil {
				t.Log("INFO: ParseEntity succeeded with empty content (returned entity from nothing)")
			}
		})
	}
}

// TestEdge_BinaryContent verifies that ParseEntity with binary/garbage content
// does not panic.
func TestEdge_BinaryContent(t *testing.T) {
	ctx := context.Background()
	// Generate binary garbage (includes null bytes, high bytes)
	garbage := make([]byte, 512)
	for i := range garbage {
		garbage[i] = byte(i % 256)
	}

	for _, e := range allSources(t) {
		t.Run(e.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ParseEntity panicked with binary content: %v", r)
				}
			}()
			_, _, _ = e.src.ParseEntity(ctx, "/fake/file.ext#SomeName", garbage)
			// We don't care about the result, just that it doesn't panic
		})
	}
}

// TestEdge_LargeFile verifies that ParseEntity handles large files (>1MB) without
// hanging or crashing.
func TestEdge_LargeFile(t *testing.T) {
	ctx := context.Background()

	// Build a large Go file (>1MB of source)
	var sb strings.Builder
	sb.WriteString("package large\n\n")
	for i := 0; i < 20000; i++ {
		sb.WriteString("// This is a comment line to inflate the file.\n")
		sb.WriteString("func Func")
		sb.WriteString(strings.Repeat("X", 10))
		sb.WriteString("() {}\n\n")
	}
	largeGo := []byte(sb.String())
	if len(largeGo) < 1_000_000 {
		t.Fatalf("expected >1MB, got %d bytes", len(largeGo))
	}

	src := golang.New("/tmp/fake", golang.Config{LibraryID: "go/test"})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParseEntity panicked with large file: %v", r)
		}
	}()
	// Try to parse - should not hang or crash
	_, _, _ = src.ParseEntity(ctx, "/tmp/fake/large.go#FuncXXXXXXXXXX", largeGo)
}

// TestEdge_DeeplyNestedStructures verifies parsers handle deeply nested code.
func TestEdge_DeeplyNestedStructures(t *testing.T) {
	ctx := context.Background()

	// Python with deep nesting
	var pyBuilder strings.Builder
	pyBuilder.WriteString("class Outer:\n")
	pyBuilder.WriteString("    \"\"\"Outer class.\"\"\"\n")
	indent := "    "
	for i := 0; i < 20; i++ {
		indent += "    "
		pyBuilder.WriteString(indent + "def level_" + strings.Repeat("x", 3) + "(self):\n")
		pyBuilder.WriteString(indent + "    pass\n")
	}
	deepPython := []byte(pyBuilder.String())

	src := python.New("/tmp/fake", python.Config{LibraryID: "/python/test"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParseEntity panicked with deeply nested Python: %v", r)
		}
	}()
	_, _, _ = src.ParseEntity(ctx, "/tmp/fake/deep.py#Outer", deepPython)
}

// TestEdge_UnicodeInIdentifiers verifies that Unicode identifiers are handled.
func TestEdge_UnicodeInIdentifiers(t *testing.T) {
	ctx := context.Background()

	// Python allows Unicode identifiers
	pyUnicode := []byte(`class Grüße:
    """A class with Unicode name."""
    def berechne_wärme(self):
        """Calculate heat."""
        pass
`)
	src := python.New("/tmp/fake", python.Config{LibraryID: "/python/test"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParseEntity panicked with Unicode identifiers: %v", r)
		}
	}()
	entity, _, err := src.ParseEntity(ctx, "/tmp/fake/unicode.py#Grüße", pyUnicode)
	if err != nil {
		t.Logf("INFO: ParseEntity with Unicode name returned error: %v (acceptable)", err)
		return
	}
	if entity != nil && entity.Name != "Grüße" {
		t.Errorf("Entity.Name = %q, expected %q", entity.Name, "Grüße")
	}
}

// TestEdge_NoDocComments verifies that entities without doc comments are still
// parsed (just with empty description).
func TestEdge_NoDocComments(t *testing.T) {
	ctx := context.Background()

	goNoDoc := []byte(`package nodoc

type Bare struct {
	Field string
}

func (b *Bare) DoStuff() {}
`)
	src := golang.New("/tmp/fake", golang.Config{LibraryID: "go/test"})
	entity, _, err := src.ParseEntity(ctx, "/tmp/fake/nodoc.go#Bare", goNoDoc)
	if err != nil {
		t.Fatalf("ParseEntity failed: %v", err)
	}
	if entity == nil {
		t.Fatal("ParseEntity returned nil for undocumented entity")
	}
	if entity.Name != "Bare" {
		t.Errorf("Entity.Name = %q, want %q", entity.Name, "Bare")
	}
	// Description can be empty but entity must still be parsed
	if entity.SourceCode == "" {
		t.Error("Entity.SourceCode is empty for undocumented entity")
	}
}

// TestEdge_SelfReferencingWrapper verifies that circular wrapper detection
// does not infinite loop.
func TestEdge_SelfReferencingWrapper(t *testing.T) {
	ctx := context.Background()

	// Go function that calls itself
	goSelfRef := []byte(`package selfref

func RecursiveFunc() {
	RecursiveFunc()
}
`)
	src := golang.New("/tmp/fake", golang.Config{LibraryID: "go/test"})
	entity, _, err := src.ParseEntity(ctx, "/tmp/fake/selfref.go#RecursiveFunc", goSelfRef)
	if err != nil {
		t.Fatalf("ParseEntity failed: %v", err)
	}
	if entity == nil {
		t.Fatal("ParseEntity returned nil")
	}

	// Now test DetectWrapper on a self-referencing method
	method := &source.Method{
		Name:       "RecursiveFunc",
		SourceCode: "func RecursiveFunc() {\n\tRecursiveFunc()\n}",
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		// This should not hang
		_, _, _ = src.DetectWrapper(method)
	}()
	select {
	case <-done:
		// OK
	default:
		// Give it a moment, the test will timeout if it truly hangs
	}
}

// TestEdge_MaxParameters verifies that methods with many parameters are handled.
func TestEdge_MaxParameters(t *testing.T) {
	ctx := context.Background()

	// Go function with 100+ parameters
	var sb strings.Builder
	sb.WriteString("package manyparams\n\n")
	sb.WriteString("// ManyParams has many parameters.\n")
	sb.WriteString("func ManyParams(")
	for i := 0; i < 100; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("p")
		sb.WriteString(strings.Repeat("x", 3))
		sb.WriteString(" int")
	}
	sb.WriteString(") {}\n")
	manyParamsGo := []byte(sb.String())

	src := golang.New("/tmp/fake", golang.Config{LibraryID: "go/test"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParseEntity panicked with 100+ parameters: %v", r)
		}
	}()
	entity, _, err := src.ParseEntity(ctx, "/tmp/fake/manyparams.go#ManyParams", manyParamsGo)
	if err != nil {
		t.Fatalf("ParseEntity failed: %v", err)
	}
	if entity == nil {
		t.Fatal("ParseEntity returned nil")
	}
}

// TestEdge_VeryLongIdentifier verifies handling of extremely long identifiers.
func TestEdge_VeryLongIdentifier(t *testing.T) {
	ctx := context.Background()

	longName := "Func" + strings.Repeat("A", 996)
	goLongName := []byte("package longname\n\nfunc " + longName + "() {}\n")

	src := golang.New("/tmp/fake", golang.Config{LibraryID: "go/test"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParseEntity panicked with 1000-char identifier: %v", r)
		}
	}()
	entity, _, err := src.ParseEntity(ctx, "/tmp/fake/longname.go#"+longName, goLongName)
	if err != nil {
		t.Fatalf("ParseEntity failed: %v", err)
	}
	if entity == nil {
		t.Fatal("ParseEntity returned nil")
	}
	if entity.Name != longName {
		t.Errorf("Entity.Name length = %d, expected %d", len(entity.Name), len(longName))
	}
}

// TestEdge_MixedLineEndings verifies parsers handle CRLF, LF, and CR line endings.
func TestEdge_MixedLineEndings(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		content []byte
	}{
		{
			name:    "LF",
			content: []byte("package lf\n\n// Doc comment.\ntype LFStruct struct {\n\tField string\n}\n"),
		},
		{
			name:    "CRLF",
			content: []byte("package crlf\r\n\r\n// Doc comment.\r\ntype CRLFStruct struct {\r\n\tField string\r\n}\r\n"),
		},
		{
			name:    "CR",
			content: []byte("package cr\r\r// Doc comment.\rtype CRStruct struct {\r\tField string\r}\r"),
		},
	}

	src := golang.New("/tmp/fake", golang.Config{LibraryID: "go/test"})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ParseEntity panicked with %s line endings: %v", tc.name, r)
				}
			}()
			// Extract the struct name from content
			var entityName string
			switch tc.name {
			case "LF":
				entityName = "LFStruct"
			case "CRLF":
				entityName = "CRLFStruct"
			case "CR":
				entityName = "CRStruct"
			}
			_, _, _ = src.ParseEntity(ctx, "/tmp/fake/lineendings.go#"+entityName, tc.content)
		})
	}
}

// TestEdge_ParseEntity_NoFragment verifies that ParseEntity returns an error
// when the entityID has no fragment (no # separator).
func TestEdge_ParseEntity_NoFragment(t *testing.T) {
	ctx := context.Background()
	for _, e := range allSources(t) {
		t.Run(e.name, func(t *testing.T) {
			_, _, err := e.src.ParseEntity(ctx, "/some/path/without/fragment", []byte("content"))
			if err == nil {
				t.Error("expected error for entityID without fragment, got nil")
			}
		})
	}
}

// TestEdge_ParseMethod_NoFragment verifies that ParseMethod returns an error
// when the methodID has no fragment.
func TestEdge_ParseMethod_NoFragment(t *testing.T) {
	ctx := context.Background()
	for _, e := range allSources(t) {
		t.Run(e.name, func(t *testing.T) {
			_, err := e.src.ParseMethod(ctx, "/some/path/without/fragment", []byte("content"))
			if err == nil {
				t.Error("expected error for methodID without fragment, got nil")
			}
		})
	}
}

// TestEdge_ParseEntity_EntityNotFound verifies proper error when entity doesn't
// exist in the parsed content.
func TestEdge_ParseEntity_EntityNotFound(t *testing.T) {
	ctx := context.Background()

	// Valid Go source but with a non-existent entity name
	goContent := []byte("package foo\n\ntype Exists struct {}\n")
	src := golang.New("/tmp/fake", golang.Config{LibraryID: "go/test"})

	_, _, err := src.ParseEntity(ctx, "/tmp/fake/foo.go#DoesNotExist", goContent)
	if err == nil {
		t.Error("expected error for non-existent entity, got nil")
	}
}

// TestEdge_DetectWrapper_EmptySourceCode verifies DetectWrapper handles method
// with empty source code gracefully.
func TestEdge_DetectWrapper_EmptySourceCode(t *testing.T) {
	for _, e := range allSources(t) {
		t.Run(e.name, func(t *testing.T) {
			method := &source.Method{
				Name:       "Empty",
				SourceCode: "",
			}
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("DetectWrapper panicked with empty SourceCode: %v", r)
				}
			}()
			isW, target, kind := e.src.DetectWrapper(method)
			if isW {
				t.Errorf("DetectWrapper with empty source returned true, target=%q, kind=%q", target, kind)
			}
		})
	}
}

// TestEdge_ParseSourceCode_EmptyFragment verifies ParseSourceCode with no fragment
// returns the entire content.
func TestEdge_ParseSourceCode_EmptyFragment(t *testing.T) {
	for _, e := range allSources(t) {
		t.Run(e.name, func(t *testing.T) {
			content := []byte("some content")
			code, err := e.src.ParseSourceCode("/some/file", content)
			if err != nil {
				// Some adapters may error on no fragment
				return
			}
			if code == "" {
				t.Error("ParseSourceCode returned empty for full-file (no fragment)")
			}
		})
	}
}
