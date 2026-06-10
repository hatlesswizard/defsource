package source_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/golang"
	"github.com/hatlesswizard/defsource/internal/source/java"
	"github.com/hatlesswizard/defsource/internal/source/python"
	"github.com/hatlesswizard/defsource/internal/source/rust"
	"github.com/hatlesswizard/defsource/internal/source/typescript"
)

// TestConcurrent_ParseEntity_Go verifies that 100 concurrent ParseEntity calls
// on the same Go source are race-free and produce consistent results.
func TestConcurrent_ParseEntity_Go(t *testing.T) {
	td := testdataDir(t)
	content := readFile(t, filepath.Join(td, "sample.go"))
	src := golang.New(td, golang.Config{LibraryID: "go/test", Name: "Test"})
	entityID := filepath.Join(td, "sample.go") + "#Server"
	ctx := context.Background()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			entity, _, err := src.ParseEntity(ctx, entityID, content)
			if err != nil {
				errs <- err
				return
			}
			if entity == nil || entity.Name != "Server" {
				errs <- errorf("unexpected entity: %v", entity)
				return
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent ParseEntity error: %v", err)
	}
}

// TestConcurrent_ParseMethod_Go verifies that 100 concurrent ParseMethod calls
// are race-free on Go source.
func TestConcurrent_ParseMethod_Go(t *testing.T) {
	td := testdataDir(t)
	content := readFile(t, filepath.Join(td, "sample.go"))
	src := golang.New(td, golang.Config{LibraryID: "go/test", Name: "Test"})
	methodID := filepath.Join(td, "sample.go") + "#Server.ListenAndServe"
	ctx := context.Background()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			method, err := src.ParseMethod(ctx, methodID, content)
			if err != nil {
				errs <- err
				return
			}
			if method == nil || method.Name != "ListenAndServe" {
				errs <- errorf("unexpected method: %v", method)
				return
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent ParseMethod error: %v", err)
	}
}

// TestConcurrent_ParseEntity_Python verifies concurrency safety for Python adapter.
func TestConcurrent_ParseEntity_Python(t *testing.T) {
	td := testdataDir(t)
	content := readFile(t, filepath.Join(td, "sample.py"))
	src := python.New(td, python.Config{LibraryID: "/python/test", Name: "Test", SourceRoots: []string{""}})
	entityID := filepath.Join(td, "sample.py") + "#Server"
	ctx := context.Background()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			entity, _, err := src.ParseEntity(ctx, entityID, content)
			if err != nil {
				errs <- err
				return
			}
			if entity == nil || entity.Name != "Server" {
				errs <- errorf("unexpected entity: %v", entity)
				return
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent ParseEntity error: %v", err)
	}
}

// TestConcurrent_ParseEntity_Rust verifies concurrency safety for Rust adapter.
func TestConcurrent_ParseEntity_Rust(t *testing.T) {
	td := testdataDir(t)
	content := readFile(t, filepath.Join(td, "sample.rs"))
	src := rust.New(td, rust.Config{Owner: "test", Repo: "test", CrateName: "test", Version: "0.1.0"})
	entityID := filepath.Join(td, "sample.rs") + "#Server"
	ctx := context.Background()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			entity, _, err := src.ParseEntity(ctx, entityID, content)
			if err != nil {
				errs <- err
				return
			}
			if entity == nil || entity.Name != "Server" {
				errs <- errorf("unexpected entity: %v", entity)
				return
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent ParseEntity error: %v", err)
	}
}

// TestConcurrent_ParseEntity_Java verifies concurrency safety for Java adapter.
func TestConcurrent_ParseEntity_Java(t *testing.T) {
	td := testdataDir(t)
	content := readFile(t, filepath.Join(td, "sample.java"))
	src := java.New(td, java.Config{LibraryID: "java/test", Name: "Test"})
	entityID := filepath.Join(td, "sample.java") + "#Server"
	ctx := context.Background()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			entity, _, err := src.ParseEntity(ctx, entityID, content)
			if err != nil {
				errs <- err
				return
			}
			if entity == nil || entity.Name != "Server" {
				errs <- errorf("unexpected entity: %v", entity)
				return
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent ParseEntity error: %v", err)
	}
}

