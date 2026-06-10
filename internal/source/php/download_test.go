//go:build sqlite_fts5 || fts5

package php

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tarball fixtures
// ---------------------------------------------------------------------------

type tarEntry struct {
	name     string
	typeflag byte
	body     string
	linkname string
}

// makeTarGz builds a gzipped tar archive from the given entries in memory.
func makeTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: e.typeflag,
			Mode:     0o644,
			Linkname: e.linkname,
			Size:     int64(len(e.body)),
		}
		if e.typeflag == tar.TypeDir {
			hdr.Mode = 0o755
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%q): %v", e.name, err)
		}
		if e.typeflag == tar.TypeReg && len(e.body) > 0 {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("Write(%q): %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// wpTarGz builds a minimal WordPress-like tarball whose entries are nested
// under the single top-level directory top (mirroring GitHub's layout).
func wpTarGz(t *testing.T, top string) []byte {
	t.Helper()
	return makeTarGz(t, []tarEntry{
		{name: top + "/", typeflag: tar.TypeDir},
		{name: top + "/wp-includes/", typeflag: tar.TypeDir},
		{name: top + "/wp-includes/version.php", typeflag: tar.TypeReg, body: "<?php\n$wp_version = '6.5.3';\n"},
		{name: top + "/wp-admin/", typeflag: tar.TypeDir},
		{name: top + "/wp-admin/includes/", typeflag: tar.TypeDir},
		{name: top + "/wp-admin/includes/plugin.php", typeflag: tar.TypeReg, body: "<?php\n// plugin helpers\n"},
	})
}

// ---------------------------------------------------------------------------
// extractTarGz — security and correctness
// ---------------------------------------------------------------------------

func TestExtractTarGz_HappyPath_StripsTopLevel(t *testing.T) {
	dest := t.TempDir()
	data := wpTarGz(t, "WordPress-6.5.3")

	if err := extractTarGz(context.Background(), bytes.NewReader(data), dest, defaultMaxExtractedBytes, defaultMaxEntries); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	// Top-level component must be stripped: files land directly under dest.
	if !validRepo(dest) {
		t.Fatalf("validRepo(%q) = false; expected wp-includes/ and wp-admin/includes/ at root", dest)
	}
	if _, err := readFile(dest, "wp-includes", "version.php"); err != nil {
		t.Errorf("version.php not extracted at root: %v", err)
	}
	// The WordPress-6.5.3/ prefix must NOT exist as a real directory.
	if _, err := readFile(dest, "WordPress-6.5.3", "wp-includes", "version.php"); err == nil {
		t.Error("top-level WordPress-6.5.3/ directory was not stripped")
	}
}

func TestExtractTarGz_RejectsPathTraversal(t *testing.T) {
	dest := t.TempDir()
	data := makeTarGz(t, []tarEntry{
		{name: "WordPress-x/", typeflag: tar.TypeDir},
		{name: "../evil.php", typeflag: tar.TypeReg, body: "pwned"},
	})

	err := extractTarGz(context.Background(), bytes.NewReader(data), dest, defaultMaxExtractedBytes, defaultMaxEntries)
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("extractTarGz error = %v; want ErrUnsafeArchive", err)
	}
}

func TestExtractTarGz_RejectsSymlink(t *testing.T) {
	dest := t.TempDir()
	data := makeTarGz(t, []tarEntry{
		{name: "WordPress-x/", typeflag: tar.TypeDir},
		{name: "WordPress-x/link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"},
	})

	err := extractTarGz(context.Background(), bytes.NewReader(data), dest, defaultMaxExtractedBytes, defaultMaxEntries)
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("extractTarGz error = %v; want ErrUnsafeArchive", err)
	}
}

func TestExtractTarGz_EnforcesSizeCap(t *testing.T) {
	dest := t.TempDir()
	data := makeTarGz(t, []tarEntry{
		{name: "WordPress-x/", typeflag: tar.TypeDir},
		{name: "WordPress-x/big.php", typeflag: tar.TypeReg, body: strings.Repeat("A", 100)},
	})

	err := extractTarGz(context.Background(), bytes.NewReader(data), dest, 10 /* maxBytes */, defaultMaxEntries)
	if !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("extractTarGz error = %v; want ErrArchiveTooLarge", err)
	}
}

func TestExtractTarGz_EnforcesEntryCap(t *testing.T) {
	dest := t.TempDir()
	data := wpTarGz(t, "WordPress-x") // 6 entries

	err := extractTarGz(context.Background(), bytes.NewReader(data), dest, defaultMaxExtractedBytes, 2 /* maxEntries */)
	if !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("extractTarGz error = %v; want ErrArchiveTooLarge", err)
	}
}

