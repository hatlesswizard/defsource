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

const maxWrapperDepth = 3

type Crawler struct {
	fetcher   *Fetcher
	store     store.Store
	source    source.Source
	workers   int
	sessionID int64
	cacheMu   sync.RWMutex
	success   atomic.Int64
	failed    atomic.Int64
	total     int
}

type RunOptions struct {
	Resume      bool
	RetryFailed bool
}

func New(fetcher *Fetcher, store store.Store, src source.Source, workers int) *Crawler {
	if workers <= 0 {
		workers = 10
	}
	return &Crawler{fetcher: fetcher, store: store, source: src, workers: workers}
}

func (c *Crawler) Run(ctx context.Context, opts RunOptions) error {
	start := time.Now()
	log.Printf("Starting crawl for %s with %d workers", c.source.ID(), c.workers)

	// Ensure cleanup on interruption
	defer func() {
		if ctx.Err() != nil {
			log.Println("Crawl interrupted, updating session and rebuilding index...")
			bgCtx := context.Background()
			c.store.UpdateCrawlSession(bgCtx, c.sessionID, "interrupted",
				int(c.success.Load()), int(c.failed.Load()), 0)
			if err := c.store.RebuildIndex(bgCtx, c.source.ID()); err != nil {
				log.Printf("ERROR: failed to rebuild search index on interrupt: %v", err)
			}
			c.printSummary()
		}
	}()

	// Step 1: Register/update the library
	meta := c.source.Meta()
	if err := c.store.UpsertLibrary(ctx, c.source.ID(), meta); err != nil {
		return fmt.Errorf("upsert library: %w", err)
	}

	// Step 2: Discover all entity URLs
	log.Println("Discovering entities...")
	entityURLs, err := c.source.DiscoverEntities(ctx, c.fetcher.Fetch)
	if err != nil {
		return fmt.Errorf("discover entities: %w", err)
	}
	log.Printf("Found %d entities to crawl", len(entityURLs))

	// Step 3: Handle resume/retry
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
					// Only retry transient errors
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
			return fmt.Errorf("create crawl session: %w", err)
		}
		c.sessionID = id
	}

	c.total = len(entityURLs)

	// Step 4: Progress display goroutine
	progressCtx, progressCancel := context.WithCancel(ctx)
	defer progressCancel()
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

	// Step 5: Process entities concurrently with worker pool
	wrapperCache := make(map[string]string)
	var totalSnippets int
	var snippetMu sync.Mutex

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
				c.processEntity(ctx, workerID, entityURL,
					wrapperCache, &totalSnippets, &snippetMu)
			}
		}(i)
	}
	wg.Wait()

	progressCancel()

	// Step 6: Update snippet count
	snippetMu.Lock()
	snippets := totalSnippets
	snippetMu.Unlock()
	if err := c.store.UpdateSnippetCount(ctx, libraryID, snippets); err != nil {
		log.Printf("WARNING: failed to update snippet count: %v", err)
	}

	// Step 7: Rebuild FTS5 search index
	log.Println("Rebuilding search index...")
	if err := c.store.RebuildIndex(ctx, libraryID); err != nil {
		log.Printf("WARNING: final index rebuild failed: %v", err)
	}

	// Step 8: Finalize session
	c.store.UpdateCrawlSession(ctx, c.sessionID, "completed",
		int(c.success.Load()), int(c.failed.Load()), 0)

	c.printSummary()

	log.Printf("Crawl complete. %d entities, %d total snippets indexed.",
		c.success.Load(), snippets)
	return nil
}

