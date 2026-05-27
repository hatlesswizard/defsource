package config

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test 1: Load() with no env vars set — all built-in defaults are returned.
// ---------------------------------------------------------------------------

func TestLoad_Defaults(t *testing.T) {
	// Ensure none of the env vars are set so we exercise the pure-default path.
	unsetAll(t)

	cfg := Load()

	if cfg.DBPath != "./data/defsource.db" {
		t.Errorf("DBPath: got %q, want %q", cfg.DBPath, "./data/defsource.db")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3", cfg.MaxRetries)
	}
	if cfg.TokenBudget != 8000 {
		t.Errorf("TokenBudget: got %d, want 8000", cfg.TokenBudget)
	}
	if cfg.UserAgent != "defSource/1.0 (open-source documentation indexer)" {
		t.Errorf("UserAgent: got %q, want %q",
			cfg.UserAgent, "defSource/1.0 (open-source documentation indexer)")
	}
	if cfg.ServerAddr != ":8080" {
		t.Errorf("ServerAddr: got %q, want %q", cfg.ServerAddr, ":8080")
	}
	if cfg.Workers != 10 {
		t.Errorf("Workers: got %d, want 10", cfg.Workers)
	}
	if cfg.RequestsPerSecond != 10 {
		t.Errorf("RequestsPerSecond: got %d, want 10", cfg.RequestsPerSecond)
	}
	if cfg.CORSOrigin != "*" {
		t.Errorf("CORSOrigin: got %q, want %q", cfg.CORSOrigin, "*")
	}
}

// ---------------------------------------------------------------------------
// Test 2: DEFSOURCE_DB_PATH overrides the default path.
// ---------------------------------------------------------------------------

func TestLoad_DBPath_Override(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_DB_PATH", "/tmp/my-custom.db")

	cfg := Load()

	if cfg.DBPath != "/tmp/my-custom.db" {
		t.Errorf("DBPath: got %q, want %q", cfg.DBPath, "/tmp/my-custom.db")
	}
	// All other fields must still be defaults.
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3 (should be unaffected)", cfg.MaxRetries)
	}
}

// ---------------------------------------------------------------------------
// Test 3: DEFSOURCE_TOKEN_BUDGET="16000" — integer parsing succeeds.
// ---------------------------------------------------------------------------

func TestLoad_TokenBudget_ValidInt(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_TOKEN_BUDGET", "16000")

	cfg := Load()

	if cfg.TokenBudget != 16000 {
		t.Errorf("TokenBudget: got %d, want 16000", cfg.TokenBudget)
	}
}

// ---------------------------------------------------------------------------
// Test 4: DEFSOURCE_TOKEN_BUDGET="abc" — invalid value is gracefully handled.
//
// Characterization: envInt cannot parse "abc", so it logs a warning and returns
// the built-in default (8000). No panic must occur.
// ---------------------------------------------------------------------------

func TestLoad_TokenBudget_InvalidInt_FallsBackToDefault(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_TOKEN_BUDGET", "abc")

	// Capture the log output so we can assert the warning is emitted.
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	cfg := Load()

	// Must not panic, and must fall back to the default.
	if cfg.TokenBudget != 8000 {
		t.Errorf("TokenBudget: got %d, want 8000 (default after bad parse)", cfg.TokenBudget)
	}

	// The warning message must reference the env var name and the bad value.
	logged := buf.String()
	if !strings.Contains(logged, "DEFSOURCE_TOKEN_BUDGET") {
		t.Errorf("expected warning to mention DEFSOURCE_TOKEN_BUDGET; got: %q", logged)
	}
	if !strings.Contains(logged, "abc") {
		t.Errorf("expected warning to include the invalid value %q; got: %q", "abc", logged)
	}
}

// ---------------------------------------------------------------------------
// Test 5: DEFSOURCE_USER_AGENT is applied.
// ---------------------------------------------------------------------------

func TestLoad_UserAgent_Override(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_USER_AGENT", "MyBot/2.0")

	cfg := Load()

	if cfg.UserAgent != "MyBot/2.0" {
		t.Errorf("UserAgent: got %q, want %q", cfg.UserAgent, "MyBot/2.0")
	}
}

