//go:build sqlite_fts5 || fts5

package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newHTTPTestServer starts an httptest.Server using the provided handler and
// registers a cleanup function to close it when the test finishes.
func newHTTPTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// newLocalFetcherWithFiles creates a t.TempDir(), writes the provided files
// into it, and returns a NewLocalFetcher together with the temp directory path
// so callers can build absolute file paths.
//
// The files map keys are path names relative to the temp directory (e.g.
// "test.php"). The values are the file contents.
func newLocalFetcherWithFiles(t *testing.T, files map[string]string) (*LocalFetcher, string) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("newLocalFetcherWithFiles: mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("newLocalFetcherWithFiles: write %s: %v", full, err)
		}
	}
	return NewLocalFetcher(), dir
}

// ---------------------------------------------------------------------------
// Test 1 — NewFetcher + Fetch HTTP success: 200 OK, body returned correctly
// ---------------------------------------------------------------------------

func TestNewFetcher_FetchHTTPSuccess(t *testing.T) {
	const want = "hello from wordpress"

	srv := newHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, want)
	})

	// rps=100 to avoid waiting during the test; maxRetry=0; empty user-agent.
	f := NewHTTPFetcher(100, 0, "test-agent")
	defer f.Close()

	ctx := context.Background()
	got, err := f.Fetch(ctx, srv.URL+"/some-path")
	if err != nil {
		t.Fatalf("Fetch() error = %v; want nil", err)
	}
	if string(got) != want {
		t.Errorf("Fetch() body = %q; want %q", string(got), want)
	}
}

// ---------------------------------------------------------------------------
// Test 2 — Rate limiter via Ticker: 3 fetches at 2 RPS ≥ 1 s elapsed
//
// At 2 RPS the ticker fires every 500 ms. The first tick is consumed
// immediately (ticker pre-fires on creation), so:
//   tick 1 → immediate
//   tick 2 → +500 ms
//   tick 3 → +500 ms
//
// Total elapsed for 3 fetches ≥ 1 s (allowing slack).
// ---------------------------------------------------------------------------

func TestRateLimiter_ThrottlesRequests(t *testing.T) {
	srv := newHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	const rps = 2
	f := NewHTTPFetcher(rps, 0, "")
	defer f.Close()

	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := f.Fetch(ctx, srv.URL); err != nil {
			t.Fatalf("Fetch #%d error = %v", i+1, err)
		}
	}
	elapsed := time.Since(start)

	// 3 fetches at 2 RPS: ticks at t=0, t=500ms, t=1000ms → total ≥ 1s.
	// Allow up to 3 s for slow CI.
	const minElapsed = 900 * time.Millisecond
	if elapsed < minElapsed {
		t.Errorf("3 fetches at %d RPS took %v; want ≥ %v", rps, elapsed, minElapsed)
	}
}

// ---------------------------------------------------------------------------
// Test 3 — Retry on 500: returns error after maxRetry attempts;
// backoff observed via server hit count timing.
// ---------------------------------------------------------------------------

func TestFetch_RetryOn500_ReturnsErrorAfterMaxRetry(t *testing.T) {
	var hitCount atomic.Int32

	srv := newHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})

	const maxRetry = 2
	// Use rps=100 so the rate limiter does not dominate elapsed time.
	f := NewHTTPFetcher(100, maxRetry, "")
	defer f.Close()

	ctx := context.Background()
	start := time.Now()
	_, err := f.Fetch(ctx, srv.URL)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Fetch() error = nil; want non-nil after exhausting retries")
	}
	if !strings.Contains(err.Error(), "retries failed") {
		t.Errorf("error = %q; want it to mention retries", err.Error())
	}

	// maxRetry=2 means 3 total attempts (attempt 0, 1, 2).
	wantHits := int32(maxRetry + 1)
	if got := hitCount.Load(); got != wantHits {
		t.Errorf("server hit count = %d; want %d", got, wantHits)
	}

	// Backoff: attempt 1 sleeps 2^0=1s, attempt 2 sleeps 2^1=2s → ≥ 3s total.
	// Allow generous slack for CI.
	const minElapsed = 2500 * time.Millisecond
	if elapsed < minElapsed {
		t.Errorf("retries took %v; want ≥ %v (backoff observed)", elapsed, minElapsed)
	}
}

// ---------------------------------------------------------------------------
// Test 4 — Context cancellation during backoff:
// ctx canceled while sleeping → Fetch returns ctx.Err() promptly.
// Verifies the Wave-1 time.NewTimer + Stop fix releases the timer.
// ---------------------------------------------------------------------------

