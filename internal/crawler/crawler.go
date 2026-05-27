// Package crawler implements a concurrent documentation crawler that fetches,
// parses, and indexes entities from a source adapter into a store. The crawler
// uses a fixed worker pool with a shared rate-limited fetcher, a SQLite-backed
// store, and a source-specific adapter to discover and parse entities.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/store"
)

// Sentinel errors for fetch failure classification.
// Fetcher implementations should wrap their errors with these sentinels using
// fmt.Errorf("...: %w", ErrFetch404) so that classifyError can use errors.Is
// instead of string matching.
var (
	// ErrFetch404 signals that the target URL returned HTTP 404 Not Found.
	ErrFetch404 = errors.New("http 404 not found")

	// ErrFetch5xx signals that the target URL returned an HTTP 5xx server error.
	ErrFetch5xx = errors.New("http 5xx server error")

	// ErrFetchNetwork signals a transient network-layer failure (connection
	// refused, DNS error, timeout, etc.).
	ErrFetchNetwork = errors.New("network fetch failed")
)

// maxWrapperDepth limits how many levels of PHP wrapper functions are followed
// when resolving a method's wrapped source. PHP wrapper chains can be multiple
// levels deep (e.g. wp_cache_get -> WP_Object_Cache::get -> underlying impl);
// 3 levels is sufficient to catch real wrappers while preventing infinite recursion.
const maxWrapperDepth = 3

// Crawler orchestrates concurrent fetching and indexing of documentation
// entities. Each call to Run discovers all entity URLs from the configured
// source adapter, then processes them in parallel using a worker pool,
// persisting results to the store and rebuilding the FTS5 search index.
type Crawler struct {
	fetcher   Fetcher
	store     store.Store
	source    source.Source
	workers   int
	sessionID int64

	// wrapperCache maps resolved wrapper URLs to their extracted source code
	// (string values). It is read via wrapperCache.Load which returns a string.
	// Writes happen only after a fetch+parse completes successfully.
	wrapperCache sync.Map

	// wrapperOnce deduplicates concurrent fetches for the same URL. It maps
	// URLs to *sync.Once values. The Once ensures exactly one goroutine performs
	// the fetch+parse; all concurrent goroutines for the same URL block inside
	// once.Do until the first completes. Both maps are reset at the start of
	// each Run call.
	wrapperOnce sync.Map

	// totalSnippets counts all stored snippets (entities + methods) for the
	// current Run. Using atomic.Int64 replaces the *int + *sync.Mutex pair that
	// was previously threaded through processEntity as parameters.
	totalSnippets atomic.Int64

	success atomic.Int64
	failed  atomic.Int64
	total   int
}

// RunOptions controls resume and retry behaviour for a crawl run.
type RunOptions struct {
	// Resume resumes the most recent interrupted session, skipping URLs that
	// were already successfully processed.
	Resume bool

	// RetryFailed re-queues URLs that failed with transient errors in the most
	// recent session. Permanent errors (http_404, parse_error, fk_error) are
	// not retried.
	RetryFailed bool
}

// New constructs a Crawler backed by the given fetcher, store, and source
// adapter. workers controls the number of concurrent fetch goroutines; if
// workers is <= 0 it defaults to 10.
func New(fetcher Fetcher, store store.Store, src source.Source, workers int) *Crawler {
	if workers <= 0 {
		workers = 10
	}
	return &Crawler{fetcher: fetcher, store: store, source: src, workers: workers}
}