// ---------------------------------------------------------------------------
// Test 6: DEFSOURCE_WORKERS="0" — characterize: zero IS a valid integer, so
// envInt returns 0 (it does not clamp to the default). Callers must guard
// against a zero-worker pool themselves.
// ---------------------------------------------------------------------------

func TestLoad_Workers_Zero(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_WORKERS", "0")

	cfg := Load()

	// Zero is a valid strconv.Atoi result, so envInt returns it as-is.
	// This characterizes the current behaviour: 0 is NOT treated as absent.
	if cfg.Workers != 0 {
		t.Errorf("Workers: got %d, want 0 (zero is a valid parse; callers must clamp)", cfg.Workers)
	}
}

// ---------------------------------------------------------------------------
// Test 7: DEFSOURCE_RPS is applied.
// ---------------------------------------------------------------------------

func TestLoad_RequestsPerSecond_Override(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_RPS", "25")

	cfg := Load()

	if cfg.RequestsPerSecond != 25 {
		t.Errorf("RequestsPerSecond: got %d, want 25", cfg.RequestsPerSecond)
	}
}

// ---------------------------------------------------------------------------
// Test 8: DEFSOURCE_ADDR is applied.
// ---------------------------------------------------------------------------

func TestLoad_ServerAddr_Override(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_ADDR", ":9890")

	cfg := Load()

	if cfg.ServerAddr != ":9890" {
		t.Errorf("ServerAddr: got %q, want %q", cfg.ServerAddr, ":9890")
	}
}

// ---------------------------------------------------------------------------
// Test 9: DEFSOURCE_MAX_RETRIES is applied.
// ---------------------------------------------------------------------------

func TestLoad_MaxRetries_Override(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_MAX_RETRIES", "7")

	cfg := Load()

	if cfg.MaxRetries != 7 {
		t.Errorf("MaxRetries: got %d, want 7", cfg.MaxRetries)
	}
}

// ---------------------------------------------------------------------------
// Test 10: DEFSOURCE_CORS_ORIGIN is applied.
// ---------------------------------------------------------------------------

func TestLoad_CORSOrigin_Override(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_CORS_ORIGIN", "https://example.com")

	cfg := Load()

	if cfg.CORSOrigin != "https://example.com" {
		t.Errorf("CORSOrigin: got %q, want %q", cfg.CORSOrigin, "https://example.com")
	}
}

// ---------------------------------------------------------------------------
// Test 11: DEFSOURCE_CORS_ORIGIN unset — defaults to "*".
// ---------------------------------------------------------------------------

func TestLoad_CORSOrigin_Default(t *testing.T) {
	unsetAll(t)
	// DEFSOURCE_CORS_ORIGIN is intentionally NOT set.

	cfg := Load()

	if cfg.CORSOrigin != "*" {
		t.Errorf("CORSOrigin: got %q, want %q (open wildcard default)", cfg.CORSOrigin, "*")
	}
}

// ---------------------------------------------------------------------------
// Test 12: envInt helper — negative values are preserved as-is.
//
// envInt has no lower-bound guard; negative integers are valid strconv.Atoi
// results and are returned without clamping.
// ---------------------------------------------------------------------------

func TestLoad_Workers_Negative_IsPreserved(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_WORKERS", "-5")

	cfg := Load()

	// Characterization: -5 is a valid parse; the default is NOT used.
	if cfg.Workers != -5 {
		t.Errorf("Workers: got %d, want -5 (negative parse is preserved)", cfg.Workers)
	}
}

// ---------------------------------------------------------------------------
// Test 13: envInt helper — whitespace-only value triggers fallback.
//
// os.Getenv returns " " (non-empty) for whitespace values. strconv.Atoi(" ")
// fails, so the fallback is used and a warning is logged.
// ---------------------------------------------------------------------------

func TestLoad_MaxRetries_Whitespace_FallsBackToDefault(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_MAX_RETRIES", "   ")

	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	cfg := Load()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3 (whitespace is not a valid int)", cfg.MaxRetries)
	}

	logged := buf.String()
	if !strings.Contains(logged, "DEFSOURCE_MAX_RETRIES") {
		t.Errorf("expected warning to mention DEFSOURCE_MAX_RETRIES; got: %q", logged)
	}
}