func TestExtractTarGz_RejectsNonGzip(t *testing.T) {
	dest := t.TempDir()
	err := extractTarGz(context.Background(), strings.NewReader("<html>not a gzip</html>"), dest, defaultMaxExtractedBytes, defaultMaxEntries)
	if err == nil {
		t.Fatal("expected error for non-gzip input, got nil")
	}
}

// ---------------------------------------------------------------------------
// EnsureRepo — resolution, download, caching, error mapping
// ---------------------------------------------------------------------------

func TestEnsureRepo_ResolvesLatest(t *testing.T) {
	srv := newWPTestServer(t, nil)

	repoPath, version, err := EnsureRepo(context.Background(), DownloadOptions{
		CacheDir:       t.TempDir(),
		APIBaseURL:     srv.URL,
		TarballBaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	// 6.10 > 6.9 numerically — proves semver (not lexicographic) selection.
	if version != "6.10" {
		t.Errorf("resolved version = %q, want %q", version, "6.10")
	}
	if !validRepo(repoPath) {
		t.Errorf("validRepo(%q) = false", repoPath)
	}
}

func TestEnsureRepo_ExplicitVersionAndCacheReuse(t *testing.T) {
	var downloads atomic.Int32
	srv := newWPTestServer(t, &downloads)
	cacheDir := t.TempDir()

	opts := DownloadOptions{
		Version:        "v6.4", // leading v must be normalised away
		CacheDir:       cacheDir,
		APIBaseURL:     srv.URL,
		TarballBaseURL: srv.URL,
	}

	repoPath, version, err := EnsureRepo(context.Background(), opts)
	if err != nil {
		t.Fatalf("EnsureRepo (first): %v", err)
	}
	if version != "6.4" {
		t.Errorf("version = %q, want %q", version, "6.4")
	}
	if !validRepo(repoPath) {
		t.Errorf("validRepo(%q) = false", repoPath)
	}
	if got := downloads.Load(); got != 1 {
		t.Fatalf("downloads after first call = %d, want 1", got)
	}

	// Second call with the same version+cache must reuse, not re-download.
	if _, _, err := EnsureRepo(context.Background(), opts); err != nil {
		t.Fatalf("EnsureRepo (second): %v", err)
	}
	if got := downloads.Load(); got != 1 {
		t.Errorf("downloads after cached second call = %d, want 1 (cache not reused)", got)
	}
}

func TestEnsureRepo_VersionNotFound(t *testing.T) {
	srv := newWPTestServer(t, nil)

	_, _, err := EnsureRepo(context.Background(), DownloadOptions{
		Version:        "99.99.99",
		CacheDir:       t.TempDir(),
		APIBaseURL:     srv.URL,
		TarballBaseURL: srv.URL,
	})
	if !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("EnsureRepo error = %v; want ErrVersionNotFound", err)
	}
}

func TestEnsureRepo_GitClone(t *testing.T) {
	repo := makeGitRepo(t, "6.4") // skips if git is unavailable

	repoPath, version, err := EnsureRepo(context.Background(), DownloadOptions{
		Version:    "6.4",
		Method:     MethodGit,
		CacheDir:   t.TempDir(),
		GitRepoURL: repo,
	})
	if err != nil {
		t.Fatalf("EnsureRepo(git): %v", err)
	}
	if version != "6.4" {
		t.Errorf("version = %q, want %q", version, "6.4")
	}
	if !validRepo(repoPath) {
		t.Errorf("validRepo(%q) = false", repoPath)
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git was not stripped from the clone")
	}
}

func TestEnsureRepo_AutoFallsBackToGit(t *testing.T) {
	repo := makeGitRepo(t, "6.4") // skips if git is unavailable

	// A tarball host that always 503s, mirroring a proxy that blocks codeload.
	mux := http.NewServeMux()
	mux.HandleFunc("/WordPress/WordPress/archive/refs/tags/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	repoPath, version, err := EnsureRepo(context.Background(), DownloadOptions{
		Version:        "6.4",
		Method:         MethodAuto,
		CacheDir:       t.TempDir(),
		TarballBaseURL: srv.URL,
		APIBaseURL:     srv.URL,
		GitRepoURL:     repo,
		MaxRetries:     1, // fail the tarball fast, then fall back
	})
	if err != nil {
		t.Fatalf("EnsureRepo(auto fallback): %v", err)
	}
	if version != "6.4" {
		t.Errorf("version = %q, want %q", version, "6.4")
	}
	if !validRepo(repoPath) {
		t.Errorf("validRepo(%q) = false", repoPath)
	}
}

