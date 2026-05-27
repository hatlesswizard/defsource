package wpgithub

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Errors returned by EnsureRepo. Callers can classify them with errors.Is.
var (
	// ErrVersionNotFound indicates the requested WordPress version has no
	// corresponding tag/tarball upstream.
	ErrVersionNotFound = errors.New("wordpress version not found upstream")
	// ErrRateLimited indicates the GitHub API rejected a request because the
	// rate limit was exhausted.
	ErrRateLimited = errors.New("github api rate limited")
	// ErrUnsafeArchive indicates a tar entry that would escape the extraction
	// root or is of an unsupported (non-file/dir) type.
	ErrUnsafeArchive = errors.New("unsafe archive entry")
	// ErrUnexpectedArchiveLayout indicates the archive did not contain the
	// expected single top-level directory / WordPress source layout.
	ErrUnexpectedArchiveLayout = errors.New("unexpected archive layout")
	// ErrArchiveTooLarge indicates the archive exceeded the extracted-size or
	// entry-count caps (decompression-bomb defense).
	ErrArchiveTooLarge = errors.New("archive too large")
)

// Acquisition methods for DownloadOptions.Method.
const (
	MethodAuto    = "auto"    // try tarball, fall back to git clone
	MethodTarball = "tarball" // HTTP tarball only (codeload.github.com)
	MethodGit     = "git"     // git clone --depth 1 over github.com
)

const (
	defaultAPIBaseURL     = "https://api.github.com"
	defaultTarballBaseURL = "https://github.com"
	defaultGitRepoURL     = "https://github.com/WordPress/WordPress.git"
	defaultUserAgent      = "defSource/1.0 (open-source documentation indexer)"

	// defaultMaxExtractedBytes caps the total decompressed size to defend
	// against decompression bombs. A real WordPress tarball is well under this.
	defaultMaxExtractedBytes int64 = 512 << 20
	// defaultMaxEntries caps the number of tar entries (inode-exhaustion guard).
	defaultMaxEntries = 200_000

	// overallDownloadTimeout bounds the whole resolve+download+extract flow.
	overallDownloadTimeout = 10 * time.Minute
)

// reReleaseTag matches stable WordPress release tags such as "6.5.3" or the
// two-part major form "6.4", optionally prefixed with "v". Pre-release tags
// (alpha/beta/RC) are intentionally rejected.
var reReleaseTag = regexp.MustCompile(`^v?(\d+)\.(\d+)(?:\.(\d+))?$`)

// DownloadOptions configures EnsureRepo. The base-URL fields are injectable so
// tests can point at an httptest.Server; they default to the real endpoints.
type DownloadOptions struct {
	Version        string // empty => resolve latest release
	CacheDir       string // where versioned extractions are cached
	Refresh        bool   // force re-download even if cached
	Method         string // MethodAuto (default), MethodTarball, or MethodGit
	Token          string // optional GitHub API token (GITHUB_TOKEN/GH_TOKEN)
	UserAgent      string
	MaxRetries     int    // per-request retries on transient failures; <=0 => 3
	MaxExtractedB  int64  // override for the extracted-size cap; <=0 => default
	APIBaseURL     string // default https://api.github.com
	TarballBaseURL string // default https://github.com (redirects to codeload)
	GitRepoURL     string // default https://github.com/WordPress/WordPress.git
}

func (o DownloadOptions) withDefaults() DownloadOptions {
	if o.APIBaseURL == "" {
		o.APIBaseURL = defaultAPIBaseURL
	}
	if o.TarballBaseURL == "" {
		o.TarballBaseURL = defaultTarballBaseURL
	}
	if o.GitRepoURL == "" {
		o.GitRepoURL = defaultGitRepoURL
	}
	if o.Method == "" {
		o.Method = MethodAuto
	}
	o.Method = strings.ToLower(o.Method)
	if o.UserAgent == "" {
		o.UserAgent = defaultUserAgent
	}
	if o.CacheDir == "" {
		o.CacheDir = filepath.Join(".", "data", "cache", "wpgithub")
	}
	if o.MaxExtractedB <= 0 {
		o.MaxExtractedB = defaultMaxExtractedBytes
	}
	return o
}