func (c *Crawler) processEntity(ctx context.Context, workerID int,
	entityURL string,
	wrapperCache map[string]string,
	totalSnippets *int, snippetMu *sync.Mutex) {

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

	// Record entity success
	c.store.RecordProgress(ctx, c.sessionID, &store.CrawlProgressItem{
		URL: entityURL, ItemType: "entity", Status: "success",
	})

	snippetMu.Lock()
	*totalSnippets++
	entityCount := *totalSnippets
	snippetMu.Unlock()

	if entityCount > 0 && entityCount%50 == 0 {
		log.Printf("[worker %d] Rebuilding search index (checkpoint at %d snippets)...", workerID, entityCount)
		if err := c.store.RebuildIndex(ctx, c.source.ID()); err != nil {
			log.Printf("[worker %d] WARNING: checkpoint index rebuild failed: %v", workerID, err)
		}
	}

	// Process methods for this entity
	for _, methodURL := range methodURLs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		body, err := c.fetcher.Fetch(ctx, methodURL)
		if err != nil {
			log.Printf("[worker %d]   WARNING: failed to fetch %s: %v (skipping)", workerID, methodURL, err)
			c.recordFailure(ctx, methodURL, "method", err)
			continue
		}

		method, err := c.source.ParseMethod(ctx, methodURL, body)
		if err != nil {
			log.Printf("[worker %d]   WARNING: failed to parse %s: %v (skipping)", workerID, methodURL, err)
			c.recordFailure(ctx, methodURL, "method", err)
			continue
		}

		c.resolveWrapperChain(ctx, method, entity.Slug, wrapperCache, 0)

		if err := c.store.UpsertMethod(ctx, entityID, method); err != nil {
			log.Printf("[worker %d]   WARNING: failed to store method %s: %v", workerID, method.Name, err)
			c.recordFailure(ctx, methodURL, "method", err)
			continue
		}

		c.store.RecordProgress(ctx, c.sessionID, &store.CrawlProgressItem{
			URL: methodURL, ItemType: "method", Status: "success",
		})

		snippetMu.Lock()
		*totalSnippets++
		snippetMu.Unlock()
	}

	c.success.Add(1)

	// Update snippet count after each entity's methods
	snippetMu.Lock()
	snippets := *totalSnippets
	snippetMu.Unlock()
	if err := c.store.UpdateSnippetCount(ctx, c.source.ID(), snippets); err != nil {
		log.Printf("[worker %d] WARNING: failed to update snippet count: %v", workerID, err)
	}
}

func (c *Crawler) recordFailure(ctx context.Context, url, itemType string, err error) {
	c.failed.Add(1)
	errType := classifyError(err)
	c.store.RecordProgress(ctx, c.sessionID, &store.CrawlProgressItem{
		URL:          url,
		ItemType:     itemType,
		Status:       "failed",
		ErrorType:    errType,
		ErrorMessage: err.Error(),
	})
}

func classifyError(err error) string {
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
	case errors.Is(err, context.Canceled):
		return "interrupted"
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
	failures, _ := c.store.GetFailures(bgCtx, c.sessionID)

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

func (c *Crawler) resolveWrapperChain(ctx context.Context, method *source.Method, entitySlug string, cache map[string]string, depth int) {
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

	c.cacheMu.RLock()
	cachedSource, cached := cache[targetURL]
	c.cacheMu.RUnlock()
	if cached {
		method.WrappedMethod = targetName
		method.WrappedSource = cachedSource
		return
	}

	body, err := c.fetcher.Fetch(ctx, targetURL)
	if err != nil {
		log.Printf("    wrapper: failed to fetch %s: %v", targetURL, err)
		return
	}

	wrappedSource, err := c.source.ParseSourceCode(body)
	if err != nil {
		log.Printf("    wrapper: failed to parse source from %s: %v", targetURL, err)
		return
	}

	c.cacheMu.Lock()
	cache[targetURL] = wrappedSource
	c.cacheMu.Unlock()
	method.WrappedMethod = targetName
	method.WrappedSource = wrappedSource

	// Recursively check if the wrapped function is itself a wrapper
	tempMethod := &source.Method{SourceCode: wrappedSource}
	c.resolveWrapperChain(ctx, tempMethod, entitySlug, cache, depth+1)
	if tempMethod.WrappedSource != "" {
		method.WrappedSource += "\n\n// --- Delegates to: " + tempMethod.WrappedMethod + " ---\n\n" + tempMethod.WrappedSource
		method.WrappedMethod += " → " + tempMethod.WrappedMethod
	}
}
