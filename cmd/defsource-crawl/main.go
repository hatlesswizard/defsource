//go:build sqlite_fts5 || fts5

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hatlesswizard/defsource/internal/config"
	"github.com/hatlesswizard/defsource/internal/crawler"
	"github.com/hatlesswizard/defsource/internal/download"
	"github.com/hatlesswizard/defsource/internal/source"
	_ "github.com/hatlesswizard/defsource/internal/source/allsources"
	"github.com/hatlesswizard/defsource/internal/source/registry"
	"github.com/hatlesswizard/defsource/internal/source/php"
	"github.com/hatlesswizard/defsource/internal/store/sqlite"
)

func main() {
	// Load env-var defaults first; CLI flags override them when explicitly passed.
	// Precedence: explicit CLI flag > env var > built-in default (see config.Load).
	cfg := config.Load()

	sourceName := flag.String("source", "", "Source to crawl (e.g., python/django, rust/tokio, wpgithub)")
	language := flag.String("language", "", "Crawl ALL frameworks for a language (e.g., python, go, rust)")
	all := flag.Bool("all", false, "Crawl all registered sources")
	yes := flag.Bool("yes", false, "Skip confirmation prompts (for use with --all)")
	listSources := flag.Bool("list-sources", false, "Print all registered sources grouped by language")
	dbPath := flag.String("db", cfg.DBPath, "Path to SQLite database")
	workers := flag.Int("workers", cfg.Workers, "Concurrent workers")
	resume := flag.Bool("resume", false, "Resume last interrupted crawl")
	retryFailed := flag.Bool("retry-failed", false, "Retry transient failures from last crawl")
	wpVersion := flag.String("version", cfg.WPVersion, "WordPress release to download (e.g. 6.5.3); empty resolves the latest release")
	cacheDir := flag.String("cache-dir", cfg.CacheDir, "Directory for cached WordPress source downloads")
	refresh := flag.Bool("refresh", false, "Force re-download even if the version is already cached")
	downloadMethod := flag.String("download-method", "auto", "WordPress acquisition method: auto (tarball, then git fallback) | tarball | git")
	flag.Parse()

	// --list-sources: print registered sources and exit.
	if *listSources {
		printRegisteredSources()
		return
	}

	// Determine which sources to crawl.
	var sourceIDs []string
	switch {
	case *all:
		if !*yes {
			fmt.Printf("This will crawl ALL %d registered sources. Continue? [y/N] ", len(registry.Default.List()))
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				fmt.Println("Aborted.")
				return
			}
		}
		sourceIDs = registry.Default.List()
	case *language != "":
		sourceIDs = registry.Default.ListByLanguage(*language)
		if len(sourceIDs) == 0 {
			log.Fatalf("No sources registered for language %q. Use --list-sources to see available sources.", *language)
		}
	case *sourceName != "":
		sourceIDs = []string{*sourceName}
	default:
		// Default to wpgithub for backward compatibility.
		sourceIDs = []string{"wpgithub"}
	}

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

	for _, srcID := range sourceIDs {
		if ctx.Err() != nil {
			break
		}
		if err := crawlSource(ctx, srcID, st, cfg, *workers, *resume, *retryFailed, *wpVersion, *cacheDir, *refresh, *downloadMethod); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("Crawl interrupted by user.")
				break
			}
			log.Printf("Crawl of %s failed: %v", srcID, err)
		}
	}
}

// crawlSource runs a crawl for a single source ID.
func crawlSource(ctx context.Context, srcID string, st *sqlite.SQLiteStore, cfg config.Config,
	workers int, resume, retryFailed bool, wpVersion, cacheDir string, refresh bool, downloadMethod string) error {

	var src source.Source
	var fetcher crawler.Fetcher

	// Check if this is the WordPress source (needs special download handling).
	canonicalID := srcID
	if canonical, ok := resolveAlias(srcID); ok {
		canonicalID = canonical
	}

	if canonicalID == "php/wordpress" || srcID == "wpgithub" {
		// WordPress-specific: download source first.
		version := wpVersion
		if version == "" && (resume || retryFailed) {
			libID := php.New("").ID()
			if rec, err := st.GetLibrary(ctx, libID); err == nil && rec != nil &&
				rec.Version != "" && rec.Version != "unknown" {
				version = rec.Version
				log.Printf("Resuming crawl pinned to stored WordPress version %s", version)
			}
		}

		repoPath, resolvedVersion, err := php.EnsureRepo(ctx, php.DownloadOptions{
			Version:    version,
			CacheDir:   cacheDir,
			Refresh:    refresh,
			Method:     downloadMethod,
			Token:      cfg.GitHubToken,
			UserAgent:  cfg.UserAgent,
			MaxRetries: cfg.MaxRetries,
		})
		if err != nil {
			return fmt.Errorf("obtain WordPress source: %w", err)
		}

		src = php.New(repoPath, php.WithRef(resolvedVersion))
		fetcher = crawler.NewLocalFetcher()
	} else {
		// Generic registry-based source: create with empty path to read Meta,
		// download the repo, then re-create with the real path.
		factoryOpts := registry.FactoryOptions{}
		probe, err := registry.Default.Create(canonicalID, "", factoryOpts)
		if err != nil {
			return fmt.Errorf("create source %q: %w", srcID, err)
		}
		meta := probe.Meta()

		owner, repo, err := parseGitHubURL(meta.SourceURL)
		if err != nil {
			return fmt.Errorf("parse source URL for %q: %w", srcID, err)
		}

		repoCacheDir := filepath.Join(cacheDir, owner, repo)
		lang := canonicalID[:strings.Index(canonicalID, "/")]
		repoPath, resolvedVersion, err := download.EnsureRepo(ctx, download.RepoConfig{
			Owner: owner,
			Repo:  repo,
		}, download.Options{
			CacheDir:  repoCacheDir,
			Refresh:   refresh,
			Method:    downloadMethod,
			Token:     cfg.GitHubToken,
			UserAgent: cfg.UserAgent,
			Validate:  validateByLanguage(lang),
			TagFilter: tagFilterFor(canonicalID),
		})
		if err != nil {
			return fmt.Errorf("download %s/%s: %w", owner, repo, err)
		}

		factoryOpts.Ref = resolvedVersion
		s, err := registry.Default.Create(canonicalID, repoPath, factoryOpts)
		if err != nil {
			return fmt.Errorf("create source %q with repo: %w", srcID, err)
		}
		src = s
		fetcher = crawler.NewLocalFetcher()
	}
	defer fetcher.Close() //nolint:errcheck

	log.Printf("Starting crawl: %s", srcID)
	c := crawler.New(fetcher, st, src, workers)

	crawlOpts := crawler.RunOptions{
		Resume:      resume,
		RetryFailed: retryFailed,
	}

	if err := c.Run(ctx, crawlOpts); err != nil {
		return err
	}
	log.Printf("Completed crawl: %s", srcID)
	return nil
}