func (o DownloadOptions) maxRetries() int {
	if o.MaxRetries <= 0 {
		return 3
	}
	return o.MaxRetries
}

// EnsureRepo resolves the requested WordPress version (or the latest release
// when Version is empty), ensures an extracted copy exists in the cache, and
// returns the local repository root plus the resolved literal tag. The returned
// root directly contains wp-includes/ and wp-admin/includes/, as the parser
// expects, and the tag is suitable for pinning source links via WithRef.
func EnsureRepo(ctx context.Context, opts DownloadOptions) (repoPath, version string, err error) {
	opts = opts.withDefaults()

	ctx, cancel := context.WithTimeout(ctx, overallDownloadTimeout)
	defer cancel()

	client := newDownloadClient()

	// Resolve to a literal upstream tag.
	tag := normalizeVersion(opts.Version)
	if tag == "" {
		tag, err = resolveLatestVersion(ctx, client, opts)
		if err != nil {
			return "", "", fmt.Errorf("resolve latest version: %w", err)
		}
		log.Printf("Resolved latest WordPress version: %s", tag)
	}

	if err := os.MkdirAll(opts.CacheDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create cache dir %s: %w", opts.CacheDir, err)
	}

	finalDir := filepath.Join(opts.CacheDir, tag)
	if !opts.Refresh && validRepo(finalDir) {
		log.Printf("Using cached WordPress %s at %s", tag, finalDir)
		return finalDir, tag, nil
	}

	dir, err := acquire(ctx, client, opts, tag, finalDir)
	if err != nil {
		return "", "", err
	}
	log.Printf("WordPress %s ready at %s", tag, dir)
	return dir, tag, nil
}

// acquire fetches and installs the source for tag using the configured method.
// MethodAuto tries the HTTP tarball first and falls back to a git clone when the
// tarball fails for any reason other than a genuine "version not found" or a
// context cancellation (some networks block the codeload tarball host while
// permitting the git protocol over github.com).
func acquire(ctx context.Context, client *http.Client, opts DownloadOptions, tag, finalDir string) (string, error) {
	switch opts.Method {
	case MethodTarball:
		return downloadExtractInstall(ctx, client, opts, tag, finalDir)
	case MethodGit:
		return gitCloneInstall(ctx, opts, tag, finalDir)
	case MethodAuto:
		dir, err := downloadExtractInstall(ctx, client, opts, tag, finalDir)
		if err == nil {
			return dir, nil
		}
		if errors.Is(err, ErrVersionNotFound) || isCtxErr(err) {
			return "", err
		}
		log.Printf("Tarball download failed (%v); falling back to git clone", err)
		return gitCloneInstall(ctx, opts, tag, finalDir)
	default:
		return "", fmt.Errorf("unknown download method %q (want %s|%s|%s)", opts.Method, MethodAuto, MethodTarball, MethodGit)
	}
}

// gitCloneInstall shallow-clones the given tag over the git protocol (which
// uses github.com, not the codeload archive host), strips the .git metadata,
// validates the layout, and atomically installs the result at finalDir.
func gitCloneInstall(ctx context.Context, opts DownloadOptions, tag, finalDir string) (string, error) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found on PATH (required for the git download method): %w", err)
	}

	tmpDir, err := os.MkdirTemp(opts.CacheDir, ".tmp-"+tag+"-git-")
	if err != nil {
		return "", fmt.Errorf("create temp clone dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, gitBin,
		"clone", "--depth", "1", "--single-branch", "--branch", tag,
		opts.GitRepoURL, tmpDir)
	// Never block on interactive credential/host prompts.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Printf("Cloning WordPress %s via git from %s", tag, opts.GitRepoURL)
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "Remote branch") || strings.Contains(msg, "not found in upstream") || strings.Contains(msg, "does not exist") {
			return "", fmt.Errorf("version %q (tag %s): %w", opts.Version, tag, ErrVersionNotFound)
		}
		return "", fmt.Errorf("git clone failed: %v: %s", err, msg)
	}

	// The parser only needs the working tree; drop git metadata to keep the cache lean.
	_ = os.RemoveAll(filepath.Join(tmpDir, ".git"))

	if !validRepo(tmpDir) {
		return "", fmt.Errorf("cloned repo is missing wp-includes/ or wp-admin/includes/: %w", ErrUnexpectedArchiveLayout)
	}
	return installAtomically(tmpDir, finalDir, opts.Refresh)
}