// Run executes the full crawl pipeline: register the library, discover all
// entity URLs, optionally resume or retry a previous session, process entities
// concurrently with a worker pool, update snippet counts, and rebuild the FTS5
// search index. The context controls graceful cancellation; on cancellation the
// interrupted session state is persisted and the search index is rebuilt from
// whatever was indexed so far.
func (c *Crawler) Run(ctx context.Context, opts RunOptions) error {
	start := time.Now()
	log.Printf("Starting crawl for %s with %d workers", c.source.ID(), c.workers)

	// Reset per-run counters so that repeated Run calls on the same Crawler
	// instance start from a clean state.
	c.totalSnippets.Store(0)
	c.success.Store(0)
	c.failed.Store(0)
	c.wrapperCache = sync.Map{}
	c.wrapperOnce = sync.Map{}

	// On context cancellation, persist the interrupted session state and rebuild
	// the search index so partial results remain queryable.
	defer func() {
		if ctx.Err() != nil {
			log.Println("Crawl interrupted, updating session and rebuilding index...")
			bgCtx := context.Background()
			if err := c.store.UpdateCrawlSession(bgCtx, c.sessionID, "interrupted",
				int(c.success.Load()), int(c.failed.Load()), 0); err != nil {
				log.Printf("ERROR: could not update session %d on interrupt: %v", c.sessionID, err)
			}
			if err := c.store.RebuildIndex(bgCtx, c.source.ID()); err != nil {
				log.Printf("ERROR: failed to rebuild search index on interrupt: %v", err)
			}
			c.printSummary()
		}
	}()

	if err := c.registerLibrary(ctx); err != nil {
		return err
	}

	entityURLs, err := c.discoverEntities(ctx)
	if err != nil {
		return err
	}

	entityURLs, err = c.applySessionFilter(ctx, entityURLs, opts)
	if err != nil {
		return err
	}

	c.total = len(entityURLs)

	stopProgress := c.startProgressReporter(ctx, start)
	defer stopProgress()

	c.dispatchWorkers(ctx, entityURLs)
	stopProgress()

	if err := c.finalizeSnippetCount(ctx); err != nil {
		log.Printf("WARNING: failed to update snippet count: %v", err)
	}

	if err := c.rebuildSearchIndex(ctx); err != nil {
		log.Printf("WARNING: final index rebuild failed: %v", err)
	}

	c.finalizeSession(ctx)
	c.printSummary()

	log.Printf("Crawl complete. %d entities, %d total snippets indexed.",
		c.success.Load(), c.totalSnippets.Load())
	return nil
}

// registerLibrary upserts the library metadata into the store.
func (c *Crawler) registerLibrary(ctx context.Context) error {
	meta := c.source.Meta()
	if err := c.store.UpsertLibrary(ctx, c.source.ID(), meta); err != nil {
		return fmt.Errorf("upsert library: %w", err)
	}
	return nil
}

// discoverEntities retrieves all entity URLs from the source adapter.
func (c *Crawler) discoverEntities(ctx context.Context) ([]string, error) {
	log.Println("Discovering entities...")
	entityURLs, err := c.source.DiscoverEntities(ctx, c.fetcher.Fetch)
	if err != nil {
		return nil, fmt.Errorf("discover entities: %w", err)
	}
	log.Printf("Found %d entities to crawl", len(entityURLs))
	return entityURLs, nil
}

// applySessionFilter filters the URL list according to resume/retry options.
// It creates a new crawl session when no prior session is being resumed.
func (c *Crawler) applySessionFilter(ctx context.Context, entityURLs []string, opts RunOptions) ([]string, error) {
	libraryID := c.source.ID()

	if opts.Resume {
		session, err := c.store.GetLastSession(ctx, libraryID)
		if err == nil && session != nil && session.Status == "interrupted" {
			processed, err := c.store.GetProcessedURLs(ctx, session.ID)
			if err == nil {
				c.sessionID = session.ID
				var remaining []string
				for _, u := range entityURLs {
					if _, done := processed[u]; !done {
						remaining = append(remaining, u)
					}
				}
				log.Printf("Resuming session %d: %d remaining of %d total",
					session.ID, len(remaining), len(entityURLs))
				entityURLs = remaining
			}
		}
	} else if opts.RetryFailed {
		session, err := c.store.GetLastSession(ctx, libraryID)
		if err == nil && session != nil {
			failures, err := c.store.GetFailures(ctx, session.ID)
			if err == nil {
				c.sessionID = session.ID
				entityURLs = nil
				for _, f := range failures {
					// Only retry transient errors.
					switch f.ErrorType {
					case "http_404", "parse_error", "fk_error":
						continue
					default:
						entityURLs = append(entityURLs, f.URL)
					}
				}
				log.Printf("Retrying %d transient failures from session %d",
					len(entityURLs), session.ID)
			}
		}
	}

	if c.sessionID == 0 {
		id, err := c.store.CreateCrawlSession(ctx, libraryID, len(entityURLs))
		if err != nil {
			return nil, fmt.Errorf("create crawl session: %w", err)
		}
		c.sessionID = id
	}

	return entityURLs, nil
}

