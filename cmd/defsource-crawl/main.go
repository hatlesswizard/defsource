//go:build sqlite_fts5 || fts5

package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hatlesswizard/defsource/internal/config"
	"github.com/hatlesswizard/defsource/internal/crawler"
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/wpgithub"
	"github.com/hatlesswizard/defsource/internal/store/sqlite"
)

func main() {
	// Load env-var defaults first; CLI flags override them when explicitly passed.
	// Precedence: explicit CLI flag > env var > built-in default (see config.Load).
	cfg := config.Load()

	sourceName := flag.String("source", "wpgithub", "Documentation source to crawl")
	dbPath := flag.String("db", cfg.DBPath, "Path to SQLite database")
	workers := flag.Int("workers", cfg.Workers, "Concurrent workers")
	resume := flag.Bool("resume", false, "Resume last interrupted crawl")
	retryFailed := flag.Bool("retry-failed", false, "Retry transient failures from last crawl")
	wpVersion := flag.String("version", cfg.WPVersion, "WordPress release to download (e.g. 6.5.3); empty resolves the latest release")
	cacheDir := flag.String("cache-dir", cfg.CacheDir, "Directory for cached WordPress source downloads")
	refresh := flag.Bool("refresh", false, "Force re-download even if the version is already cached")
	downloadMethod := flag.String("download-method", "auto", "WordPress acquisition method: auto (tarball, then git fallback) | tarball | git")
	flag.Parse()

	// Ensure parent directory of DB path exists
	if dir := filepath.Dir(*dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create database directory %s: %v", dir, err)
		}
	}

	// Open store
	st, err := sqlite.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer st.Close()

	// Set up graceful shutdown early so the download itself is cancelable.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Select source adapter and create matching fetcher
	var src source.Source
	var fetcher crawler.Fetcher
	switch *sourceName {
	case "wpgithub":
		// On resume/retry, pin to the version already in the DB (if any) so a
		// resumed crawl never drifts to a newer "latest" release.
		version := *wpVersion
		if version == "" && (*resume || *retryFailed) {
			libID := wpgithub.New("").ID()
			if rec, err := st.GetLibrary(ctx, libID); err == nil && rec != nil &&
				rec.Version != "" && rec.Version != "unknown" {
				version = rec.Version
				log.Printf("Resuming crawl pinned to stored WordPress version %s", version)
			}
		}

		repoPath, resolvedVersion, err := wpgithub.EnsureRepo(ctx, wpgithub.DownloadOptions{
			Version:    version,
			CacheDir:   *cacheDir,
			Refresh:    *refresh,
			Method:     *downloadMethod,
			Token:      cfg.GitHubToken,
			UserAgent:  cfg.UserAgent,
			MaxRetries: cfg.MaxRetries,
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("Download interrupted by user.")
				return
			}
			log.Fatalf("Failed to obtain WordPress source: %v", err)
		}

		src = wpgithub.New(repoPath, wpgithub.WithRef(resolvedVersion))
		fetcher = crawler.NewLocalFetcher()
	default:
		log.Fatalf("Unknown source: %s", *sourceName)
	}
	defer fetcher.Close() //nolint:errcheck
	c := crawler.New(fetcher, st, src, *workers)

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
