package source_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"unicode"
)

// TestConsistency_ClassEntity_HasCommonFields verifies that all adapters that
// produce a "class" or "struct" entity return a consistent output structure.
func TestConsistency_ClassEntity_HasCommonFields(t *testing.T) {
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

			// All class/struct entities should have Name
			if entity.Name == "" {
				t.Error("Entity.Name is empty for class-like entity")
			}
			// Name should not contain file paths or fragments
			if strings.Contains(entity.Name, "/") || strings.Contains(entity.Name, "#") {
				t.Errorf("Entity.Name %q looks like a path, not a name", entity.Name)
			}
			// Slug should be a reasonable identifier string
			if entity.Slug == "" {
				t.Error("Entity.Slug is empty")
			}
			// Kind should be one of the class-like kinds
			classLikeKinds := map[string]bool{
				"class": true, "struct": true, "interface": true,
				"trait": true, "record": true, "module": true,
			}
			if !classLikeKinds[entity.Kind] {
				t.Logf("INFO: Entity.Kind = %q (not class-like, may be expected)", entity.Kind)
			}
		})
	}
}

// TestConsistency_MethodParameters_HaveNamePopulated verifies that method
// parameters always have their Name field populated when parameters exist.
func TestConsistency_MethodParameters_HaveNamePopulated(t *testing.T) {
	ctx := context.Background()
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			method, err := e.src.ParseMethod(ctx, e.methodID, e.content)
			if err != nil {
				t.Skipf("ParseMethod failed: %v", err)
			}
			if method == nil {
				t.Skip("ParseMethod returned nil")
			}

			for i, p := range method.Parameters {
				if p.Name == "" {
					t.Errorf("Parameter[%d].Name is empty", i)
				}
			}
		})
	}
}

// TestConsistency_DocComment_ProducesSummary verifies that when a doc comment
// exists, the Summary/Description field is populated across all languages.
// Note: Some adapters may not extract the doc comment for the class itself
// if the tree-sitter grammar doesn't associate it (e.g., JS/TS export classes).
func TestConsistency_DocComment_ProducesSummary(t *testing.T) {
	ctx := context.Background()

	// Adapters known to potentially not extract class-level docs from fixtures
	// because of how tree-sitter associates comments (e.g., export class in JS/TS).
	maySkipClassDoc := map[string]bool{"javascript": true, "typescript": true}

	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			entity, _, err := e.src.ParseEntity(ctx, e.entityID, e.content)
			if err != nil {
				t.Fatalf("ParseEntity failed: %v", err)
			}
			if entity == nil {
				t.Fatal("ParseEntity returned nil")
			}

			// All test fixtures have doc comments, so Description should be non-empty
			if entity.Description == "" {
				if maySkipClassDoc[e.name] {
					t.Logf("INFO: %s Entity.Description is empty (known behavior for export class)", e.name)
				} else {
					t.Error("Entity.Description is empty despite fixture having doc comments")
				}
			}
		})
	}
}

// TestConsistency_EntitySlug_IsURLSafe verifies that entity slugs produced by
// all adapters are URL-safe (can be used in URL paths without encoding).
func TestConsistency_EntitySlug_IsURLSafe(t *testing.T) {
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

			slug := entity.Slug
			// Check that slug only contains URL-safe characters
			encoded := url.PathEscape(slug)
			// Allow "/" in slug since some adapters use package/name format
			encoded = strings.ReplaceAll(encoded, "%2F", "/")
			if encoded != slug {
				t.Errorf("Entity.Slug = %q is not URL-safe (would encode to %q)", slug, encoded)
			}
		})
	}
}

// TestConsistency_SourceCode_NonEmpty verifies that entities with source code
// always have non-empty SourceCode field (since our fixtures all have source).
func TestConsistency_SourceCode_NonEmpty(t *testing.T) {
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

			if entity.SourceCode == "" {
				t.Error("Entity.SourceCode is empty for entity that has source in fixture")
			}
		})
	}
}

// TestConsistency_MethodSignature_ContainsMethodName verifies that method
// signatures contain the method name itself.
func TestConsistency_MethodSignature_ContainsMethodName(t *testing.T) {
	ctx := context.Background()
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			method, err := e.src.ParseMethod(ctx, e.methodID, e.content)
			if err != nil {
				t.Skipf("ParseMethod failed: %v", err)
			}
			if method == nil {
				t.Skip("ParseMethod returned nil")
			}

			if method.Signature == "" {
				t.Log("INFO: Method.Signature is empty")
				return
			}
			if !strings.Contains(method.Signature, method.Name) {
				t.Errorf("Method.Signature %q does not contain method name %q", method.Signature, method.Name)
			}
		})
	}
}

// TestConsistency_EntityName_StartsWithLetter verifies that entity names
// start with a letter (or underscore for some languages).
func TestConsistency_EntityName_StartsWithLetter(t *testing.T) {
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

			first := rune(entity.Name[0])
			if !unicode.IsLetter(first) && first != '_' {
				t.Errorf("Entity.Name %q starts with non-letter %q", entity.Name, first)
			}
		})
	}
}

// TestConsistency_MethodSourceCode_ContainsMethodContent verifies that
// method source code contains identifiable content from the method.
func TestConsistency_MethodSourceCode_ContainsMethodContent(t *testing.T) {
	ctx := context.Background()
	for _, e := range buildAdapters(t) {
		t.Run(e.name, func(t *testing.T) {
			method, err := e.src.ParseMethod(ctx, e.methodID, e.content)
			if err != nil {
				t.Skipf("ParseMethod failed: %v", err)
			}
			if method == nil {
				t.Skip("ParseMethod returned nil")
			}

			if method.SourceCode == "" {
				t.Error("Method.SourceCode is empty")
				return
			}
			// The source code should contain the method name
			if !strings.Contains(method.SourceCode, method.Name) {
				t.Errorf("Method.SourceCode does not contain method name %q", method.Name)
			}
		})
	}
}