// downloadExtractInstall downloads the tarball for tag, extracts it into a
// temp dir on the same volume as the cache, validates the layout, and
// atomically installs it at finalDir.
func downloadExtractInstall(ctx context.Context, client *http.Client, opts DownloadOptions, tag, finalDir string) (string, error) {
	tmpDir, err := os.MkdirTemp(opts.CacheDir, ".tmp-"+tag+"-")
	if err != nil {
		return "", fmt.Errorf("create temp extraction dir: %w", err)
	}
	// Always attempt cleanup: a no-op once the dir has been renamed into place,
	// and the safety net for any error or context-cancel path.
	defer os.RemoveAll(tmpDir)

	resp, err := fetchTarball(ctx, client, opts, tag)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := extractTarGz(ctx, resp.Body, tmpDir, opts.MaxExtractedB, defaultMaxEntries); err != nil {
		return "", err
	}
	if !validRepo(tmpDir) {
		return "", fmt.Errorf("extracted archive is missing wp-includes/ or wp-admin/includes/: %w", ErrUnexpectedArchiveLayout)
	}

	return installAtomically(tmpDir, finalDir, opts.Refresh)
}

// fetchTarball requests the GitHub source tarball for tag and returns the
// successful (200) response with its body open for streaming. The 302 redirect
// to codeload.github.com is followed automatically by the client.
func fetchTarball(ctx context.Context, client *http.Client, opts DownloadOptions, tag string) (*http.Response, error) {
	url := fmt.Sprintf("%s/WordPress/WordPress/archive/refs/tags/%s.tar.gz",
		strings.TrimRight(opts.TarballBaseURL, "/"), tag)

	resp, err := opts.get(ctx, client, url, false)
	if err != nil {
		return nil, fmt.Errorf("download tarball: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp, nil
	case http.StatusNotFound:
		resp.Body.Close()
		return nil, fmt.Errorf("version %q (tag %s): %w", opts.Version, tag, ErrVersionNotFound)
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("download tarball: unexpected status %d", resp.StatusCode)
	}
}

// resolveLatestVersion paginates the GitHub tags API, filters to stable release
// tags, and returns the highest-versioned literal tag string. It does not rely
// on the API's tag ordering (which is lexicographic, not semver).
func resolveLatestVersion(ctx context.Context, client *http.Client, opts DownloadOptions) (string, error) {
	url := strings.TrimRight(opts.APIBaseURL, "/") + "/repos/WordPress/WordPress/tags?per_page=100"

	var (
		best    semver
		bestTag string
		found   bool
	)
	for url != "" {
		tags, next, err := fetchTagPage(ctx, client, opts, url)
		if err != nil {
			return "", err
		}
		for _, t := range tags {
			sv, ok := parseSemver(t.Name)
			if !ok {
				continue
			}
			if !found || best.less(sv) {
				best, bestTag, found = sv, t.Name, true
			}
		}
		url = next
	}
	if !found {
		return "", errors.New("no stable release tags found upstream")
	}
	return bestTag, nil
}

type ghTag struct {
	Name string `json:"name"`
}

// fetchTagPage retrieves one page of tags and the URL of the next page (parsed
// from the Link header), or "" when there is no next page.
func fetchTagPage(ctx context.Context, client *http.Client, opts DownloadOptions, url string) ([]ghTag, string, error) {
	resp, err := opts.get(ctx, client, url, true)
	if err != nil {
		return nil, "", fmt.Errorf("fetch tags: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		// fall through to decode
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return nil, "", fmt.Errorf("github api rate limit exhausted (resets at %s); set GITHUB_TOKEN to raise the limit: %w",
			formatRateLimitReset(resp.Header.Get("X-RateLimit-Reset")), ErrRateLimited)
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, "", fmt.Errorf("github tags api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tags []ghTag
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&tags); err != nil {
		return nil, "", fmt.Errorf("decode tags response: %w", err)
	}
	return tags, parseNextLink(resp.Header.Get("Link")), nil
}

// get issues a GET request with the configured headers, retrying connection
// errors and 5xx responses with bounded exponential backoff. Non-5xx responses
// (200/403/404) are returned to the caller for handling. Context errors are
// returned immediately without retry.
func (o DownloadOptions) get(ctx context.Context, client *http.Client, url string, apiHeaders bool) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= o.maxRetries(); attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("User-Agent", o.UserAgent)
		if apiHeaders {
			req.Header.Set("Accept", "application/vnd.github+json")
			if o.Token != "" {
				req.Header.Set("Authorization", "Bearer "+o.Token)
			}
		}

		resp, err := client.Do(req)
		switch {
		case err != nil:
			if isCtxErr(err) {
				return nil, err
			}
			lastErr = fmt.Errorf("request %s: %w", url, err)
		case resp.StatusCode >= 500:
			resp.Body.Close()
			lastErr = fmt.Errorf("request %s: upstream status %d", url, resp.StatusCode)
		default:
			return resp, nil
		}

		if attempt < o.maxRetries() && !sleepBackoff(ctx, attempt) {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

// extractTarGz streams a gzipped tar from r into destRoot, confining all writes
// to destRoot via os.Root, stripping the single top-level directory, allowing
// only regular files and directories, and enforcing the supplied size and
// entry-count caps. It never buffers the whole archive in memory.
func extractTarGz(ctx context.Context, r io.Reader, destRoot string, maxBytes int64, maxEntries int) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip stream (corrupt download or not a gzip — HTML error page?): %w", err)
	}
	defer gz.Close()

	root, err := os.OpenRoot(destRoot)
	if err != nil {
		return fmt.Errorf("open extraction root: %w", err)
	}
	defer root.Close()

	tr := tar.NewReader(gz)
	var (
		topLevel   string
		totalBytes int64
		entries    int
	)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry (truncated download?): %w", err)
		}

		entries++
		if entries > maxEntries {
			return fmt.Errorf("archive has more than %d entries: %w", maxEntries, ErrArchiveTooLarge)
		}

		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeDir:
		default:
			return fmt.Errorf("entry %q has unsupported type %d: %w", hdr.Name, hdr.Typeflag, ErrUnsafeArchive)
		}

		name := path.Clean(filepath.ToSlash(hdr.Name))
		if name == "." || name == "/" {
			continue
		}
		if name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("entry %q escapes archive root: %w", hdr.Name, ErrUnsafeArchive)
		}

		first, rest, _ := strings.Cut(name, "/")
		if topLevel == "" {
			topLevel = first
		} else if first != topLevel {
			return fmt.Errorf("archive has multiple top-level dirs (%q, %q): %w", topLevel, first, ErrUnexpectedArchiveLayout)
		}
		if rest == "" {
			continue // the top-level directory entry itself; nothing to extract
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := mkdirAllInRoot(root, rest); err != nil {
				return err
			}
		case tar.TypeReg:
			if dir := path.Dir(rest); dir != "." {
				if err := mkdirAllInRoot(root, dir); err != nil {
					return err
				}
			}
			written, err := writeFileInRoot(root, rest, tr, maxBytes-totalBytes)
			if err != nil {
				return err
			}
			totalBytes += written
			if totalBytes > maxBytes {
				return fmt.Errorf("archive decompresses to more than %d bytes: %w", maxBytes, ErrArchiveTooLarge)
			}
		}
	}
	if topLevel == "" {
		return fmt.Errorf("archive is empty: %w", ErrUnexpectedArchiveLayout)
	}
	return nil
}

