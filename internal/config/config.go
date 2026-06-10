// Package config loads defSource runtime configuration from environment variables.
//
// Precedence (highest to lowest):
//  1. Explicit CLI flag value (callers pass flag defaults as cfg.Field)
//  2. Environment variable (read by Load)
//  3. Built-in default (hard-coded in Load)
//
// Usage pattern for cmd mains:
//
//	cfg := config.Load()
//	dbPath := flag.String("db", cfg.DBPath, "Path to SQLite database")
//	flag.Parse()
//
// After flag.Parse, *dbPath holds the explicit flag value when the user
// passed --db, or the env-var-aware default from cfg.DBPath otherwise.
package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all runtime configuration for defSource binaries.
// Fields are populated by Load from environment variables with built-in
// defaults; CLI flags should take their default values from this struct
// so that env-var overrides are honoured when no flag is passed explicitly.
type Config struct {
	// DBPath is the filesystem path to the SQLite database file.
	// Env: DEFSOURCE_DB_PATH. Default: "./data/defsource.db".
	DBPath string

	// MaxRetries is the maximum number of retry attempts for a failing HTTP fetch.
	// Env: DEFSOURCE_MAX_RETRIES. Default: 3.
	MaxRetries int

	// TokenBudget is the approximate token limit for formatted doc snippet output.
	// Env: DEFSOURCE_TOKEN_BUDGET. Default: 8000.
	TokenBudget int

	// UserAgent is the HTTP User-Agent header sent by the crawler fetcher.
	// Env: DEFSOURCE_USER_AGENT. Default: "defSource/1.0 (open-source documentation indexer)".
	UserAgent string

	// ServerAddr is the TCP address the HTTP API server listens on.
	// Env: DEFSOURCE_ADDR. Default: ":8080".
	ServerAddr string

	// Workers is the number of concurrent crawler worker goroutines.
	// Env: DEFSOURCE_WORKERS. Default: 10.
	Workers int

	// RequestsPerSecond is the crawler fetch rate limit in requests per second.
	// Env: DEFSOURCE_RPS. Default: 10.
	RequestsPerSecond int

	// CORSOrigin is the value of the Access-Control-Allow-Origin header.
	// Env: DEFSOURCE_CORS_ORIGIN. Default: "*".
	CORSOrigin string

	// CacheDir is the directory where downloaded source repositories are
	// extracted and cached (one subdirectory per owner/repo/version).
	// Env: DEFSOURCE_CACHE_DIR. Default: <os.UserCacheDir>/defsource/sources,
	// falling back to "./data/cache" when the user cache dir is unavailable.
	CacheDir string

	// WPVersion is the WordPress release to download. Empty means resolve the
	// latest release tag from the GitHub tags API.
	// Env: DEFSOURCE_WP_VERSION. Default: "" (latest).
	WPVersion string

	// GitHubToken is an optional GitHub API token used to raise the API rate
	// limit when resolving the latest version. It is never sent to the tarball
	// (codeload) download.
	// Env: GITHUB_TOKEN, then GH_TOKEN. Default: "".
	GitHubToken string
}

// Load returns a Config populated from environment variables, falling back to
// built-in defaults for any variable that is absent or empty. The returned
// Config should be used as the source of default values for CLI flag
// definitions so that the env-var layer is respected when no explicit flag
// is passed by the operator.
func Load() Config {
	return Config{
		DBPath:            envOr("DEFSOURCE_DB_PATH", "./data/defsource.db"),
		MaxRetries:        envInt("DEFSOURCE_MAX_RETRIES", 3),
		TokenBudget:       envInt("DEFSOURCE_TOKEN_BUDGET", 8000),
		UserAgent:         envOr("DEFSOURCE_USER_AGENT", "defSource/1.0 (open-source documentation indexer)"),
		ServerAddr:        envOr("DEFSOURCE_ADDR", ":8080"),
		Workers:           envInt("DEFSOURCE_WORKERS", 10),
		RequestsPerSecond: envInt("DEFSOURCE_RPS", 10),
		CORSOrigin:        envOr("DEFSOURCE_CORS_ORIGIN", "*"),
		CacheDir:          envOr("DEFSOURCE_CACHE_DIR", defaultCacheDir()),
		WPVersion:         envOr("DEFSOURCE_WP_VERSION", ""),
		GitHubToken:       envOr("GITHUB_TOKEN", envOr("GH_TOKEN", "")),
	}
}

func defaultCacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "defsource", "sources")
	}
	return filepath.Join(".", "data", "cache")
}

// envOr returns the value of the environment variable named by key, or
// fallback if the variable is absent or empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt returns the integer value of the environment variable named by key,
// or fallback if the variable is absent, empty, or not a valid integer.
// An invalid value is logged as a warning so operators are notified at startup.
func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Printf("WARNING: invalid value for %s: %q, using default %d", key, v, fallback)
	}
	return fallback
}
