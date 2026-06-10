package source_test

import (
	"context"
	"os"
	"path/filepath"
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

// testdataDir returns the path to the shared testdata directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "testdata")
}

// adapterEntry holds a source adapter and its associated fixture content.
type adapterEntry struct {
	name       string
	src        source.Source
	content    []byte
	entityID   string
	methodID   string
	language   string
}

// buildAdapters constructs all 11 source adapters pointing at the testdata directory.
// Each entry uses the correct entity/method ID format for its adapter.
func buildAdapters(t *testing.T) []adapterEntry {
	t.Helper()
	td := testdataDir(t)

	entries := []adapterEntry{
		{
			name:     "go",
			src:      golang.New(td, golang.Config{LibraryID: "go/test", Name: "Go Test", Description: "Test", SourceURL: "https://github.com/test/test", Ref: "main"}),
			content:  readFile(t, filepath.Join(td, "sample.go")),
			entityID: filepath.Join(td, "sample.go") + "#Server",
			methodID: filepath.Join(td, "sample.go") + "#Server.ListenAndServe",
			language: "go",
		},
		{
			name:     "python",
			src:      python.New(td, python.Config{LibraryID: "/python/test", Name: "Python Test", Description: "Test", SourceURL: "https://github.com/test/test", Version: "1.0", TrustScore: 0.9, SourceRoots: []string{""}}),
			content:  readFile(t, filepath.Join(td, "sample.py")),
			entityID: filepath.Join(td, "sample.py") + "#Server",
			methodID: filepath.Join(td, "sample.py") + "#Server.listen_and_serve",
			language: "python",
		},
		{
			name:     "rust",
			src:      rust.New(td, rust.Config{Owner: "test", Repo: "test", CrateName: "test", Version: "0.1.0", Description: "Test crate"}),
			content:  readFile(t, filepath.Join(td, "sample.rs")),
			entityID: filepath.Join(td, "sample.rs") + "#Server",
			methodID: filepath.Join(td, "sample.rs") + "#Server::new",
			language: "rust",
		},
		{
			name:     "java",
			src:      java.New(td, java.Config{LibraryID: "java/test", Name: "Java Test", Description: "Test", SourceURL: "https://github.com/test/test", SourceRoots: []string{""}}),
			content:  readFile(t, filepath.Join(td, "sample.java")),
			entityID: filepath.Join(td, "sample.java") + "#Server",
			methodID: filepath.Join(td, "sample.java") + "#Server::listenAndServe",
			language: "java",
		},
		{
			name: "javascript",
			src: javascript.New(td, javascript.WithConfig(javascript.LibraryConfig{
				ID: "/javascript/test", Name: "JS Test", Description: "Test",
				SourceURL: "https://github.com/test/test", Version: "1.0", BlobRef: "main",
			})),
			content:  readFile(t, filepath.Join(td, "sample.js")),
			entityID: filepath.Join(td, "sample.js") + "#Server",
			methodID: filepath.Join(td, "sample.js") + "#Server.listenAndServe",
			language: "javascript",
		},
		{
			name:     "typescript",
			src:      typescript.New(typescript.Config{LibraryID: "typescript/test", Name: "TS Test", Description: "Test", SourceURL: "https://github.com/test/test"}, td),
			content:  readFile(t, filepath.Join(td, "sample.ts")),
			entityID: filepath.Join(td, "sample.ts") + "#Server",
			methodID: filepath.Join(td, "sample.ts") + "#Server.listenAndServe",
			language: "typescript",
		},
		{
			name:     "ruby",
			src:      ruby.New(td, ruby.Config{LibraryID: "/ruby/test", Name: "Ruby Test", Description: "Test", SourceURL: "https://github.com/test/test", Version: "1.0", TrustScore: 0.9, SourceRoots: []string{""}}),
			content:  readFile(t, filepath.Join(td, "sample.rb")),
			entityID: filepath.Join(td, "sample.rb") + "#Server",
			methodID: filepath.Join(td, "sample.rb") + "#Server#listen_and_serve",
			language: "ruby",
		},
		{
			// PHP (WordPress): class entities have NO fragment; function entities have #FuncName
			// Method IDs use ClassName::methodName format
			name: "php",
			src: php.New(td, php.WithConfig(php.PHPConfig{
				LibraryID:   "/php/test",
				Name:        "PHP Test",
				Description: "Test",
				SourceURL:   "https://github.com/test/test",
			})),
			content:  readFile(t, filepath.Join(td, "sample.php")),
			entityID: filepath.Join(td, "sample.php"),              // class entity: no fragment
			methodID: filepath.Join(td, "sample.php") + "#Server::listenAndServe",
			language: "php",
		},
		{
			// C#: entity fragment format is "Namespace.TypeName"
			name:     "csharp",
			src:      csharp.New(td, csharp.Config{LibraryID: "csharp/test", Name: "C# Test", Description: "Test", SourceURL: "https://github.com/test/test"}),
			content:  readFile(t, filepath.Join(td, "sample.cs")),
			entityID: filepath.Join(td, "sample.cs") + "#Example.Server",
			methodID: filepath.Join(td, "sample.cs") + "#Example.Server.ListenAndServeAsync",
			language: "csharp",
		},
		{
			name:     "c",
			src:      clang.New(td, clang.Config{LibraryID: "/c/test", Name: "C Test", Description: "Test", SourceURL: "https://github.com/test/test", Version: "1.0", TrustScore: 0.9}),
			content:  readFile(t, filepath.Join(td, "sample.c")),
			entityID: filepath.Join(td, "sample.c") + "#Server",
			methodID: filepath.Join(td, "sample.c") + "#new_server",
			language: "c",
		},
		{
			name:     "cpp",
			src:      cpp.New(cpp.Config{RepoPath: td, LibraryID: "cpp/test", Name: "C++ Test", Description: "Test", SourceURL: "https://github.com/test/test", Version: "1.0", IncludeDirs: []string{""}}),
			content:  readFile(t, filepath.Join(td, "sample.cpp")),
			entityID: filepath.Join(td, "sample.cpp") + "#Server",
			methodID: filepath.Join(td, "sample.cpp") + "#Server::listenAndServe",
			language: "cpp",
		},
	}

	return entries
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return content
}