// writeFileInRoot copies at most remaining+1 bytes from src into the file named
// rel under root, so that an oversize file (beyond the running cap) is detected
// by the caller. It returns the number of bytes written.
func writeFileInRoot(root *os.Root, rel string, src io.Reader, remaining int64) (int64, error) {
	if remaining < 0 {
		remaining = 0
	}
	f, err := root.OpenFile(filepath.FromSlash(rel), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("create %q: %w", rel, err)
	}
	n, copyErr := io.Copy(f, io.LimitReader(src, remaining+1))
	closeErr := f.Close()
	if copyErr != nil {
		return n, fmt.Errorf("write %q: %w", rel, copyErr)
	}
	if closeErr != nil {
		return n, fmt.Errorf("close %q: %w", rel, closeErr)
	}
	return n, nil
}

// mkdirAllInRoot creates rel (a slash-separated path) and all missing parents
// under root, tolerating already-existing directories.
func mkdirAllInRoot(root *os.Root, rel string) error {
	rel = path.Clean(rel)
	if rel == "." || rel == "/" {
		return nil
	}
	var cur string
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			continue
		}
		if cur == "" {
			cur = seg
		} else {
			cur += "/" + seg
		}
		if err := root.Mkdir(filepath.FromSlash(cur), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("mkdir %q: %w", cur, err)
		}
	}
	return nil
}

