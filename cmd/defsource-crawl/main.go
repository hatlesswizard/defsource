//go:build sqlite_fts5 || fts5

package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hatlesswizard/defsource/internal/crawler"
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/wordpress"
	"github.com/hatlesswizard/defsource/internal/store/sqlite"
)

func main() {
	sourceName := flag.String("source", "wordpress", "Documentation source to crawl")
	dbPath := flag.String("db", "./data/defsource.db", "Path to SQLite database")
	workers := flag.Int("workers", 10, "Concurrent workers")
	rps := flag.Int("rps", 10, "Requests per second")
	resume := flag.Bool("resume", false, "Resume last interrupted crawl")
	retryFailed := flag.Bool("retry-failed", false, "Retry transient failures from last crawl")
	flag.Parse()

	// Ensure data directory exists
	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Open store
	st, err := sqlite.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer st.Close()

	// Select source adapter
	var src source.Source
	switch *sourceName {
	case "wordpress":
		src = wordpress.New()
	default:
		log.Fatalf("Unknown source: %s", *sourceName)
	}

	// Create fetcher and crawler
	fetcher := crawler.NewFetcher(*rps, 3, "defSource/1.0 (open-source documentation indexer)")
	defer fetcher.Close()
	c := crawler.New(fetcher, st, src, *workers)

	// Run with graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := crawler.RunOptions{
		Resume:      *resume,
		RetryFailed: *retryFailed,
	}

	if err := c.Run(ctx, opts); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Printf("Crawl interrupted by user.")
		} else {
			log.Fatalf("Crawl failed: %v", err)
		}
	}
}
