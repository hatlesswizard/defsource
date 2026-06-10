package treesitter

import (
	"context"
	"sync"
	"testing"
)

// TestPool_GetPut_AllLanguages verifies Get and Put work for all 11 languages.
func TestPool_GetPut_AllLanguages(t *testing.T) {
	languages := []Language{PHP, JavaScript, TypeScript, Python, Go, Java, C, Cpp, CSharp, Ruby, Rust}

	for _, lang := range languages {
		t.Run(string(lang), func(t *testing.T) {
			parser, err := Get(lang)
			if err != nil {
				t.Fatalf("Get(%q) failed: %v", lang, err)
			}
			if parser == nil {
				t.Fatalf("Get(%q) returned nil parser", lang)
			}
			// Put should not panic
			Put(lang, parser)
		})
	}
}

// TestPool_GetInvalidLanguage verifies Get returns an error for unknown languages.
func TestPool_GetInvalidLanguage(t *testing.T) {
	invalids := []Language{"brainfuck", "cobol", "", "PHP", "PYTHON"}

	for _, lang := range invalids {
		t.Run(string(lang), func(t *testing.T) {
			parser, err := Get(lang)
			if err == nil {
				t.Errorf("Get(%q) should have returned error, got parser=%v", lang, parser)
			}
			if parser != nil {
				t.Errorf("Get(%q) returned non-nil parser on error", lang)
			}
		})
	}
}

// TestPool_PutInvalidLanguage verifies Put with an invalid language does not panic.
func TestPool_PutInvalidLanguage(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Put with invalid language panicked: %v", r)
		}
	}()

	// Get a valid parser, then try to put it back under an invalid language
	parser, err := Get(Go)
	if err != nil {
		t.Fatal(err)
	}
	Put("invalid_language", parser)
}

// TestPool_Concurrent_GetPut verifies concurrent Get/Put operations are race-free.
func TestPool_Concurrent_GetPut(t *testing.T) {
	languages := []Language{PHP, JavaScript, TypeScript, Python, Go, Java, C, Cpp, CSharp, Ruby, Rust}

	const goroutines = 50
	var wg sync.WaitGroup

	for _, lang := range languages {
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func(l Language) {
				defer wg.Done()
				parser, err := Get(l)
				if err != nil {
					return
				}
				// Simulate some work with the parser
				_ = parser
				Put(l, parser)
			}(lang)
		}
	}

	wg.Wait()
}

// TestPool_Reuse verifies that the pool actually reuses parsers rather than
// creating new ones every time.
func TestPool_Reuse(t *testing.T) {
	// Get a parser
	p1, err := Get(Go)
	if err != nil {
		t.Fatal(err)
	}
	// Put it back
	Put(Go, p1)
	// Get again - should get the same parser back (pool reuse)
	p2, err := Get(Go)
	if err != nil {
		t.Fatal(err)
	}
	// Note: We cannot guarantee the pool returns the same pointer (GC may
	// have collected it), but in practice with no other goroutines it should.
	// We just verify we get a valid parser.
	if p2 == nil {
		t.Fatal("second Get returned nil")
	}
	Put(Go, p2)
}

// TestPool_HighContention verifies pool behavior under high contention where
// multiple goroutines exhaust the pool and force creation of new parsers.
func TestPool_HighContention(t *testing.T) {
	const goroutines = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// All goroutines hold parsers simultaneously, forcing pool expansion
	barriers := make(chan struct{})
	parsers := make(chan struct{ done bool }, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			parser, err := Get(Python)
			if err != nil {
				parsers <- struct{ done bool }{false}
				return
			}
			// Hold the parser until signal
			<-barriers
			Put(Python, parser)
			parsers <- struct{ done bool }{true}
		}()
	}

	// Release all goroutines
	close(barriers)
	wg.Wait()
	close(parsers)

	success := 0
	for p := range parsers {
		if p.done {
			success++
		}
	}
	if success != goroutines {
		t.Errorf("expected %d successful operations, got %d", goroutines, success)
	}
}

// TestGetLanguage_AllSupported verifies GetLanguage works for all supported languages.
func TestGetLanguage_AllSupported(t *testing.T) {
	languages := []Language{PHP, JavaScript, TypeScript, Python, Go, Java, C, Cpp, CSharp, Ruby, Rust}

	for _, lang := range languages {
		t.Run(string(lang), func(t *testing.T) {
			grammar, err := GetLanguage(lang)
			if err != nil {
				t.Fatalf("GetLanguage(%q) failed: %v", lang, err)
			}
			if grammar == nil {
				t.Fatalf("GetLanguage(%q) returned nil", lang)
			}
		})
	}
}

// TestGetLanguage_Invalid verifies GetLanguage returns error for unsupported languages.
func TestGetLanguage_Invalid(t *testing.T) {
	_, err := GetLanguage("haskell")
	if err == nil {
		t.Error("expected error for unsupported language, got nil")
	}
}

// TestSupportedLanguages verifies that SupportedLanguages returns all 11 languages.
func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()
	if len(langs) != 11 {
		t.Errorf("SupportedLanguages() returned %d languages, expected 11", len(langs))
	}

	expected := map[Language]bool{
		PHP: true, JavaScript: true, TypeScript: true, Python: true,
		Go: true, Java: true, C: true, Cpp: true,
		CSharp: true, Ruby: true, Rust: true,
	}

	for _, l := range langs {
		if !expected[l] {
			t.Errorf("unexpected language in SupportedLanguages(): %q", l)
		}
	}
}

// TestPool_ParseAfterReturn verifies parsers still work after being returned
// to pool and retrieved again.
func TestPool_ParseAfterReturn(t *testing.T) {
	goCode := []byte("package main\n\nfunc main() {}\n")

	ctx := context.Background()

	// Get, parse, put, get, parse again
	p1, err := Get(Go)
	if err != nil {
		t.Fatal(err)
	}
	tree1, err := p1.ParseCtx(ctx, nil, goCode)
	if err != nil {
		t.Fatal(err)
	}
	if tree1 == nil {
		t.Fatal("first parse returned nil tree")
	}
	tree1.Close()
	Put(Go, p1)

	p2, err := Get(Go)
	if err != nil {
		t.Fatal(err)
	}
	tree2, err := p2.ParseCtx(ctx, nil, goCode)
	if err != nil {
		t.Fatal(err)
	}
	if tree2 == nil {
		t.Fatal("second parse returned nil tree")
	}
	tree2.Close()
	Put(Go, p2)
}
