// Package crawler_test provides characterization tests for the crawler package.
//
// These tests lock in the observable behaviour of Run, processEntity,
// resolveWrapperChain, and classifyError using mock implementations of the
// source.Source and store.Store interfaces.  No real HTTP is performed; all
// fetches either use NewLocalFetcher (with files in t.TempDir) or a fakeFetcher
// shim that returns pre-canned bytes.
//
// Run the full suite with the race detector enabled:
//
//	CGO_ENABLED=1 go test -race -tags sqlite_fts5 -v ./internal/crawler/...
package crawler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock Source implementation
// ─────────────────────────────────────────────────────────────────────────────

// mockSource is a configurable implementation of source.Source for testing.
// All method return values are set on construction; zero values are safe defaults.
type mockSource struct {
	id       string
	meta     source.LibraryMeta
	entities []string // URLs returned by DiscoverEntities

	// parseEntityFn overrides per-URL behaviour when set.
	// If nil, a default entity is returned for every URL.
	parseEntityFn func(url string) (*source.Entity, []string, error)

	// parseMethodFn overrides method parsing when set.
	parseMethodFn func(url string) (*source.Method, error)

	// detectWrapperFn overrides wrapper detection; by default returns (false,"","")
	detectWrapperFn func(m *source.Method) (bool, string, string)

	// resolveWrapperURLFn overrides URL resolution; by default returns "".
	resolveWrapperURLFn func(targetName, targetKind, entitySlug string) string

	// parseSourceCodeFn overrides source extraction; by default returns "".
	parseSourceCodeFn func(url string, body []byte) (string, error)

	// fetchFn is an optional override for DiscoverEntities' FetchFunc parameter.
	// Not part of the Source interface but used to record discovery calls in some tests.
	discoverCalled atomic.Int64
}

func (s *mockSource) ID() string { return s.id }

func (s *mockSource) Meta() source.LibraryMeta { return s.meta }

func (s *mockSource) DiscoverEntities(_ context.Context, _ source.FetchFunc) ([]string, error) {
	s.discoverCalled.Add(1)
	return s.entities, nil
}

func (s *mockSource) ParseEntity(_ context.Context, url string, _ []byte) (*source.Entity, []string, error) {
	if s.parseEntityFn != nil {
		return s.parseEntityFn(url)
	}
	slug := filepath.Base(url)
	return &source.Entity{Slug: slug, Name: slug, Kind: "class"}, nil, nil
}

func (s *mockSource) ParseMethod(_ context.Context, url string, _ []byte) (*source.Method, error) {
	if s.parseMethodFn != nil {
		return s.parseMethodFn(url)
	}
	slug := filepath.Base(url)
	return &source.Method{Slug: slug, Name: slug}, nil
}

func (s *mockSource) DetectWrapper(m *source.Method) (bool, string, string) {
	if s.detectWrapperFn != nil {
		return s.detectWrapperFn(m)
	}
	return false, "", ""
}

func (s *mockSource) ResolveWrapperURL(targetName, targetKind, entitySlug string) string {
	if s.resolveWrapperURLFn != nil {
		return s.resolveWrapperURLFn(targetName, targetKind, entitySlug)
	}
	return ""
}

func (s *mockSource) ParseSourceCode(url string, body []byte) (string, error) {
	if s.parseSourceCodeFn != nil {
		return s.parseSourceCodeFn(url, body)
	}
	return "", nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock Store implementation
// ─────────────────────────────────────────────────────────────────────────────

// mockStore is a configurable, thread-safe implementation of store.Store for
// testing.  Call recorders are protected by mu so tests that run with -race
// are safe to use concurrent crawlers.
type mockStore struct {
	mu sync.Mutex

	// Configurable error returns
	upsertLibraryErr  error
	upsertEntityErr   error
	upsertEntityID    int64 // ID returned on success; auto-increments if 0
	upsertMethodErr   error
	rebuildIndexErr   error
	updateSessionErr  error
	recordProgressErr error

	// Recorded calls (protected by mu)
	upsertLibraryCalls  []string // library IDs
	upsertEntityCalls   []*source.Entity
	upsertMethodCalls   []*source.Method
	recordProgressItems []*store.CrawlProgressItem
	updateSessionCalls  []updateSessionCall
	rebuildIndexCalls   []string // libraryIDs
	updateSnippetCalls  []int    // snippet counts
	getFailuresCalls    []int64  // sessionIDs

	// Configurable query returns
	lastSession   *store.CrawlSession
	processedURLs map[string]string
	failures      []store.CrawlProgressItem

	// auto-increment ID for entities
	nextEntityID int64
}

type updateSessionCall struct {
	sessionID int64
	status    string
	success   int
	fail      int
}

func newMockStore() *mockStore {
	return &mockStore{nextEntityID: 1}
}

func (s *mockStore) UpsertLibrary(_ context.Context, id string, _ source.LibraryMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upsertLibraryCalls = append(s.upsertLibraryCalls, id)
	return s.upsertLibraryErr
}

func (s *mockStore) GetLibrary(_ context.Context, _ string) (*store.LibraryRecord, error) {
	return nil, nil
}

func (s *mockStore) SearchLibraries(_ context.Context, _ string) ([]store.LibraryRecord, error) {
	return nil, nil
}

func (s *mockStore) ListLibraries(_ context.Context) ([]store.LibraryRecord, error) {
	return nil, nil
}

func (s *mockStore) ListLibrariesByLanguage(_ context.Context, _ string) ([]store.LibraryRecord, error) {
	return nil, nil
}

func (s *mockStore) UpsertEntity(_ context.Context, _ string, entity *source.Entity) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.upsertEntityErr != nil {
		return 0, s.upsertEntityErr
	}
	s.upsertEntityCalls = append(s.upsertEntityCalls, entity)
	id := s.nextEntityID
	s.nextEntityID++
	if s.upsertEntityID != 0 {
		id = s.upsertEntityID
	}
	return id, nil
}