// startProgressReporter launches a background goroutine that logs crawl
// progress every 30 seconds. Call the returned stop function to shut it down.
func (c *Crawler) startProgressReporter(ctx context.Context, start time.Time) func() {
	progressCtx, progressCancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s, f := c.success.Load(), c.failed.Load()
				elapsed := time.Since(start).Round(time.Second)
				pct := float64(0)
				if c.total > 0 {
					pct = float64(s+f) / float64(c.total) * 100
				}
				log.Printf("[Progress] %d/%d entities (%.1f%%) | %d failed | %s elapsed",
					s+f, c.total, pct, f, elapsed)
			case <-progressCtx.Done():
				return
			}
		}
	}()
	return progressCancel
}

// dispatchWorkers fans entityURLs out to c.workers goroutines and waits for
// all of them to finish before returning.
func (c *Crawler) dispatchWorkers(ctx context.Context, entityURLs []string) {
	entityCh := make(chan string, len(entityURLs))
	for _, url := range entityURLs {
		entityCh <- url
	}
	close(entityCh)

	var wg sync.WaitGroup
	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for entityURL := range entityCh {
				select {
				case <-ctx.Done():
					return
				default:
				}
				c.processEntity(ctx, workerID, entityURL)
			}
		}(i)
	}
	wg.Wait()
}

// finalizeSnippetCount writes the total accumulated snippet count to the store.
// This is the single, authoritative UpdateSnippetCount call for a Run; per-entity
// calls have been removed (Wave-3 refactor).
func (c *Crawler) finalizeSnippetCount(ctx context.Context) error {
	return c.store.UpdateSnippetCount(ctx, c.source.ID(), int(c.totalSnippets.Load()))
}

// rebuildSearchIndex requests a full FTS5 index rebuild for the library.
func (c *Crawler) rebuildSearchIndex(ctx context.Context) error {
	log.Println("Rebuilding search index...")
	return c.store.RebuildIndex(ctx, c.source.ID())
}

// finalizeSession marks the crawl session as "completed" in the store.
func (c *Crawler) finalizeSession(ctx context.Context) {
	if err := c.store.UpdateCrawlSession(ctx, c.sessionID, "completed",
		int(c.success.Load()), int(c.failed.Load()), 0); err != nil {
		log.Printf("ERROR: could not finalize session %d as completed: %v", c.sessionID, err)
	}
}

// processEntity fetches, parses, and stores a single entity and all of its
// associated methods. On any fatal error the entity is recorded as a failure
// and processing stops for that entity. Snippet counts are tracked via the
// shared c.totalSnippets atomic counter.
func (c *Crawler) processEntity(ctx context.Context, workerID int, entityURL string) {
	log.Printf("[worker %d] Fetching entity: %s", workerID, entityURL)

	body, err := c.fetcher.Fetch(ctx, entityURL)
	if err != nil {
		log.Printf("[worker %d] WARNING: failed to fetch %s: %v (skipping)", workerID, entityURL, err)
		c.recordFailure(ctx, entityURL, "entity", err)
		return
	}

	entity, methodURLs, err := c.source.ParseEntity(ctx, entityURL, body)
	if err != nil {
		log.Printf("[worker %d] WARNING: failed to parse entity %s: %v (skipping)", workerID, entityURL, err)
		c.recordFailure(ctx, entityURL, "entity", err)
		return
	}

	entityID, err := c.store.UpsertEntity(ctx, c.source.ID(), entity)
	if err != nil {
		log.Printf("[worker %d] WARNING: failed to store entity %s: %v (skipping)", workerID, entity.Name, err)
		c.recordFailure(ctx, entityURL, "entity", err)
		return
	}

	if err := c.store.RecordProgress(ctx, c.sessionID, &store.CrawlProgressItem{
		URL: entityURL, ItemType: "entity", Status: "success",
	}); err != nil {
		log.Printf("[worker %d] WARNING: failed to record entity progress for %s: %v", workerID, entityURL, err)
	}

	entityCount := c.totalSnippets.Add(1)

	if entityCount > 0 && entityCount%50 == 0 {
		log.Printf("[worker %d] Rebuilding search index (checkpoint at %d snippets)...", workerID, entityCount)
		if err := c.store.RebuildIndex(ctx, c.source.ID()); err != nil {
			log.Printf("[worker %d] WARNING: checkpoint index rebuild failed: %v", workerID, err)
		}
	}

	// For function entities, create a method record from the same body (no extra fetch).
	if entity.Kind == "function" && len(methodURLs) == 0 {
		method, err := c.source.ParseMethod(ctx, entityURL, body)
		if err == nil {
			if storeErr := c.store.UpsertMethod(ctx, entityID, method); storeErr != nil {
				log.Printf("[worker %d] WARNING: failed to store function method %s: %v", workerID, method.Name, storeErr)
				c.recordFailure(ctx, entityURL, "method", storeErr)
			} else {
				c.totalSnippets.Add(1)
			}
		}
	}

	c.processEntityMethods(ctx, workerID, entityID, entity, methodURLs)

	c.success.Add(1)
}