func TestEnsureRepo_RateLimited(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/WordPress/WordPress/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
		w.WriteHeader(http.StatusForbidden)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	_, _, err := EnsureRepo(context.Background(), DownloadOptions{
		CacheDir:       t.TempDir(),
		APIBaseURL:     srv.URL,
		TarballBaseURL: srv.URL,
	})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("EnsureRepo error = %v; want ErrRateLimited", err)
	}
}

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

func TestBlobBase_PinsToRef(t *testing.T) {
	if got := New("x", WithRef("6.5.3")).blobBase(); got != "https://github.com/WordPress/WordPress/blob/6.5.3/" {
		t.Errorf("blobBase with ref = %q", got)
	}
	if got := New("x").blobBase(); got != "https://github.com/WordPress/WordPress/blob/master/" {
		t.Errorf("blobBase default = %q", got)
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"6.5.3":  "6.5.3",
		"v6.5.3": "6.5.3",
		"V6.4":   "6.4",
		" 6.4 ":  "6.4",
		"":       "",
	}
	for in, want := range cases {
		if got := normalizeVersion(in); got != want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseSemverOrdering(t *testing.T) {
	// 6.10 must sort above 6.9 (numeric, not lexicographic), and pre-release
	// tags must be rejected.
	for _, bad := range []string{"6.6-beta1", "not-a-version", "6.x", "6"} {
		if _, ok := parseSemver(bad); ok {
			t.Errorf("parseSemver(%q) accepted; want rejected", bad)
		}
	}
	a, _ := parseSemver("6.9")
	b, _ := parseSemver("6.10")
	if !a.less(b) {
		t.Errorf("expected 6.9 < 6.10")
	}
	c, _ := parseSemver("6.4") // two-part => patch 0
	d, _ := parseSemver("6.4.1")
	if !c.less(d) {
		t.Errorf("expected 6.4 < 6.4.1")
	}
}

func TestParseNextLink(t *testing.T) {
	hdr := `<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=9>; rel="last"`
	if got := parseNextLink(hdr); got != "https://api.github.com/x?page=2" {
		t.Errorf("parseNextLink = %q", got)
	}
	if got := parseNextLink(`<https://api.github.com/x?page=9>; rel="last"`); got != "" {
		t.Errorf("parseNextLink without next = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Test server + small helpers
// ---------------------------------------------------------------------------

// newWPTestServer serves a paginated tags API and per-tag tarballs. When
// downloads is non-nil it counts tarball requests (for cache-reuse assertions).
func newWPTestServer(t *testing.T, downloads *atomic.Int32) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/WordPress/WordPress/tags", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			w.Header().Set("Link", "<"+srv.URL+"/repos/WordPress/WordPress/tags?per_page=100&page=2>; rel=\"next\"")
			writeJSON(t, w, []ghTag{{Name: "6.4"}, {Name: "6.10"}, {Name: "5.9"}})
		case "2":
			writeJSON(t, w, []ghTag{{Name: "6.9"}, {Name: "6.5.3"}, {Name: "not-a-version"}, {Name: "6.6-beta1"}})
		default:
			writeJSON(t, w, []ghTag{})
		}
	})

	mux.HandleFunc("/WordPress/WordPress/archive/refs/tags/", func(w http.ResponseWriter, r *http.Request) {
		tag := strings.TrimSuffix(path.Base(r.URL.Path), ".tar.gz")
		if tag == "99.99.99" {
			http.NotFound(w, r)
			return
		}
		if downloads != nil {
			downloads.Add(1)
		}
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(wpTarGz(t, "WordPress-"+tag))
	})

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode json: %v", err)
	}
}

// readFile reads dest/parts... and returns its contents.
func readFile(dest string, parts ...string) ([]byte, error) {
	return os.ReadFile(filepath.Join(append([]string{dest}, parts...)...))
}

// makeGitRepo builds a local git repository with a minimal WordPress layout and
// the given tag, usable as a clone source. It skips the test when git is absent.
func makeGitRepo(t *testing.T, tag string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "wp-includes", "version.php"), "<?php\n$wp_version = '"+tag+"';\n")
	mustWrite(t, filepath.Join(dir, "wp-admin", "includes", "plugin.php"), "<?php\n")

	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "-c", "user.email=t@example.com", "-c", "user.name=test", "commit", "-q", "-m", "init")
	runGit(t, dir, "tag", tag)
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