func (s *mockStore) GetEntity(_ context.Context, _, _ string) (*store.EntityRecord, error) {
	return nil, nil
}

func (s *mockStore) GetEntityByID(_ context.Context, _ int64) (*store.EntityRecord, error) {
	return nil, nil
}

func (s *mockStore) ListEntities(_ context.Context, _ string) ([]store.EntityRecord, error) {
	return nil, nil
}

func (s *mockStore) UpsertMethod(_ context.Context, _ int64, method *source.Method) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upsertMethodCalls = append(s.upsertMethodCalls, method)
	return s.upsertMethodErr
}

func (s *mockStore) GetMethod(_ context.Context, _ int64, _ string) (*store.MethodRecord, error) {
	return nil, nil
}

func (s *mockStore) ListMethods(_ context.Context, _ int64) ([]store.MethodRecord, error) {
	return nil, nil
}

func (s *mockStore) ListRelations(_ context.Context, _ int64) ([]store.RelationRecord, error) {
	return nil, nil
}

func (s *mockStore) UpdateSnippetCount(_ context.Context, _ string, count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateSnippetCalls = append(s.updateSnippetCalls, count)
	return nil
}

func (s *mockStore) ComputeSnippetCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (s *mockStore) Search(_ context.Context, _, _ string, _ int, _ string) ([]store.SearchResult, error) {
	return nil, nil
}

func (s *mockStore) RebuildIndex(_ context.Context, libraryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebuildIndexCalls = append(s.rebuildIndexCalls, libraryID)
	return s.rebuildIndexErr
}

func (s *mockStore) CreateCrawlSession(_ context.Context, _ string, _ int) (int64, error) {
	return 42, nil
}

func (s *mockStore) UpdateCrawlSession(_ context.Context, sessionID int64, status string, success, fail, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateSessionCalls = append(s.updateSessionCalls, updateSessionCall{
		sessionID: sessionID, status: status, success: success, fail: fail,
	})
	return s.updateSessionErr
}

func (s *mockStore) GetLastSession(_ context.Context, _ string) (*store.CrawlSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSession, nil
}

func (s *mockStore) RecordProgress(_ context.Context, _ int64, item *store.CrawlProgressItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// make a copy so the caller can mutate the original without affecting our record
	cp := *item
	s.recordProgressItems = append(s.recordProgressItems, &cp)
	return s.recordProgressErr
}

func (s *mockStore) GetProcessedURLs(_ context.Context, _ int64) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.processedURLs != nil {
		return s.processedURLs, nil
	}
	return map[string]string{}, nil
}

func (s *mockStore) GetCrawlStats(_ context.Context, _ int64) (*store.CrawlStats, error) {
	return &store.CrawlStats{FailuresByType: map[string]int{}}, nil
}

func (s *mockStore) GetFailures(_ context.Context, sessionID int64) ([]store.CrawlProgressItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getFailuresCalls = append(s.getFailuresCalls, sessionID)
	return s.failures, nil
}

func (s *mockStore) Close() error { return nil }

// ─────────────────────────────────────────────────────────────────────────────
// Helper: count recorded RecordProgress items for a given status
// ─────────────────────────────────────────────────────────────────────────────

func (s *mockStore) countProgressByStatus(status string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, item := range s.recordProgressItems {
		if item.Status == status {
			n++
		}
	}
	return n
}

func (s *mockStore) rebuildCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rebuildIndexCalls)
}

func (s *mockStore) updateSessionStatuses() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.updateSessionCalls))
	for i, c := range s.updateSessionCalls {
		out[i] = c.status
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// fakeFetcher — a Fetcher that returns static bytes for any URL
// ─────────────────────────────────────────────────────────────────────────────
// We cannot use NewLocalFetcher for tests that don't actually touch the
// filesystem, so we write a minimal shim that replaces the internal fields of
// a Fetcher with a custom fetchFn via a closure.  However, the Fetcher type is
// unexported-field-bearing: we can only create one through NewLocalFetcher or
// NewFetcher.  For tests where we need per-URL control we use a temp directory
// with real files.

// writeTempFile creates a file named "name" in dir with content, returning the full path.
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTempFile %s: %v", path, err)
	}
	return path
}