func TestFetch_ContextCanceledDuringBackoff(t *testing.T) {
	// First call always returns 500 so a backoff of 1 s is triggered.
	srv := newHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	// maxRetry=3 to ensure we enter the backoff path; rps=100.
	f := NewHTTPFetcher(100, 3, "")
	defer f.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after a short delay so we hit the timer.C / ctx.Done()
	// select inside the backoff sleep. The backoff for attempt 1 is 1 s, so
	// canceling after 100 ms should interrupt it cleanly.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := f.Fetch(ctx, srv.URL)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Fetch() error = nil; want context.Canceled")
	}
	if err != context.Canceled {
		t.Errorf("Fetch() error = %v; want context.Canceled", err)
	}
	// Should return well before the full 1 s backoff completes.
	const maxElapsed = 800 * time.Millisecond
	if elapsed > maxElapsed {
		t.Errorf("Fetch() took %v after cancel; want < %v", elapsed, maxElapsed)
	}
}

// ---------------------------------------------------------------------------
// Test 5 — Context cancellation while waiting for rate-limit tick:
// ctx canceled before ticker fires → Fetch returns ctx.Err() promptly.
// ---------------------------------------------------------------------------

func TestFetch_ContextCanceledWaitingForRateLimitTick(t *testing.T) {
	srv := newHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	// rps=1 → tick every 1 s. We consume the first tick manually so the next
	// Fetch call will have to wait a full second before the ticker fires.
	f := NewHTTPFetcher(1, 0, "")
	defer f.Close()

	// Consume the pre-fired first tick to arm the wait.
	ctx := context.Background()
	if _, err := f.Fetch(ctx, srv.URL); err != nil {
		t.Fatalf("warm-up Fetch error = %v", err)
	}

	// Now cancel the context immediately before the second tick is ready.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel at once — the tick won't fire for ~1 s

	start := time.Now()
	_, err := f.Fetch(cancelCtx, srv.URL)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Fetch() error = nil; want context.Canceled")
	}
	if err != context.Canceled {
		t.Errorf("Fetch() error = %v; want context.Canceled", err)
	}
	// Should return in well under 500 ms.
	const maxElapsed = 400 * time.Millisecond
	if elapsed > maxElapsed {
		t.Errorf("Fetch() took %v after cancel; want < %v", elapsed, maxElapsed)
	}
}

// ---------------------------------------------------------------------------
// Test 6 — NewLocalFetcher Fetch: reads file from disk, strips #fragment
// ---------------------------------------------------------------------------

func TestNewLocalFetcher_FetchReadsFileStripsFragment(t *testing.T) {
	const content = "<?php class WP_Post {}"
	f, dir := newLocalFetcherWithFiles(t, map[string]string{
		"wp-post.php": content,
	})

	// URL with a fragment — the fetcher must strip "#WP_Post" before reading.
	url := filepath.Join(dir, "wp-post.php") + "#WP_Post"
	got, err := f.Fetch(context.Background(), url)
	if err != nil {
		t.Fatalf("Fetch() error = %v; want nil", err)
	}
	if string(got) != content {
		t.Errorf("Fetch() body = %q; want %q", string(got), content)
	}
}

// ---------------------------------------------------------------------------
// Test 7 — NewLocalFetcher fileCache: same URL twice → second call uses cache
// (file mutated on disk between calls; second call still returns original).
// ---------------------------------------------------------------------------

func TestNewLocalFetcher_FileCacheReturnsCachedContent(t *testing.T) {
	const original = "<?php // original"
	const mutated = "<?php // mutated"

	f, dir := newLocalFetcherWithFiles(t, map[string]string{
		"test.php": original,
	})
	absPath := filepath.Join(dir, "test.php")

	ctx := context.Background()

	// First fetch populates the cache.
	got1, err := f.Fetch(ctx, absPath)
	if err != nil {
		t.Fatalf("first Fetch() error = %v", err)
	}
	if string(got1) != original {
		t.Errorf("first Fetch() = %q; want %q", string(got1), original)
	}

	// Mutate the file on disk.
	if err := os.WriteFile(absPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// Second fetch must return cached (original) content, not the mutated file.
	got2, err := f.Fetch(ctx, absPath)
	if err != nil {
		t.Fatalf("second Fetch() error = %v", err)
	}
	if string(got2) != original {
		t.Errorf("second Fetch() = %q; want cached %q (file changed on disk should not affect cache)", string(got2), original)
	}
}

// ---------------------------------------------------------------------------
// Test 8 — Close stops the ticker:
// After Close, the rateLimiter channel is drained/stopped so a subsequent
// Fetch blocks on the ctx.Done() branch. We characterize the behavior by
// verifying that calling Fetch after Close returns quickly when ctx is
// pre-cancelled (the rate limiter select falls through to ctx.Done()).
// ---------------------------------------------------------------------------

func TestClose_StopsTheTicker(t *testing.T) {
	srv := newHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	// rps=1 so each tick is spaced 1 s apart.
	f := NewHTTPFetcher(1, 0, "")

	// Consume the first (immediate) tick.
	ctx := context.Background()
	if _, err := f.Fetch(ctx, srv.URL); err != nil {
		t.Fatalf("warm-up Fetch error = %v", err)
	}

	// Stop the ticker — no more ticks will fire.
	f.Close()

	// A pre-cancelled context must return context.Canceled immediately because
	// the rateLimiter channel will never produce another value.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := f.Fetch(cancelCtx, srv.URL)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Fetch() after Close() returned nil error; want context error")
	}
	if err != context.Canceled {
		t.Errorf("Fetch() after Close() error = %v; want context.Canceled", err)
	}
	// Should resolve essentially immediately (well under 200 ms).
	const maxElapsed = 200 * time.Millisecond
	if elapsed > maxElapsed {
		t.Errorf("Fetch() after Close() took %v; want < %v", elapsed, maxElapsed)
	}
}