// TestAllAdapters_ID verifies that every source adapter returns a non-empty ID.
func TestAllAdapters_ID(t *testing.T) {
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			id := e.src.ID()
			if id == "" {
				t.Error("ID() returned empty string")
			}
		})
	}
}

// TestAllAdapters_Meta verifies that every source adapter returns valid metadata.
func TestAllAdapters_Meta(t *testing.T) {
	validLanguages := map[string]bool{
		"php": true, "go": true, "python": true, "javascript": true,
		"typescript": true, "java": true, "c": true, "cpp": true,
		"csharp": true, "ruby": true, "rust": true,
	}

	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			meta := e.src.Meta()
			if meta.Name == "" {
				t.Error("Meta().Name is empty")
			}
			if meta.Language == "" {
				t.Error("Meta().Language is empty")
			}
			if !validLanguages[meta.Language] {
				t.Errorf("Meta().Language = %q is not a valid language", meta.Language)
			}
			if meta.Language != e.language {
				t.Errorf("Meta().Language = %q, expected %q", meta.Language, e.language)
			}
		})
	}
}

// TestAllAdapters_ParseEntity verifies that ParseEntity returns a valid entity
// with required fields populated for every source adapter.
func TestAllAdapters_ParseEntity(t *testing.T) {
	ctx := context.Background()
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			entity, _, err := e.src.ParseEntity(ctx, e.entityID, e.content)
			if err != nil {
				t.Fatalf("ParseEntity failed: %v", err)
			}
			if entity == nil {
				t.Fatal("ParseEntity returned nil entity")
			}
			if entity.Name == "" {
				t.Error("Entity.Name is empty")
			}
			if entity.Slug == "" {
				t.Error("Entity.Slug is empty")
			}
			if entity.Kind == "" {
				t.Error("Entity.Kind is empty")
			}
			if entity.SourceCode == "" {
				t.Error("Entity.SourceCode is empty")
			}
		})
	}
}