// processEntityMethods fetches, parses, and stores each method URL for the
// given entity. Wrapper chains are resolved before storing each method.
func (c *Crawler) processEntityMethods(
	ctx context.Context,
	workerID int,
	entityID int64,
	entity *source.Entity,
	methodURLs []string,
) {
	for _, methodURL := range methodURLs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		methodBody, err := c.fetcher.Fetch(ctx, methodURL)
		if err != nil {
			log.Printf("[worker %d]   WARNING: failed to fetch %s: %v (skipping)", workerID, methodURL, err)
			c.recordFailure(ctx, methodURL, "method", err)
			continue
		}

		method, err := c.source.ParseMethod(ctx, methodURL, methodBody)
		if err != nil {
			log.Printf("[worker %d]   WARNING: failed to parse %s: %v (skipping)", workerID, methodURL, err)
			c.recordFailure(ctx, methodURL, "method", err)
			continue
		}

		c.resolveWrapperChain(ctx, method, entity.Slug, 0)

		if err := c.store.UpsertMethod(ctx, entityID, method); err != nil {
			log.Printf("[worker %d]   WARNING: failed to store method %s: %v", workerID, method.Name, err)
			c.recordFailure(ctx, methodURL, "method", err)
			continue
		}

		if err := c.store.RecordProgress(ctx, c.sessionID, &store.CrawlProgressItem{
			URL: methodURL, ItemType: "method", Status: "success",
		}); err != nil {
			log.Printf("[worker %d]   WARNING: failed to record method progress for %s: %v", workerID, methodURL, err)
		}

		c.totalSnippets.Add(1)
	}
}

func (c *Crawler) recordFailure(ctx context.Context, url, itemType string, err error) {
	c.failed.Add(1)
	errType := classifyError(err)
	if progressErr := c.store.RecordProgress(ctx, c.sessionID, &store.CrawlProgressItem{
		URL:          url,
		ItemType:     itemType,
		Status:       "failed",
		ErrorType:    errType,
		ErrorMessage: err.Error(),
	}); progressErr != nil {
		log.Printf("WARNING: failed to record failure progress for %s: %v", url, progressErr)
	}
}

// classifyError maps an error to one of the canonical error-type strings used
// by the crawl progress table and the retry logic.
//
// The check order is:
//  1. context.Canceled — checked first via errors.Is so a cancelled context is
//     always classified as "interrupted" regardless of any wrapping.
//  2. Sentinel errors (ErrFetch404, ErrFetch5xx, ErrFetchNetwork) — checked via
//     errors.Is so that callers who wrap with %w get correct classification.
//  3. String heuristics — final fallback for errors not yet wrapped with sentinels.
func classifyError(err error) string {
	// context.Canceled must be checked first — a cancelled context must always
	// win even if the error message happens to contain "timeout" or "404".
	if errors.Is(err, context.Canceled) {
		return "interrupted"
	}

	// Sentinel errors — fetcher.go can wrap with these for typed classification.
	// TODO(Wave-3-C): update fetcher.go to wrap errors with ErrFetch404,
	// ErrFetch5xx, and ErrFetchNetwork using fmt.Errorf("...: %w", sentinel)
	// so that the string-match fallbacks below can eventually be removed.
	if errors.Is(err, ErrFetch404) {
		return "http_404"
	}
	if errors.Is(err, ErrFetch5xx) {
		return "http_5xx"
	}
	if errors.Is(err, ErrFetchNetwork) {
		return "timeout"
	}

	// String-match fallback for error messages not yet using sentinels.
	msg := err.Error()
	switch {
	case strings.Contains(msg, "404"):
		return "http_404"
	case strings.Contains(msg, "429"):
		return "http_429"
	case strings.Contains(msg, "server error"):
		return "http_5xx"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return "timeout"
	case strings.Contains(msg, "FOREIGN KEY"):
		return "fk_error"
	default:
		return "other"
	}
}

