//go:build sqlite_fts5 || fts5

package defsource

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewClient(t *testing.T) {
	// Create a temp directory for the test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	// Test ListLibraries on empty DB
	ctx := context.Background()
	libs, err := client.ListLibraries(ctx)
	if err != nil {
		t.Fatalf("ListLibraries() error: %v", err)
	}
	if len(libs) != 0 {
		t.Errorf("ListLibraries() = %d libraries, want 0", len(libs))
	}
}

func TestResolveLibraryEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	libs, err := client.ResolveLibrary(ctx, "test query", "wordpress")
	if err != nil {
		t.Fatalf("ResolveLibrary() error: %v", err)
	}
	if len(libs) != 0 {
		t.Errorf("ResolveLibrary() = %d, want 0 on empty DB", len(libs))
	}
}

func TestQueryDocsNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	_, err = client.QueryDocs(ctx, "/nonexistent/lib", "test")
	if err == nil {
		t.Error("QueryDocs() expected error for nonexistent library, got nil")
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Verify DB file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}