// TestAllAdapters_ParseEntity_KindIsValid verifies that ParseEntity returns
// entities with a known valid kind.
func TestAllAdapters_ParseEntity_KindIsValid(t *testing.T) {
	ctx := context.Background()
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			entity, _, err := e.src.ParseEntity(ctx, e.entityID, e.content)
			if err != nil {
				t.Fatalf("ParseEntity failed: %v", err)
			}
			if entity == nil {
				t.Fatal("ParseEntity returned nil")
			}
			if !source.ValidKind(entity.Kind) {
				t.Errorf("Entity.Kind = %q is not a valid kind", entity.Kind)
			}
		})
	}
}

// TestAllAdapters_ParseMethod verifies that ParseMethod returns a valid method
// with required fields populated for every source adapter.
func TestAllAdapters_ParseMethod(t *testing.T) {
	ctx := context.Background()
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			method, err := e.src.ParseMethod(ctx, e.methodID, e.content)
			if err != nil {
				t.Fatalf("ParseMethod failed: %v", err)
			}
			if method == nil {
				t.Fatal("ParseMethod returned nil method")
			}
			if method.Name == "" {
				t.Error("Method.Name is empty")
			}
			if method.SourceCode == "" {
				t.Error("Method.SourceCode is empty")
			}
		})
	}
}

// TestAllAdapters_DetectWrapper verifies that DetectWrapper returns consistent
// (bool, string, string) values for every source adapter.
func TestAllAdapters_DetectWrapper(t *testing.T) {
	ctx := context.Background()
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			method, err := e.src.ParseMethod(ctx, e.methodID, e.content)
			if err != nil {
				t.Skipf("ParseMethod failed, skipping DetectWrapper: %v", err)
			}

			isWrapper, target, kind := e.src.DetectWrapper(method)
			// If it's a wrapper, target and kind must be non-empty
			if isWrapper {
				if target == "" {
					t.Error("DetectWrapper returned isWrapper=true but target is empty")
				}
				if kind == "" {
					t.Error("DetectWrapper returned isWrapper=true but kind is empty")
				}
			}
			// If not a wrapper, target and kind should be empty
			if !isWrapper {
				if target != "" {
					t.Errorf("DetectWrapper returned isWrapper=false but target = %q", target)
				}
				if kind != "" {
					t.Errorf("DetectWrapper returned isWrapper=false but kind = %q", kind)
				}
			}
		})
	}
}

// TestAllAdapters_DetectWrapper_NilMethod verifies DetectWrapper handles nil gracefully.
func TestAllAdapters_DetectWrapper_NilMethod(t *testing.T) {
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			isWrapper, target, kind := e.src.DetectWrapper(nil)
			if isWrapper {
				t.Error("DetectWrapper(nil) returned true")
			}
			if target != "" {
				t.Errorf("DetectWrapper(nil) returned target = %q", target)
			}
			if kind != "" {
				t.Errorf("DetectWrapper(nil) returned kind = %q", kind)
			}
		})
	}
}

// TestAllAdapters_ParseSourceCode verifies that ParseSourceCode extracts code.
func TestAllAdapters_ParseSourceCode(t *testing.T) {
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			code, err := e.src.ParseSourceCode(e.entityID, e.content)
			if err != nil {
				t.Fatalf("ParseSourceCode failed: %v", err)
			}
			if code == "" {
				t.Error("ParseSourceCode returned empty string")
			}
		})
	}
}

// TestAllAdapters_ResolveWrapperURL verifies ResolveWrapperURL does not panic
// for arbitrary inputs.
func TestAllAdapters_ResolveWrapperURL(t *testing.T) {
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			// Should not panic with empty values
			_ = e.src.ResolveWrapperURL("", "", "")
			_ = e.src.ResolveWrapperURL("SomeTarget", "function", "some-entity")
			_ = e.src.ResolveWrapperURL("SomeTarget", "method", "some-entity")
		})
	}
}