// ---------------------------------------------------------------------------
// Test 14: envOr helper — empty string value falls back to default.
//
// t.Setenv with an empty value ("") is indistinguishable from unset at the
// os.Getenv level (both return ""); the fallback default is used.
// ---------------------------------------------------------------------------

func TestLoad_DBPath_EmptyEnvVar_UsesDefault(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_DB_PATH", "")

	cfg := Load()

	// An empty env var should behave identically to an absent one.
	if cfg.DBPath != "./data/defsource.db" {
		t.Errorf("DBPath: got %q, want default %q", cfg.DBPath, "./data/defsource.db")
	}
}

// ---------------------------------------------------------------------------
// Test 15: All env vars set simultaneously — each field reflects its override.
//
// Exercises that no field bleeds into another (independent os.Getenv calls).
// ---------------------------------------------------------------------------

func TestLoad_AllFieldsOverriddenTogether(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_DB_PATH", "/data/custom.db")
	t.Setenv("DEFSOURCE_MAX_RETRIES", "5")
	t.Setenv("DEFSOURCE_TOKEN_BUDGET", "4000")
	t.Setenv("DEFSOURCE_USER_AGENT", "TestAgent/1.0")
	t.Setenv("DEFSOURCE_ADDR", ":7070")
	t.Setenv("DEFSOURCE_WORKERS", "20")
	t.Setenv("DEFSOURCE_RPS", "50")
	t.Setenv("DEFSOURCE_CORS_ORIGIN", "https://test.example")

	cfg := Load()

	if cfg.DBPath != "/data/custom.db" {
		t.Errorf("DBPath: got %q, want /data/custom.db", cfg.DBPath)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries: got %d, want 5", cfg.MaxRetries)
	}
	if cfg.TokenBudget != 4000 {
		t.Errorf("TokenBudget: got %d, want 4000", cfg.TokenBudget)
	}
	if cfg.UserAgent != "TestAgent/1.0" {
		t.Errorf("UserAgent: got %q, want TestAgent/1.0", cfg.UserAgent)
	}
	if cfg.ServerAddr != ":7070" {
		t.Errorf("ServerAddr: got %q, want :7070", cfg.ServerAddr)
	}
	if cfg.Workers != 20 {
		t.Errorf("Workers: got %d, want 20", cfg.Workers)
	}
	if cfg.RequestsPerSecond != 50 {
		t.Errorf("RequestsPerSecond: got %d, want 50", cfg.RequestsPerSecond)
	}
	if cfg.CORSOrigin != "https://test.example" {
		t.Errorf("CORSOrigin: got %q, want https://test.example", cfg.CORSOrigin)
	}
}

// ---------------------------------------------------------------------------
// Test 16: envInt helper — float string triggers fallback with warning.
// ---------------------------------------------------------------------------

func TestLoad_Workers_FloatString_FallsBackToDefault(t *testing.T) {
	unsetAll(t)
	t.Setenv("DEFSOURCE_WORKERS", "3.14")

	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	cfg := Load()

	if cfg.Workers != 10 {
		t.Errorf("Workers: got %d, want 10 (float is not a valid int)", cfg.Workers)
	}
	logged := buf.String()
	if !strings.Contains(logged, "DEFSOURCE_WORKERS") {
		t.Errorf("expected warning to mention DEFSOURCE_WORKERS; got: %q", logged)
	}
	if !strings.Contains(logged, "3.14") {
		t.Errorf("expected warning to include the invalid value 3.14; got: %q", logged)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// unsetAll removes all DEFSOURCE_* env vars for the duration of the test,
// ensuring each test runs in a clean environment regardless of the shell
// environment the test runner is invoked from.
func unsetAll(t *testing.T) {
	t.Helper()
	vars := []string{
		"DEFSOURCE_DB_PATH",
		"DEFSOURCE_MAX_RETRIES",
		"DEFSOURCE_TOKEN_BUDGET",
		"DEFSOURCE_USER_AGENT",
		"DEFSOURCE_ADDR",
		"DEFSOURCE_WORKERS",
		"DEFSOURCE_RPS",
		"DEFSOURCE_CORS_ORIGIN",
	}
	for _, v := range vars {
		t.Setenv(v, "") // t.Setenv restores on cleanup; set to "" to ensure os.Getenv returns ""
	}
}