func (c *Crawler) printSummary() {
	bgCtx := context.Background()
	stats, err := c.store.GetCrawlStats(bgCtx, c.sessionID)
	if err != nil {
		log.Printf("WARNING: could not get crawl stats: %v", err)
		return
	}
	failures, err := c.store.GetFailures(bgCtx, c.sessionID)
	if err != nil {
		log.Printf("WARNING: could not get crawl failures for session %d: %v", c.sessionID, err)
	}

	log.Println("\n=== CRAWL SUMMARY ===")
	log.Printf("Session: #%d", c.sessionID)
	log.Printf("Total: %d | Success: %d | Failed: %d",
		stats.Total, stats.Success, stats.Failed)

	if len(stats.FailuresByType) > 0 {
		log.Println("\nFailures by type:")
		for t, cnt := range stats.FailuresByType {
			log.Printf("  %s: %d", t, cnt)
		}
	}
	if len(failures) > 0 {
		log.Println("\nFailed URLs:")
		for _, f := range failures {
			log.Printf("  [%s] %s: %s", f.ErrorType, f.URL, f.ErrorMessage)
		}
	}
	log.Println("\nTo resume: defsource-crawl --resume")
	log.Println("To retry transient failures: defsource-crawl --retry-failed")
}

// resolveWrapperChain follows a chain of wrapper functions starting from method,
// fetching and storing the wrapped source code at each level up to maxWrapperDepth.
//
// Two sync.Maps collaborate to guarantee exactly ONE fetch+parse per URL:
//   - c.wrapperOnce maps URLs to *sync.Once. All goroutines for the same URL
//     call once.Do, which blocks them all until the first goroutine completes
//     the fetch+parse.
//   - c.wrapperCache maps URLs to the resolved source string. It is written
//     only after a successful parse, and can be read directly as a string.
//
// c.wrapperCache.Load(targetURL) is guaranteed to return a string value,
// satisfying tests that assert the cache type.
func (c *Crawler) resolveWrapperChain(ctx context.Context, method *source.Method, entitySlug string, depth int) {
	if depth >= maxWrapperDepth {
		return
	}

	isWrapper, targetName, targetKind := c.source.DetectWrapper(method)
	if !isWrapper {
		return
	}

	targetURL := c.source.ResolveWrapperURL(targetName, targetKind, entitySlug)
	if targetURL == "" {
		return
	}

	// Fast path: already in the string cache.
	if cached, ok := c.wrapperCache.Load(targetURL); ok {
		method.WrappedMethod = targetName
		method.WrappedSource = cached.(string)
		return
	}

	// Slow path: get-or-create the Once for this URL and execute exactly once.
	onceVal, _ := c.wrapperOnce.LoadOrStore(targetURL, &sync.Once{})
	once := onceVal.(*sync.Once)
	once.Do(func() {
		body, err := c.fetcher.Fetch(ctx, targetURL)
		if err != nil {
			log.Printf("    wrapper: failed to fetch %s: %v", targetURL, err)
			return
		}
		src, err := c.source.ParseSourceCode(targetURL, body)
		if err != nil {
			log.Printf("    wrapper: failed to parse source from %s: %v", targetURL, err)
			return
		}
		// Store the string result so subsequent Load calls return a string.
		c.wrapperCache.Store(targetURL, src)
	})

	// Read the result (may be absent if fetch/parse failed).
	cached, ok := c.wrapperCache.Load(targetURL)
	if !ok {
		return
	}
	wrappedSource := cached.(string)

	method.WrappedMethod = targetName
	method.WrappedSource = wrappedSource

	// Recursively check if the wrapped function is itself a wrapper.
	tempMethod := &source.Method{SourceCode: wrappedSource}
	c.resolveWrapperChain(ctx, tempMethod, entitySlug, depth+1)
	if tempMethod.WrappedSource != "" {
		method.WrappedSource += "\n\n// --- Delegates to: " + tempMethod.WrappedMethod + " ---\n\n" + tempMethod.WrappedSource
		method.WrappedMethod += " → " + tempMethod.WrappedMethod
	}
}