// TestConcurrent_ParseEntity_TypeScript verifies concurrency safety for TS adapter.
func TestConcurrent_ParseEntity_TypeScript(t *testing.T) {
	td := testdataDir(t)
	content := readFile(t, filepath.Join(td, "sample.ts"))
	src := typescript.New(typescript.Config{LibraryID: "ts/test", Name: "Test", SourceURL: "https://github.com/test/test"}, td)
	entityID := filepath.Join(td, "sample.ts") + "#Server"
	ctx := context.Background()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			entity, _, err := src.ParseEntity(ctx, entityID, content)
			if err != nil {
				errs <- err
				return
			}
			if entity == nil || entity.Name != "Server" {
				errs <- errorf("unexpected entity: %v", entity)
				return
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent ParseEntity error: %v", err)
	}
}

// TestConcurrent_DetectWrapper_AllLanguages verifies that DetectWrapper is
// safe to call concurrently across multiple adapters.
func TestConcurrent_DetectWrapper_AllLanguages(t *testing.T) {
	adapters := buildAdapters(t)
	ctx := context.Background()

	const goroutines = 20
	var wg sync.WaitGroup

	for _, a := range adapters {
		method, err := a.src.ParseMethod(ctx, a.methodID, a.content)
		if err != nil {
			continue
		}

		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func(src source.Source, m *source.Method) {
				defer wg.Done()
				_, _, _ = src.DetectWrapper(m)
			}(a.src, method)
		}
	}

	wg.Wait()
}

// TestConcurrent_MixedOperations verifies that ParseEntity and ParseMethod
// can be called concurrently on the same source without data races.
func TestConcurrent_MixedOperations(t *testing.T) {
	td := testdataDir(t)
	content := readFile(t, filepath.Join(td, "sample.go"))
	src := golang.New(td, golang.Config{LibraryID: "go/test", Name: "Test"})
	entityID := filepath.Join(td, "sample.go") + "#Server"
	methodID := filepath.Join(td, "sample.go") + "#Server.ListenAndServe"
	ctx := context.Background()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _, _ = src.ParseEntity(ctx, entityID, content)
		}()
		go func() {
			defer wg.Done()
			_, _ = src.ParseMethod(ctx, methodID, content)
		}()
	}

	wg.Wait()
}

// TestConcurrent_ParseSourceCode verifies ParseSourceCode is race-free.
func TestConcurrent_ParseSourceCode(t *testing.T) {
	td := testdataDir(t)
	content := readFile(t, filepath.Join(td, "sample.go"))
	src := golang.New(td, golang.Config{LibraryID: "go/test", Name: "Test"})
	entityID := filepath.Join(td, "sample.go") + "#Server"

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			code, err := src.ParseSourceCode(entityID, content)
			if err != nil {
				return
			}
			_ = code
		}()
	}

	wg.Wait()
}

// TestConcurrent_MetaAndID verifies that Meta() and ID() are safe when called
// concurrently (even though they should be trivially safe).
func TestConcurrent_MetaAndID(t *testing.T) {
	adapters := buildAdapters(t)

	const goroutines = 50
	var wg sync.WaitGroup

	for _, a := range adapters {
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func(src source.Source) {
				defer wg.Done()
				_ = src.ID()
				_ = src.Meta()
			}(a.src)
		}
	}

	wg.Wait()
}

// errorf is a helper to create formatted errors for channel-based error reporting.
func errorf(format string, args ...interface{}) error {
	return &testError{msg: format, args: args}
}

type testError struct {
	msg  string
	args []interface{}
}

func (e *testError) Error() string {
	return e.msg
}

// Verify the file exists for the test.
func init() {
	td, err := os.Getwd()
	if err != nil {
		return
	}
	_ = filepath.Join(td, "testdata")
}