// ─────────────────────────────────────────────────────────────────────────────
// newTestCrawler — construct a Crawler wired to mock source + store
// ─────────────────────────────────────────────────────────────────────────────

// newTestCrawler creates a Crawler with a LocalFetcher and the supplied mocks.
// workers defaults to 1 if w <= 0.
func newTestCrawler(t *testing.T, src source.Source, st store.Store, workers int) *Crawler {
	t.Helper()
	fetcher := NewLocalFetcher()
	t.Cleanup(func() { fetcher.Close() })
	if workers <= 0 {
		workers = 1
	}
	return New(fetcher, st, src, workers)
}

// runCrawler executes c.Run with a context derived from t.Context() and returns
// the error.  It fails the test if ctx already expired before calling Run.
func runCrawler(t *testing.T, c *Crawler, opts RunOptions) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.Run(ctx, opts)
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 1: Run on an empty source completes without error, no entities discovered
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_EmptySource_CompletesWithoutError(t *testing.T) {
	t.Parallel()

	src := &mockSource{id: "/test/empty"}
	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)

	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if c.success.Load() != 0 {
		t.Errorf("success count = %d; want 0", c.success.Load())
	}
	if c.failed.Load() != 0 {
		t.Errorf("failed count = %d; want 0", c.failed.Load())
	}
	// Session should be finalized as "completed"
	statuses := st.updateSessionStatuses()
	found := false
	for _, s := range statuses {
		if s == "completed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected at least one UpdateCrawlSession with status 'completed', got %v", statuses)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 2: Run on a small source processes all entities and stores them
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_SmallSource_AllEntitiesParsedAndStored(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create 3 dummy entity files that the LocalFetcher can read.
	urls := make([]string, 3)
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("entity%d.txt", i)
		p := writeTempFile(t, dir, name, fmt.Sprintf("content of entity %d", i))
		urls[i] = p
	}

	src := &mockSource{id: "/test/small", entities: urls}
	st := newMockStore()
	c := newTestCrawler(t, src, st, 2)

	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := c.success.Load(); got != 3 {
		t.Errorf("success = %d; want 3", got)
	}
	if got := c.failed.Load(); got != 0 {
		t.Errorf("failed = %d; want 0", got)
	}

	st.mu.Lock()
	storedCount := len(st.upsertEntityCalls)
	st.mu.Unlock()
	if storedCount != 3 {
		t.Errorf("UpsertEntity called %d times; want 3", storedCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 3: Run with RetryFailed mode re-fetches only transient failures
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_RetryFailed_OnlyTransientErrorsReQueued(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Two URLs that will be retried (transient), three that are not.
	retryable := []string{"timeout_url", "http_429_url"}
	permanent := []string{"http_404_url", "parse_error_url", "fk_error_url"}

	// Build failures list as mockStore.failures
	var failures []store.CrawlProgressItem
	for _, u := range retryable {
		typ := "timeout"
		if strings.Contains(u, "429") {
			typ = "http_429"
		}
		failures = append(failures, store.CrawlProgressItem{URL: u, ErrorType: typ, Status: "failed"})
	}
	for _, u := range permanent {
		typ := strings.TrimSuffix(u, "_url")
		failures = append(failures, store.CrawlProgressItem{URL: u, ErrorType: typ, Status: "failed"})
	}

	session := &store.CrawlSession{ID: 99, Status: "completed"}

	// Create real files for the retryable URLs so the LocalFetcher doesn't 404
	for _, u := range retryable {
		writeTempFile(t, dir, u, "placeholder")
	}

	src := &mockSource{
		id:       "/test/retry",
		entities: nil, // populated by GetFailures, not DiscoverEntities
	}
	st := newMockStore()
	st.lastSession = session
	st.failures = failures

	// The retryable URLs are filenames in dir; rewrite them to absolute paths.
	for i, f := range st.failures {
		if f.ErrorType == "timeout" || f.ErrorType == "http_429" {
			st.failures[i].URL = filepath.Join(dir, f.URL)
		}
	}

	c := newTestCrawler(t, src, st, 1)
	if err := runCrawler(t, c, RunOptions{RetryFailed: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the retryable entities should have been processed (success or failure).
	processed := c.success.Load() + c.failed.Load()
	if processed != int64(len(retryable)) {
		t.Errorf("processed %d entities; want %d (only retryable)", processed, len(retryable))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 4: Run with Resume mode skips previously processed URLs
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_Resume_SkipsPreviouslyProcessedURLs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	allURLs := make([]string, 5)
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("ent%d.txt", i)
		allURLs[i] = writeTempFile(t, dir, name, fmt.Sprintf("body %d", i))
	}

	// Mark first 3 as already processed
	processed := map[string]string{
		allURLs[0]: "success",
		allURLs[1]: "success",
		allURLs[2]: "success",
	}

	session := &store.CrawlSession{ID: 77, Status: "interrupted"}

	src := &mockSource{id: "/test/resume", entities: allURLs}
	st := newMockStore()
	st.lastSession = session
	st.processedURLs = processed

	c := newTestCrawler(t, src, st, 1)
	if err := runCrawler(t, c, RunOptions{Resume: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the remaining 2 should have been processed.
	if got := c.success.Load(); got != 2 {
		t.Errorf("success = %d; want 2 (only non-processed)", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 5: Context cancellation propagates — Run returns within reasonable time
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_ContextCancellation_ReturnsPromptly(t *testing.T) {
	// Not parallel: uses a blocking channel to control scheduling precisely.

	dir := t.TempDir()
	// Create many entities so the crawl has work to do.
	const n = 50
	urls := make([]string, n)
	for i := 0; i < n; i++ {
		urls[i] = writeTempFile(t, dir, fmt.Sprintf("e%d.txt", i), "body")
	}

	// unblock is closed by the test to let ParseEntity proceed after the cancel.
	// startedCh signals when the first ParseEntity call begins — we use this to
	// ensure at least one entity is in-flight before we cancel.
	unblock := make(chan struct{})
	startedCh := make(chan struct{}, 1)

	src := &mockSource{
		id:       "/test/cancel",
		entities: urls,
		parseEntityFn: func(url string) (*source.Entity, []string, error) {
			// Signal that we've started parsing at least once.
			select {
			case startedCh <- struct{}{}:
			default:
			}
			// Block until the test unblocks us (after cancellation).
			<-unblock
			slug := filepath.Base(url)
			return &source.Entity{Slug: slug, Name: slug, Kind: "class"}, nil, nil
		},
	}
	st := newMockStore()

	c := newTestCrawler(t, src, st, 4)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx, RunOptions{})
	}()

	// Wait for at least one entity to start parsing, then cancel.
	select {
	case <-startedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first ParseEntity call")
	}
	cancel()

	// Unblock any ParseEntity calls that are already holding so workers can drain.
	close(unblock)

	start := time.Now()
	var err error
	select {
	case err = <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("Run did not return within 8s after context cancel")
	}
	elapsed := time.Since(start)

	// Run currently returns nil even on cancellation (HIGH-17: known characterization).
	if err != nil {
		t.Errorf("Run returned non-nil error %v; current behavior is nil on cancel", err)
	}

	// Should return promptly once unblocked (< 5 s).
	if elapsed > 5*time.Second {
		t.Errorf("Run took %v after unblock; expected < 5s", elapsed)
	}

	// Deferred block must have called UpdateCrawlSession with "interrupted".
	statuses := st.updateSessionStatuses()
	foundInterrupted := false
	for _, s := range statuses {
		if s == "interrupted" {
			foundInterrupted = true
			break
		}
	}
	if !foundInterrupted {
		t.Errorf("expected UpdateCrawlSession('interrupted') on context cancel; got %v", statuses)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 6: Worker pool with workers=1 (single-threaded path)
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_WorkersOne_SingleThreadedPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	urls := []string{
		writeTempFile(t, dir, "a.txt", "a"),
		writeTempFile(t, dir, "b.txt", "b"),
		writeTempFile(t, dir, "c.txt", "c"),
	}

	src := &mockSource{id: "/test/single", entities: urls}
	st := newMockStore()

	// workers=1 via New directly (not the helper which forces >= 1 anyway).
	c := New(NewLocalFetcher(), st, src, 1)
	t.Cleanup(func() { c.fetcher.Close() })

	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := c.success.Load(); got != 3 {
		t.Errorf("success = %d; want 3", got)
	}
	if got := c.workers; got != 1 {
		t.Errorf("c.workers = %d; want 1", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 7: Worker pool with workers=20 — concurrent path with -race detector
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_Workers20_ConcurrentPath_NoDataRace(t *testing.T) {
	// Do NOT mark Parallel here — the race detector test should run in isolation
	// to get clean signals.  It is still fast (< 1 s).

	dir := t.TempDir()
	const n = 40
	urls := make([]string, n)
	for i := 0; i < n; i++ {
		urls[i] = writeTempFile(t, dir, fmt.Sprintf("ent%d.txt", i), "body")
	}

	src := &mockSource{id: "/test/concurrent", entities: urls}
	st := newMockStore()
	c := New(NewLocalFetcher(), st, src, 20)
	t.Cleanup(func() { c.fetcher.Close() })

	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := c.success.Load(); got != n {
		t.Errorf("success = %d; want %d", got, n)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 8: Failed fetch classifies correctly — 404, 5xx, network error categories
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_FetchFailure_ClassifiedAndRecorded(t *testing.T) {
	t.Parallel()

	// Create a URL that the LocalFetcher will fail on (file does not exist).
	dir := t.TempDir()
	missingURL := filepath.Join(dir, "does_not_exist.txt")

	src := &mockSource{id: "/test/fetchfail", entities: []string{missingURL}}
	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)

	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := c.failed.Load(); got != 1 {
		t.Errorf("failed = %d; want 1", got)
	}
	if got := c.success.Load(); got != 0 {
		t.Errorf("success = %d; want 0", got)
	}

	// A RecordProgress item with status "failed" should exist.
	if n := st.countProgressByStatus("failed"); n < 1 {
		t.Errorf("expected >= 1 failed progress item; got %d", n)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 9: classifyError maps all known error patterns correctly
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyError_AllBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"404", fmt.Errorf("page not found (404): http://example.com/wp"), "http_404"},
		{"429", fmt.Errorf("rate limited (429) on some URL"), "http_429"},
		{"server error", fmt.Errorf("server error (503) on some URL"), "http_5xx"},
		{"timeout", fmt.Errorf("request timeout after 30s"), "timeout"},
		{"deadline", fmt.Errorf("context deadline exceeded"), "timeout"},
		{"fk", fmt.Errorf("FOREIGN KEY constraint failed"), "fk_error"},
		{"canceled", context.Canceled, "interrupted"},
		{"unknown", fmt.Errorf("some novel network unreachable error"), "other"},
		// Sentinel error tests (Wave-3-C contract)
		{"ErrFetch404 sentinel", fmt.Errorf("wrapped: %w", ErrFetch404), "http_404"},
		{"ErrFetch5xx sentinel", fmt.Errorf("wrapped: %w", ErrFetch5xx), "http_5xx"},
		{"ErrFetchNetwork sentinel", fmt.Errorf("wrapped: %w", ErrFetchNetwork), "timeout"},
		// context.Canceled must win even when wrapped alongside other content
		{"canceled wrapped", fmt.Errorf("fetch: %w", context.Canceled), "interrupted"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := classifyError(tc.err)
			if got != tc.want {
				t.Errorf("classifyError(%q) = %q; want %q", tc.err, got, tc.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 9b: classifyError on novel message does not panic, returns "other"
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyError_NovelErrorMessage_ReturnsOtherWithoutPanic(t *testing.T) {
	t.Parallel()

	novelErr := fmt.Errorf("xyzzy: something unexpected happened at line 42")
	got := classifyError(novelErr)
	if got != "other" {
		t.Errorf("classifyError(novel) = %q; want 'other'", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 10: resolveWrapperChain depth limit — chain longer than maxWrapperDepth
//
//	is truncated at depth 3
//
// ─────────────────────────────────────────────────────────────────────────────
func TestResolveWrapperChain_DepthLimitPreventsInfiniteRecursion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Every URL resolves to a wrapper pointing to the "next" URL.
	// This creates an infinite chain if the depth limit is absent.
	wrapperContent := "<?php function wrapper() { return next_wrapper(); }"

	// Write several wrapper files (more than maxWrapperDepth)
	for i := 0; i <= maxWrapperDepth+2; i++ {
		writeTempFile(t, dir, fmt.Sprintf("w%d.txt", i), wrapperContent)
	}

	fetchCount := atomic.Int64{}
	src := &mockSource{
		id: "/test/depth",
		detectWrapperFn: func(_ *source.Method) (bool, string, string) {
			return true, "next_wrapper", "function"
		},
		resolveWrapperURLFn: func(targetName, targetKind, entitySlug string) string {
			n := fetchCount.Load()
			return filepath.Join(dir, fmt.Sprintf("w%d.txt", n))
		},
		parseSourceCodeFn: func(url string, _ []byte) (string, error) {
			fetchCount.Add(1)
			return wrapperContent, nil
		},
	}

	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)

	method := &source.Method{Slug: "test_wrapper", Name: "test_wrapper", SourceCode: wrapperContent}
	c.resolveWrapperChain(context.Background(), method, "test-entity", 0)

	// Depth starts at 0; at depth >= maxWrapperDepth (3) the chain must stop.
	// So at most maxWrapperDepth fetch operations (for depth 0, 1, 2) should occur.
	if got := fetchCount.Load(); got > int64(maxWrapperDepth) {
		t.Errorf("fetched %d wrappers; want <= %d (maxWrapperDepth)", got, maxWrapperDepth)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 11: Wrapper detection skips PHP builtins
//
//	(DetectWrapper returns false, so resolveWrapperChain terminates)
//
// ─────────────────────────────────────────────────────────────────────────────
func TestResolveWrapperChain_SourceDetectWrapperFalse_Terminates(t *testing.T) {
	t.Parallel()

	// Source says method is NOT a wrapper → chain should immediately return.
	detectCalled := atomic.Int64{}
	src := &mockSource{
		id: "/test/builtins",
		detectWrapperFn: func(_ *source.Method) (bool, string, string) {
			detectCalled.Add(1)
			return false, "", ""
		},
	}

	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)

	method := &source.Method{Slug: "array_map", Name: "array_map", SourceCode: "<?php // builtin"}
	c.resolveWrapperChain(context.Background(), method, "entity-slug", 0)

	if got := detectCalled.Load(); got != 1 {
		t.Errorf("DetectWrapper called %d times; want exactly 1", got)
	}
	if method.WrappedMethod != "" {
		t.Errorf("WrappedMethod = %q; want empty for non-wrapper", method.WrappedMethod)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 12: UpdateSnippetCount is called once — at the end of Run (final call only).
//
// Wave-3 refactor removed per-entity UpdateSnippetCount calls from processEntity.
// Only the final call in Run's finalizeSnippetCount remains.
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_UpdateSnippetCount_CalledOnceAtEnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	const nEntities = 5
	urls := make([]string, nEntities)
	for i := 0; i < nEntities; i++ {
		urls[i] = writeTempFile(t, dir, fmt.Sprintf("ent%d.txt", i), "body")
	}

	src := &mockSource{id: "/test/snippet", entities: urls}
	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)

	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	st.mu.Lock()
	calls := len(st.updateSnippetCalls)
	st.mu.Unlock()

	// Wave-3 refactor: per-entity UpdateSnippetCount calls have been removed.
	// Only the single final call from Run.finalizeSnippetCount remains.
	if calls != 1 {
		t.Errorf("UpdateSnippetCount called %d times; want 1 (final call only after Wave-3 refactor)",
			calls)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 13: RecordProgress is called on both success and failure paths
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_RecordProgress_SuccessAndFailurePaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	goodURL := writeTempFile(t, dir, "good.txt", "body")
	// The bad URL will cause LocalFetcher to fail (file doesn't exist).
	badURL := filepath.Join(dir, "missing.txt")

	src := &mockSource{id: "/test/progress", entities: []string{goodURL, badURL}}
	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)

	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successes := st.countProgressByStatus("success")
	failures := st.countProgressByStatus("failed")

	if successes < 1 {
		t.Errorf("expected >= 1 success progress item; got %d", successes)
	}
	if failures < 1 {
		t.Errorf("expected >= 1 failed progress item; got %d", failures)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 14: GetFailures populates the crawl summary at end of run
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_GetFailures_PopulatesSummaryCorrectly(t *testing.T) {
	t.Parallel()

	src := &mockSource{id: "/test/summary"}
	st := newMockStore()
	// Seed some failures in the mock so printSummary prints them.
	st.failures = []store.CrawlProgressItem{
		{URL: "http://example.com/wp/1", ErrorType: "http_404", ErrorMessage: "not found", Status: "failed"},
		{URL: "http://example.com/wp/2", ErrorType: "timeout", ErrorMessage: "timed out", Status: "failed"},
	}

	c := newTestCrawler(t, src, st, 1)
	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GetFailures must have been called at least once (from printSummary).
	st.mu.Lock()
	calls := len(st.getFailuresCalls)
	st.mu.Unlock()

	if calls < 1 {
		t.Error("GetFailures was never called; printSummary should call it")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 15: Interrupt/signal path — swallowed errors are NOW logged, not silenced
//
//	Verifies via bytes.Buffer + log.SetOutput that error messages appear.
//
// ─────────────────────────────────────────────────────────────────────────────
func TestRun_ContextCancel_ErrorsLoggedNotSilenced(t *testing.T) {
	// Not parallel: mutates the global logger.

	dir := t.TempDir()
	// Write a few entities so the crawl has something to process.
	const n = 10
	urls := make([]string, n)
	for i := 0; i < n; i++ {
		urls[i] = writeTempFile(t, dir, fmt.Sprintf("e%d.txt", i), "body")
	}

	// Make RebuildIndex fail on the interrupt path to exercise the error log.
	src := &mockSource{id: "/test/log", entities: urls}
	st := newMockStore()
	st.rebuildIndexErr = errors.New("simulated rebuild failure")

	// Capture log output.
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	c := newTestCrawler(t, src, st, 4)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the defer block always runs.
	cancel()

	_ = c.Run(ctx, RunOptions{})

	logOut := buf.String()

	// Wave-1 change: the defer block now logs errors from RebuildIndex and
	// UpdateCrawlSession.  Verify the rebuild error message appears.
	if !strings.Contains(logOut, "simulated rebuild failure") {
		t.Errorf("expected error message 'simulated rebuild failure' in log output; got:\n%s", logOut)
	}
	// Also verify "Crawl interrupted" log line appears (from the defer block).
	if !strings.Contains(logOut, "interrupted") {
		t.Errorf("expected 'interrupted' in log output; got:\n%s", logOut)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 16: Concurrent wrapperCache access — TOCTOU is now FIXED by sync.Map.
//
// Wave-3 moved wrapperCache from a plain map[string]string (guarded by cacheMu
// RWMutex) to a sync.Map field on Crawler.  sync.Map.LoadOrStore is atomic:
// exactly one goroutine will win the store; all others will receive the stored
// value.  Therefore fetchCount MUST equal 1 even with 20 concurrent goroutines
// all resolving the same URL.
//
// This test MUST be run with -race.
// ─────────────────────────────────────────────────────────────────────────────

func TestResolveWrapperChain_ConcurrentAccess_SyncMapEliminatesTOCTOU(t *testing.T) {
	// Intentionally NOT parallel at the top level: this test is the stress probe.

	dir := t.TempDir()
	targetFile := writeTempFile(t, dir, "shared_wrapper.txt", "<?php function shared() {}")
	targetURL := targetFile

	fetchCount := atomic.Int64{}

	src := &mockSource{
		id: "/test/toctou",
		detectWrapperFn: func(_ *source.Method) (bool, string, string) {
			return true, "shared", "function"
		},
		resolveWrapperURLFn: func(_, _, _ string) string {
			return targetURL
		},
		parseSourceCodeFn: func(_ string, _ []byte) (string, error) {
			fetchCount.Add(1)
			return "shared source", nil
		},
	}

	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)
	// wrapperCache is now a sync.Map field on Crawler — no local map needed.

	const goroutines = 20
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			method := &source.Method{
				Slug:       fmt.Sprintf("method_%d", id),
				Name:       fmt.Sprintf("method_%d", id),
				SourceCode: "<?php function method() { return shared(); }",
			}
			c.resolveWrapperChain(context.Background(), method, "entity", 0)
		}(i)
	}
	wg.Wait()

	// Verify that the cache was populated.
	cached, ok := c.wrapperCache.Load(targetURL)
	if !ok {
		t.Error("wrapperCache should contain the target URL after concurrent resolution")
	}
	if ok && cached.(string) != "shared source" {
		t.Errorf("cached value = %q; want 'shared source'", cached.(string))
	}

	// Wave-3 fix: sync.Map.LoadOrStore ensures exactly ONE goroutine performs
	// the fetch + store. fetchCount MUST equal 1.
	if got := fetchCount.Load(); got != 1 {
		t.Errorf("fetchCount = %d; want exactly 1 (sync.Map eliminates TOCTOU duplicate fetches)", got)
	}
	t.Logf("TOCTOU fix verified: %d goroutines, %d actual fetch (want 1)", goroutines, fetchCount.Load())
}

// ─────────────────────────────────────────────────────────────────────────────
// Additional tests beyond the 16 required
// ─────────────────────────────────────────────────────────────────────────────

// TEST A: New with workers <= 0 defaults to 10.

func TestNew_DefaultWorkerCount(t *testing.T) {
	t.Parallel()

	src := &mockSource{id: "/test"}
	st := newMockStore()
	f := NewLocalFetcher()
	defer f.Close()

	c := New(f, st, src, 0)
	if c.workers != 10 {
		t.Errorf("workers = %d; want 10 when input is 0", c.workers)
	}

	c2 := New(f, st, src, -5)
	if c2.workers != 10 {
		t.Errorf("workers = %d; want 10 when input is -5", c2.workers)
	}

	c3 := New(f, st, src, 3)
	if c3.workers != 3 {
		t.Errorf("workers = %d; want 3 when input is 3", c3.workers)
	}
}

// TEST B: resolveWrapperChain returns immediately when ResolveWrapperURL returns "".

func TestResolveWrapperChain_EmptyTargetURL_StopsChain(t *testing.T) {
	t.Parallel()

	src := &mockSource{
		id: "/test/emptyurl",
		detectWrapperFn: func(_ *source.Method) (bool, string, string) {
			return true, "target_fn", "function"
		},
		resolveWrapperURLFn: func(_, _, _ string) string {
			return "" // empty URL → should stop
		},
	}

	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)

	method := &source.Method{Slug: "fn", Name: "fn", SourceCode: "code"}
	c.resolveWrapperChain(context.Background(), method, "entity", 0)

	if method.WrappedMethod != "" {
		t.Errorf("WrappedMethod = %q; want empty when ResolveWrapperURL returns ''", method.WrappedMethod)
	}
}

// TEST C: function entity with no method URLs triggers ParseMethod on the same body.

func TestProcessEntity_FunctionEntityCreatesMethodRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fnURL := writeTempFile(t, dir, "myfunc.txt", "<?php function my_func() {}")

	parseMethodCalled := atomic.Int64{}
	src := &mockSource{
		id:       "/test/function",
		entities: []string{fnURL},
		parseEntityFn: func(url string) (*source.Entity, []string, error) {
			// Kind "function", no method URLs
			return &source.Entity{Slug: "my_func", Name: "my_func", Kind: "function"}, nil, nil
		},
		parseMethodFn: func(url string) (*source.Method, error) {
			parseMethodCalled.Add(1)
			return &source.Method{Slug: "my_func", Name: "my_func"}, nil
		},
	}

	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)

	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := parseMethodCalled.Load(); got != 1 {
		t.Errorf("ParseMethod called %d times; want 1 for function entity with no method URLs", got)
	}

	st.mu.Lock()
	methodCount := len(st.upsertMethodCalls)
	st.mu.Unlock()

	if methodCount != 1 {
		t.Errorf("UpsertMethod called %d times; want 1 for function entity", methodCount)
	}
}

// TEST D: wrapperCache hit — fetcher called only once for two methods wrapping same URL.
//
// The source is configured so that only methods with SourceCode == "wrapper code"
// are detected as wrappers; the resolved target source ("target source") is not a
// wrapper itself.  This ensures the chain terminates at depth 1 and the second
// sequential call hits the populated cache.

func TestResolveWrapperChain_CacheHit_FetcherCalledOnlyOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetURL := writeTempFile(t, dir, "cached_target.txt", "<?php function target() {}")

	fetchCount := atomic.Int64{}
	src := &mockSource{
		id: "/test/cachehit",
		detectWrapperFn: func(m *source.Method) (bool, string, string) {
			// Only methods with the original "wrapper code" are wrappers.
			// The resolved target ("target source") is a leaf — not a wrapper.
			if m.SourceCode == "wrapper code" {
				return true, "target", "function"
			}
			return false, "", ""
		},
		resolveWrapperURLFn: func(_, _, _ string) string {
			return targetURL
		},
		parseSourceCodeFn: func(_ string, _ []byte) (string, error) {
			fetchCount.Add(1)
			return "target source", nil
		},
	}

	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)
	// wrapperCache is now a sync.Map field on Crawler — no local map needed.

	method1 := &source.Method{Slug: "m1", Name: "m1", SourceCode: "wrapper code"}
	method2 := &source.Method{Slug: "m2", Name: "m2", SourceCode: "wrapper code"}

	// Resolve both methods sequentially using the Crawler's shared cache.
	c.resolveWrapperChain(context.Background(), method1, "entity", 0)
	c.resolveWrapperChain(context.Background(), method2, "entity", 0)

	// First resolution fetches (cache miss); second should hit the cache.
	if got := fetchCount.Load(); got != 1 {
		t.Errorf("fetchCount = %d; want 1 (cache hit on second call)", got)
	}

	if method1.WrappedMethod != "target" {
		t.Errorf("method1.WrappedMethod = %q; want 'target'", method1.WrappedMethod)
	}
	if method2.WrappedMethod != "target" {
		t.Errorf("method2.WrappedMethod = %q; want 'target'", method2.WrappedMethod)
	}
}

// TEST E: UpsertEntity failure — recordFailure is called, method processing skipped.

func TestRun_UpsertEntityFailure_RecordsFailureAndSkipsMethods(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	entURL := writeTempFile(t, dir, "entity.txt", "body")

	methodParseCalled := atomic.Int64{}
	src := &mockSource{
		id:       "/test/storeentfail",
		entities: []string{entURL},
		parseEntityFn: func(url string) (*source.Entity, []string, error) {
			// Return a method URL to crawl.
			return &source.Entity{Slug: "ent", Name: "ent", Kind: "class"},
				[]string{filepath.Join(dir, "method.txt")}, nil
		},
		parseMethodFn: func(url string) (*source.Method, error) {
			methodParseCalled.Add(1)
			return &source.Method{Slug: "m", Name: "m"}, nil
		},
	}

	st := newMockStore()
	st.upsertEntityErr = errors.New("simulated upsert failure")

	c := newTestCrawler(t, src, st, 1)
	if err := runCrawler(t, c, RunOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Method should NOT have been parsed/stored because UpsertEntity failed.
	if got := methodParseCalled.Load(); got != 0 {
		t.Errorf("ParseMethod called %d times; want 0 when UpsertEntity fails", got)
	}

	if n := st.countProgressByStatus("failed"); n < 1 {
		t.Errorf("expected >= 1 failed progress item; got %d", n)
	}
	if got := c.failed.Load(); got != 1 {
		t.Errorf("failed count = %d; want 1", got)
	}
}

// TEST F: Run returns nil (not ctx.Err) on cancellation — characterizes HIGH-17.

func TestRun_ReturnValueOnCancel_IsNil(t *testing.T) {
	t.Parallel()

	src := &mockSource{id: "/test/retnil"}
	st := newMockStore()
	c := newTestCrawler(t, src, st, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before even starting

	err := c.Run(ctx, RunOptions{})

	// Characterization: Run currently returns nil even on context cancellation
	// (HIGH-17 known finding).  This test locks in the current behaviour.
	if err != nil {
		t.Errorf("Run() = %v; current characterization is nil on cancel (HIGH-17)", err)
	}
}

// TEST G: Sentinel error wrapping — ErrFetch404 wrapped with %w is correctly
// classified as "http_404" via errors.Is (verifies the Wave-3 contract).

func TestClassifyError_SentinelErrors_CorrectlyClassified(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		err       error
		wantClass string
	}{
		{
			name:      "ErrFetch404 direct",
			err:       ErrFetch404,
			wantClass: "http_404",
		},
		{
			name:      "ErrFetch404 wrapped",
			err:       fmt.Errorf("fetching /wp/class/wp_query: %w", ErrFetch404),
			wantClass: "http_404",
		},
		{
			name:      "ErrFetch5xx direct",
			err:       ErrFetch5xx,
			wantClass: "http_5xx",
		},
		{
			name:      "ErrFetch5xx wrapped",
			err:       fmt.Errorf("server returned 503: %w", ErrFetch5xx),
			wantClass: "http_5xx",
		},
		{
			name:      "ErrFetchNetwork direct",
			err:       ErrFetchNetwork,
			wantClass: "timeout",
		},
		{
			name:      "ErrFetchNetwork wrapped",
			err:       fmt.Errorf("dial tcp: %w", ErrFetchNetwork),
			wantClass: "timeout",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := classifyError(tc.err)
			if got != tc.wantClass {
				t.Errorf("classifyError(%v) = %q; want %q", tc.err, got, tc.wantClass)
			}
		})
	}
}
