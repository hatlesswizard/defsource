// Package crawler implements the concurrent crawl pipeline: a rate-limited HTTP
// fetcher and a worker pool that discovers, fetches, parses, and stores
// WordPress documentation entities.
package crawler

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Fetcher is the interface satisfied by both HTTPFetcher and LocalFetcher.
// Fetch retrieves the content at url, respecting ctx cancellation.
// Close releases any resources held by the fetcher (e.g. the rate-limit ticker
// in HTTPFetcher). Close is a no-op on LocalFetcher and always returns nil.
type Fetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
	Close() error
}

// ---------------------------------------------------------------------------
// HTTPFetcher — rate-limited, retrying HTTP fetcher
// ---------------------------------------------------------------------------

// HTTPFetcher fetches URLs over HTTP with ticker-based rate limiting and
// exponential-backoff retry. It is the production fetch implementation used by
// the crawl worker pool.
//
// Rate limiting is implemented with a time.Ticker whose channel (rateLimiter)
// is consumed once per Fetch call. This is goroutine-safe without any mutex:
// the channel itself serialises access to the rate budget.
//
// Call Close when the HTTPFetcher is no longer needed to stop the ticker goroutine.
type HTTPFetcher struct {
	client    *http.Client
	userAgent string
	maxRetry  int

	// rateLimiter is the receive end of ticker.C. Workers block on this channel
	// to pace outbound requests. One tick is consumed per Fetch call, making rate
	// limiting goroutine-safe without any additional mutex.
	rateLimiter <-chan time.Time

	// ticker drives rateLimiter. It must be stopped via Close to prevent the
	// internal goroutine from leaking after the HTTPFetcher is discarded.
	ticker *time.Ticker
}

// NewHTTPFetcher creates an HTTPFetcher that issues at most requestsPerSecond
// requests per second. Each request is retried up to maxRetry times on
// transient failures (429, 5xx, network error) with exponential backoff.
// userAgent is sent in the User-Agent request header.
//
// Call Close when the HTTPFetcher is no longer needed to stop the ticker goroutine.
func NewHTTPFetcher(requestsPerSecond int, maxRetry int, userAgent string) *HTTPFetcher {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 5
	}
	ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
	return &HTTPFetcher{
		client:      &http.Client{Timeout: 30 * time.Second},
		userAgent:   userAgent,
		maxRetry:    maxRetry,
		rateLimiter: ticker.C,
		ticker:      ticker,
	}
}

// Close stops the rate-limiting ticker, releasing its goroutine. Safe to call
// multiple times. Implements Fetcher.
func (f *HTTPFetcher) Close() error {
	if f.ticker != nil {
		f.ticker.Stop()
	}
	return nil
}

// Fetch waits for a rate-limiter tick, then issues a GET request with
// exponential-backoff retry on transient errors (429, 5xx, network failures).
// 404 responses are returned immediately without retrying.
//
// Fetch respects ctx cancellation at the rate-limiter wait and at each backoff
// sleep. Implements Fetcher.
func (f *HTTPFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	select {
	case <-f.rateLimiter:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	var lastErr error
	for attempt := 0; attempt <= f.maxRetry; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			}
			timer.Stop()
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("User-Agent", f.userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read body: %w", err)
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			return body, nil
		case resp.StatusCode == http.StatusTooManyRequests:
			lastErr = fmt.Errorf("rate limited (429) on %s", url)
			continue
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("server error (%d) on %s", resp.StatusCode, url)
			continue
		case resp.StatusCode == http.StatusNotFound:
			return nil, fmt.Errorf("page not found (404): %s", url)
		default:
			return nil, fmt.Errorf("unexpected status %d on %s", resp.StatusCode, url)
		}
	}

	return nil, fmt.Errorf("all %d retries failed for %s: %w", f.maxRetry+1, url, lastErr)
}

// ---------------------------------------------------------------------------
// LocalFetcher — filesystem fetcher for tests and local-clone sources
// ---------------------------------------------------------------------------

// LocalFetcher reads bytes from the local filesystem instead of the network.
// Fragment identifiers in URLs (the "#…" suffix) are stripped before the path
// is read. File contents are cached in memory so that multiple calls with
// different fragments of the same file incur only one os.ReadFile call.
//
// LocalFetcher has no rate limiting; workers read at maximum speed and the
// sqlite.mu is the sole bottleneck during local crawls.
//
// Close is a no-op; it always returns nil.
type LocalFetcher struct {
	rootDir string

	// fileCache is a path→bytes in-memory cache. It avoids re-reading the same
	// file when multiple URL fragments point into one file. All accesses must be
	// gated by cacheMu.
	fileCache map[string][]byte

	// cacheMu guards fileCache. Reads acquire RLock; writes acquire Lock.
	// The TOCTOU window between a failed RLock read and the subsequent Lock write
	// is benign: file content is deterministic, so two goroutines that both miss
	// the cache will read and store identical bytes.
	cacheMu sync.RWMutex
}

// NewLocalFetcher creates a LocalFetcher that reads from the local filesystem.
// rootDir is informational only; paths in Fetch calls are used as-is.
//
// Intended for unit tests and local PHP-repository sources.
func NewLocalFetcher() *LocalFetcher {
	return &LocalFetcher{
		fileCache: make(map[string][]byte),
	}
}

// Close is a no-op on LocalFetcher. Implements Fetcher.
func (f *LocalFetcher) Close() error {
	return nil
}

// Fetch reads from the filesystem at the given path, stripping any URL fragment
// (the "#…" suffix) before the read. Repeated calls for the same underlying
// path (regardless of fragment) return cached bytes from the first read.
// Implements Fetcher.
func (f *LocalFetcher) Fetch(_ context.Context, path string) ([]byte, error) {
	filePath := path
	if idx := strings.Index(path, "#"); idx != -1 {
		filePath = path[:idx]
	}

	f.cacheMu.RLock()
	if data, ok := f.fileCache[filePath]; ok {
		f.cacheMu.RUnlock()
		return data, nil
	}
	f.cacheMu.RUnlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read local file: %w", err)
	}

	f.cacheMu.Lock()
	f.fileCache[filePath] = data
	f.cacheMu.Unlock()

	return data, nil
}