// ---------------------------------------------------------------------------
// Test 9 — Concurrent Fetch from multiple goroutines under HTTP fetcher
// with -race: tests cacheMu (and ticker channel) correctness under load.
// ---------------------------------------------------------------------------

func TestConcurrentFetch_HTTPFetcher_NoDataRace(t *testing.T) {
	var requestCount atomic.Int32

	srv := newHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		fmt.Fprint(w, "response")
	})

	// rps=50 so concurrent goroutines do not wait too long in tests.
	const goroutines = 10
	f := NewHTTPFetcher(50, 0, "test")
	defer f.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, errs[i] = f.Fetch(ctx, srv.URL)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d Fetch() error = %v", i, err)
		}
	}
	if got := requestCount.Load(); got != int32(goroutines) {
		t.Errorf("server received %d requests; want %d", got, goroutines)
	}
}

// ---------------------------------------------------------------------------
// Test 10 — Concurrent Fetch under local fetcher:
// tests fileCache mutex correctness with -race.
// Multiple goroutines fetch from the same file simultaneously; no goroutine
// should observe a partial read and there must be no data race.
// ---------------------------------------------------------------------------

func TestConcurrentFetch_LocalFetcher_NoDataRace(t *testing.T) {
	const content = "<?php class WP_Query {}"
	f, dir := newLocalFetcherWithFiles(t, map[string]string{
		"wp-query.php": content,
	})
	absPath := filepath.Join(dir, "wp-query.php")

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	bodies := make([]string, goroutines)

	ctx := context.Background()

	// Use two different fragment suffixes to exercise fragment-stripping under
	// concurrency as well.
	fragments := []string{"", "#WP_Query", "#WP_Query::get_posts"}

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			url := absPath + fragments[i%len(fragments)]
			var data []byte
			data, errs[i] = f.Fetch(ctx, url)
			bodies[i] = string(data)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d Fetch() error = %v", i, err)
		}
		if bodies[i] != content {
			t.Errorf("goroutine %d body = %q; want %q", i, bodies[i], content)
		}
	}
}

// ---------------------------------------------------------------------------
// Bonus: No-retry on 404 — characterizes the immediate-error branch
// ---------------------------------------------------------------------------

func TestFetch_NoRetryOn404(t *testing.T) {
	var hitCount atomic.Int32

	srv := newHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		http.NotFound(w, r)
	})

	// maxRetry=3 but 404 must not be retried.
	f := NewHTTPFetcher(100, 3, "")
	defer f.Close()

	_, err := f.Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("Fetch() error = nil; want 404 error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q; want it to mention 404", err.Error())
	}
	if got := hitCount.Load(); got != 1 {
		t.Errorf("server hit count = %d; want 1 (no retries on 404)", got)
	}
}

// ---------------------------------------------------------------------------
// Bonus: Retry on 429 — characterizes TooManyRequests retry path.
// Server returns 429 twice then 200; fetcher with maxRetry=2 should succeed.
// ---------------------------------------------------------------------------

func TestFetch_RetryOn429_ThenSucceeds(t *testing.T) {
	var hitCount atomic.Int32

	srv := newHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := hitCount.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, "final success")
	})

	// rps=100; maxRetry=2 so 3 total attempts.
	f := NewHTTPFetcher(100, 2, "")
	defer f.Close()

	got, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch() error = %v; want nil (succeeded on third attempt)", err)
	}
	if string(got) != "final success" {
		t.Errorf("Fetch() body = %q; want %q", string(got), "final success")
	}
	if n := hitCount.Load(); n != 3 {
		t.Errorf("server hit count = %d; want 3", n)
	}
}