// installAtomically moves tmpDir into place at finalDir. When refresh is set,
// any existing finalDir is renamed aside and removed after the swap. Without
// refresh, an existing finalDir (e.g. created by a concurrent run) is adopted.
// The rename is the synchronization point — no lock file is needed.
func installAtomically(tmpDir, finalDir string, refresh bool) (string, error) {
	if !refresh {
		if _, err := os.Stat(finalDir); err == nil {
			return finalDir, nil // another run already produced it
		}
	} else if _, err := os.Stat(finalDir); err == nil {
		aside := finalDir + ".old-" + filepath.Base(tmpDir)
		if err := os.Rename(finalDir, aside); err != nil {
			return "", fmt.Errorf("rename existing cache aside: %w", err)
		}
		defer os.RemoveAll(aside)
	}

	if err := os.Rename(tmpDir, finalDir); err != nil {
		// A concurrent run may have created finalDir between our check and rename.
		if !refresh {
			if _, statErr := os.Stat(finalDir); statErr == nil {
				return finalDir, nil
			}
		}
		return "", fmt.Errorf("install cache dir %s: %w", finalDir, err)
	}
	return finalDir, nil
}

// validRepo reports whether dir directly contains the two source roots the
// parser walks (wp-includes/ and wp-admin/includes/). Because installation is
// atomic, existence implies a complete extraction.
func validRepo(dir string) bool {
	for _, sub := range [][]string{{"wp-includes"}, {"wp-admin", "includes"}} {
		fi, err := os.Stat(filepath.Join(append([]string{dir}, sub...)...))
		if err != nil || !fi.IsDir() {
			return false
		}
	}
	return true
}

// newDownloadClient builds an HTTP client suited to large downloads: timeouts
// live on the transport (dial/TLS/response-header) while the overall duration
// is bounded by the request context, not a blunt Client.Timeout.
func newDownloadClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}

// normalizeVersion trims surrounding space and a single leading "v"/"V".
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	return v
}

type semver struct {
	major, minor, patch int
}

// parseSemver parses a stable release tag; two-part tags ("6.4") are treated as
// patch 0 for comparison.
func parseSemver(tag string) (semver, bool) {
	m := reReleaseTag.FindStringSubmatch(tag)
	if m == nil {
		return semver{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch := 0
	if m[3] != "" {
		patch, _ = strconv.Atoi(m[3])
	}
	return semver{major, minor, patch}, true
}

func (a semver) less(b semver) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	return a.patch < b.patch
}

// parseNextLink extracts the rel="next" URL from a GitHub Link header, or "".
func parseNextLink(link string) string {
	if link == "" {
		return ""
	}
	for _, part := range strings.Split(link, ",") {
		segs := strings.Split(strings.TrimSpace(part), ";")
		if len(segs) < 2 {
			continue
		}
		urlPart := strings.TrimSpace(segs[0])
		if !strings.HasPrefix(urlPart, "<") || !strings.HasSuffix(urlPart, ">") {
			continue
		}
		for _, attr := range segs[1:] {
			if strings.TrimSpace(attr) == `rel="next"` {
				return urlPart[1 : len(urlPart)-1]
			}
		}
	}
	return ""
}

func formatRateLimitReset(reset string) string {
	if reset == "" {
		return "unknown time"
	}
	if sec, err := strconv.ParseInt(reset, 10, 64); err == nil {
		return time.Unix(sec, 0).Format(time.RFC3339)
	}
	return reset
}

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// sleepBackoff waits 2^(attempt-1) seconds (capped at 30s) or until ctx is
// done. It returns false if the context was cancelled during the wait.
func sleepBackoff(ctx context.Context, attempt int) bool {
	d := time.Duration(1<<(attempt-1)) * time.Second
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