// resolveAlias resolves an alias to its canonical ID. Returns the canonical ID
// and true if an alias was found, or the original ID and false otherwise.
func resolveAlias(id string) (string, bool) {
	// Check if the registry can create it (handles aliases internally).
	_, err := registry.Default.Create(id, "", registry.FactoryOptions{})
	if err == nil {
		return id, false
	}
	// If it's not directly found, it might be an alias resolved internally.
	// The registry.Create already resolves aliases, so if it fails the ID is unknown.
	return id, false
}

// sourceTagPrefixes constrains version resolution for repos whose tag
// namespace confuses the generic latest-version heuristic in the download
// package (no GitHub releases combined with mixed tag schemes).
var sourceTagPrefixes = map[string]string{
	// golang/go publishes no GitHub releases; its tags mix go1.x.y with
	// ancient weekly.* and release.r60-style snapshots.
	"go/stdlib": "go1.",
	// gcc-mirror/gcc tags mix releases/gcc-N with basepoints/ and vendor refs.
	"cpp/libstdcpp": "releases/gcc-",
	// sveltejs/kit is a monorepo tagging every package; pin to the kit package.
	"javascript/sveltekit": "@sveltejs/kit@",
}

// tagFilterFor returns a tag filter for the given source ID, or nil when the
// generic resolution heuristic suffices.
func tagFilterFor(sourceID string) download.TagFilterFunc {
	prefix, ok := sourceTagPrefixes[sourceID]
	if !ok {
		return nil
	}
	return func(tag string) bool { return strings.HasPrefix(tag, prefix) }
}

var langExtensions = map[string][]string{
	"php":        {".php"},
	"go":         {".go"},
	"python":     {".py"},
	"rust":       {".rs"},
	"java":       {".java"},
	"javascript": {".js", ".mjs", ".cjs"},
	"typescript": {".ts", ".tsx"},
	"ruby":       {".rb"},
	"csharp":     {".cs"},
	"c":          {".c", ".h"},
	"cpp":        {".cpp", ".cc", ".cxx", ".hpp", ".h"},
}

func validateByLanguage(lang string) func(string) bool {
	exts, ok := langExtensions[lang]
	if !ok {
		return nil
	}
	return func(dir string) bool {
		found := false
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || found {
				return filepath.SkipDir
			}
			if d.IsDir() {
				return nil
			}
			name := strings.ToLower(d.Name())
			for _, ext := range exts {
				if strings.HasSuffix(name, ext) {
					found = true
					return filepath.SkipDir
				}
			}
			return nil
		})
		return found
	}
}

// parseGitHubURL extracts the owner and repo from a GitHub URL like
// "https://github.com/laravel/framework".
func parseGitHubURL(rawURL string) (owner, repo string, err error) {
	rawURL = strings.TrimSuffix(rawURL, ".git")
	rawURL = strings.TrimRight(rawURL, "/")
	const prefix = "github.com/"
	idx := strings.Index(rawURL, prefix)
	if idx < 0 {
		return "", "", fmt.Errorf("not a GitHub URL: %s", rawURL)
	}
	path := rawURL[idx+len(prefix):]
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("cannot extract owner/repo from %s", rawURL)
	}
	return parts[0], parts[1], nil
}

// printRegisteredSources prints all registered sources grouped by language.
func printRegisteredSources() {
	langs := registry.Default.Languages()
	if len(langs) == 0 {
		fmt.Println("No sources registered.")
		return
	}
	fmt.Printf("Registered sources (%d total):\n\n", len(registry.Default.List()))
	for _, lang := range langs {
		ids := registry.Default.ListByLanguage(lang.Language)
		fmt.Printf("  %s (%d):\n", lang.Language, len(ids))
		for _, id := range ids {
			fmt.Printf("    %s\n", id)
		}
		fmt.Println()
	}
}
