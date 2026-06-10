package download

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

var (
	ErrVersionNotFound      = errors.New("version not found upstream")
	ErrRateLimited          = errors.New("github api rate limited")
	ErrUnsafeArchive        = errors.New("unsafe archive entry")
	ErrUnexpectedLayout     = errors.New("unexpected archive layout")
	ErrArchiveTooLarge      = errors.New("archive too large")
	ErrValidationFailed     = errors.New("validation failed")
)

const (
	MethodAuto    = "auto"
	MethodTarball = "tarball"
	MethodGit     = "git"
)

const (
	defaultAPIBaseURL     = "https://api.github.com"
	defaultTarballBaseURL = "https://github.com"
	defaultUserAgent      = "defSource/1.0 (open-source documentation indexer)"
	defaultMaxExtractedB  int64 = 4 << 30 // 4 GiB: large repos (nodejs, esp-idf, qt) exceed 512 MiB extracted
	defaultMaxEntries           = 500_000
	overallTimeout              = 30 * time.Minute // large repos (angular, dpdk, abp) exceed 10 minutes
	maxTagPages                 = 10                // bound tag pagination: 10 pages x 100 tags
)

type ValidateFunc func(repoPath string) bool

type TagFilterFunc func(tagName string) bool

// pathUnsafeChars matches characters that cannot appear in a single path
// element on all supported platforms. Tags like "stable/5.1.x" (django),
// "@sveltejs/kit@2.0.0" (monorepos), or "rel/commons-lang-3.20.0" (Apache)
// contain separators that would break cache-dir and temp-dir construction.
var pathUnsafeChars = regexp.MustCompile(`[/\\@:*?"<>| ]+`)

// sanitizeTag converts a git tag into a string safe to use as a single
// filesystem path element. The original tag must still be used for git
// and GitHub API operations.
func sanitizeTag(tag string) string {
	s := pathUnsafeChars.ReplaceAllString(tag, "-")
	s = strings.Trim(s, "-.")
	if s == "" {
		return "unknown"
	}
	return s
}

type RepoConfig struct {
	Owner   string
	Repo    string
	GitURL  string // override; default is https://github.com/{Owner}/{Repo}.git
}

func (rc RepoConfig) gitURL() string {
	if rc.GitURL != "" {
		return rc.GitURL
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", rc.Owner, rc.Repo)
}

type Options struct {
	Version        string
	CacheDir       string
	Refresh        bool
	Method         string
	Token          string
	UserAgent      string
	MaxRetries     int
	MaxExtractedB  int64
	APIBaseURL     string
	TarballBaseURL string
	Validate       ValidateFunc
	TagFilter      TagFilterFunc
}

func (o Options) withDefaults(rc RepoConfig) Options {
	if o.APIBaseURL == "" {
		o.APIBaseURL = defaultAPIBaseURL
	}
	if o.TarballBaseURL == "" {
		o.TarballBaseURL = defaultTarballBaseURL
	}
	if o.Method == "" {
		o.Method = MethodAuto
	}
	o.Method = strings.ToLower(o.Method)
	if o.UserAgent == "" {
		o.UserAgent = defaultUserAgent
	}
	if o.CacheDir == "" {
		home, _ := os.UserHomeDir()
		if home == "" {
			home = "."
		}
		o.CacheDir = filepath.Join(home, ".defSource", "cache", rc.Owner, rc.Repo)
	}
	if o.MaxExtractedB <= 0 {
		o.MaxExtractedB = defaultMaxExtractedB
	}
	if o.Validate == nil {
		o.Validate = func(string) bool { return true }
	}
	return o
}

func (o Options) maxRetries() int {
	if o.MaxRetries <= 0 {
		return 3
	}
	return o.MaxRetries
}

func EnsureRepo(ctx context.Context, rc RepoConfig, opts Options) (repoPath, version string, err error) {
	opts = opts.withDefaults(rc)

	ctx, cancel := context.WithTimeout(ctx, overallTimeout)
	defer cancel()

	client := newClient()

	tag := strings.TrimSpace(opts.Version)
	if tag == "" {
		tag, err = resolveLatestTag(ctx, client, rc, opts)
		if err != nil {
			return "", "", fmt.Errorf("resolve latest version: %w", err)
		}
		log.Printf("Resolved latest %s/%s version: %s", rc.Owner, rc.Repo, tag)
	}

	if err := os.MkdirAll(opts.CacheDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create cache dir %s: %w", opts.CacheDir, err)
	}

	finalDir := filepath.Join(opts.CacheDir, sanitizeTag(tag))
	if !opts.Refresh && opts.Validate(finalDir) {
		log.Printf("Using cached %s/%s %s at %s", rc.Owner, rc.Repo, tag, finalDir)
		return finalDir, tag, nil
	}

	dir, err := acquire(ctx, client, rc, opts, tag, finalDir)
	if err != nil {
		return "", "", err
	}
	log.Printf("%s/%s %s ready at %s", rc.Owner, rc.Repo, tag, dir)
	return dir, tag, nil
}

func acquire(ctx context.Context, client *http.Client, rc RepoConfig, opts Options, tag, finalDir string) (string, error) {
	switch opts.Method {
	case MethodTarball:
		return downloadExtractInstall(ctx, client, rc, opts, tag, finalDir)
	case MethodGit:
		return gitCloneInstall(ctx, rc, opts, tag, finalDir)
	case MethodAuto:
		dir, err := downloadExtractInstall(ctx, client, rc, opts, tag, finalDir)
		if err == nil {
			return dir, nil
		}
		if errors.Is(err, ErrVersionNotFound) || isCtxErr(err) {
			return "", err
		}
		log.Printf("Tarball download failed (%v); falling back to git clone", err)
		return gitCloneInstall(ctx, rc, opts, tag, finalDir)
	default:
		return "", fmt.Errorf("unknown download method %q (want %s|%s|%s)", opts.Method, MethodAuto, MethodTarball, MethodGit)
	}
}

func gitCloneInstall(ctx context.Context, rc RepoConfig, opts Options, tag, finalDir string) (string, error) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found on PATH (required for the git download method): %w", err)
	}

	tmpDir, err := os.MkdirTemp(opts.CacheDir, ".tmp-"+sanitizeTag(tag)+"-git-")
	if err != nil {
		return "", fmt.Errorf("create temp clone dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	gitURL := rc.gitURL()
	// core.longpaths is required on Windows for repos with deep paths
	// (nodejs, spring-framework, quarkus) which otherwise fail with
	// "Filename too long" during checkout.
	cmd := exec.CommandContext(ctx, gitBin,
		"-c", "core.longpaths=true",
		"clone", "--depth", "1", "--single-branch", "--branch", tag,
		gitURL, tmpDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Printf("Cloning %s/%s %s via git from %s", rc.Owner, rc.Repo, tag, gitURL)
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

	_ = os.RemoveAll(filepath.Join(tmpDir, ".git"))

	if !opts.Validate(tmpDir) {
		return "", fmt.Errorf("cloned repo failed validation: %w", ErrValidationFailed)
	}
	return installAtomically(tmpDir, finalDir, opts.Refresh)
}

func downloadExtractInstall(ctx context.Context, client *http.Client, rc RepoConfig, opts Options, tag, finalDir string) (string, error) {
	tmpDir, err := os.MkdirTemp(opts.CacheDir, ".tmp-"+sanitizeTag(tag)+"-")
	if err != nil {
		return "", fmt.Errorf("create temp extraction dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	resp, err := fetchTarball(ctx, client, rc, opts, tag)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := ExtractTarGz(ctx, resp.Body, tmpDir, opts.MaxExtractedB, defaultMaxEntries); err != nil {
		return "", err
	}
	if !opts.Validate(tmpDir) {
		return "", fmt.Errorf("extracted archive failed validation: %w", ErrValidationFailed)
	}

	return installAtomically(tmpDir, finalDir, opts.Refresh)
}

func fetchTarball(ctx context.Context, client *http.Client, rc RepoConfig, opts Options, tag string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s/archive/refs/tags/%s.tar.gz",
		strings.TrimRight(opts.TarballBaseURL, "/"), rc.Owner, rc.Repo, tag)

	resp, err := doGet(ctx, client, opts, url, false)
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

// resolveLatestTag determines the most recent stable version of a repository.
//
// It pools two candidate sources and picks the numerically-highest stable
// version among them:
//
//   - GET /releases/latest — GitHub's notion of the latest release. Useful
//     for recency, but cannot be trusted alone: repos that abandoned GitHub
//     releases return decade-old entries (meteor v0.5.9), and maintainers
//     sometimes mark prereleases as latest (remix@3.0.0-beta.4).
//   - The tags list. The GitHub tags API has no meaningful order, so the
//     "first tag" can be an ancient snapshot (weekly.2012-03-27 for
//     golang/go) — tags only contribute through version comparison.
//
// Options.TagFilter, when set, constrains every candidate to matching tags.
func resolveLatestTag(ctx context.Context, client *http.Client, rc RepoConfig, opts Options) (string, error) {
	releaseTag, err := fetchLatestRelease(ctx, client, rc, opts)
	if err != nil {
		return "", err
	}
	if releaseTag != "" && opts.TagFilter != nil && !opts.TagFilter(releaseTag) {
		releaseTag = ""
	}

	url := strings.TrimRight(opts.APIBaseURL, "/") + fmt.Sprintf("/repos/%s/%s/tags?per_page=100", rc.Owner, rc.Repo)

	candidates := make([]string, 0, 128)
	if releaseTag != "" {
		candidates = append(candidates, releaseTag)
	}
	for page := 0; url != "" && page < maxTagPages; page++ {
		tags, next, err := fetchTagPage(ctx, client, opts, url)
		if err != nil {
			// The release tag alone is still a usable answer.
			if releaseTag != "" {
				return releaseTag, nil
			}
			return "", err
		}
		for _, t := range tags {
			if opts.TagFilter != nil && !opts.TagFilter(t.Name) {
				continue
			}
			candidates = append(candidates, t.Name)
		}
		url = next
	}
	if len(candidates) == 0 {
		return "", errors.New("no matching tags found upstream")
	}
	return pickBestTag(candidates, releaseTag), nil
}

// stableTagRe matches plain release tags: "1.2", "v1.2.3", "v1.2.3.4".
var stableTagRe = regexp.MustCompile(`^v?\d+(\.\d+){1,3}$`)

// prefixedTagRe matches release tags with a project prefix: "go1.25.4",
// "zephyr-v3.5.0", "releases/gcc-15.1.0", "@sveltejs/kit@2.46.5". The
// numeric tail must run to the end of the tag so prerelease suffixes
// don't match, and the prefix must not contain digits so that the version
// captured is the tag's only number sequence — otherwise a tag like
// "release/METEOR@1.8.1-issue-10516.0" would rank by its issue number.
var prefixedTagRe = regexp.MustCompile(`^[@A-Za-z][A-Za-z._/@-]*?v?(\d+(\.\d+){1,3})$`)

// prereleaseMarkers are substrings that indicate a non-stable tag.
var prereleaseMarkers = []string{"alpha", "beta", "rc", "pre", "dev", "next", "canary", "snapshot", "milestone", "nightly", "-m"}

// pickBestTag selects the best release tag from an unordered candidate list:
// the numerically-highest version tag — plain (v1.2.3) or prefixed
// (go1.25.4, releases/gcc-15.1.0, @sveltejs/kit@2.46.5) — else the
// releases/latest tag, else the first tag without prerelease markers,
// else the first tag. Tags whose version cannot be parsed (which includes
// all prerelease-suffixed tags) never participate in version ranking.
func pickBestTag(tags []string, releaseTag string) string {
	best := ""
	var bestVer []int
	for _, t := range tags {
		// Prerelease-marked tags never participate in version ranking
		// (e.g. "release/template-engine-preview-release-10.1" must not
		// outrank "release/METEOR@3.4.2").
		if hasPrereleaseMarker(t) {
			continue
		}
		ver, ok := tagVersion(t)
		if !ok {
			continue
		}
		if best == "" || compareVersions(ver, bestVer) > 0 {
			best, bestVer = t, ver
		}
	}
	if best != "" {
		return best
	}
	if releaseTag != "" {
		return releaseTag
	}
	for _, t := range tags {
		if !hasPrereleaseMarker(t) {
			return t
		}
	}
	return tags[0]
}

// tagVersion extracts the comparable version tuple from a tag, accepting
// both plain (v1.2.3) and prefixed (go1.25.4) forms.
func tagVersion(tag string) ([]int, bool) {
	if stableTagRe.MatchString(tag) {
		return parseVersion(strings.TrimPrefix(tag, "v")), true
	}
	if m := prefixedTagRe.FindStringSubmatch(tag); m != nil {
		return parseVersion(m[1]), true
	}
	return nil, false
}

func hasPrereleaseMarker(tag string) bool {
	lower := strings.ToLower(tag)
	for _, m := range prereleaseMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// parseVersion splits "1.2.3" into [1 2 3]; unparseable parts become 0.
func parseVersion(s string) []int {
	parts := strings.Split(s, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out
}

// compareVersions compares two version tuples component-wise; missing
// components count as 0. Returns -1, 0, or 1.
func compareVersions(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

type ghRelease struct {
	TagName string `json:"tag_name"`
}

// fetchLatestRelease returns the tag of the repo's latest stable GitHub
// release, or "" when the repo has no releases (HTTP 404).
func fetchLatestRelease(ctx context.Context, client *http.Client, rc RepoConfig, opts Options) (string, error) {
	url := strings.TrimRight(opts.APIBaseURL, "/") + fmt.Sprintf("/repos/%s/%s/releases/latest", rc.Owner, rc.Repo)
	resp, err := doGet(ctx, client, opts, url, true)
	if err != nil {
		return "", fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusNotFound:
		return "", nil // repo publishes no releases; fall back to tags
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return "", fmt.Errorf("github api rate limit exhausted (resets at %s); set GITHUB_TOKEN to raise the limit: %w",
			formatRateLimitReset(resp.Header.Get("X-RateLimit-Reset")), ErrRateLimited)
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return "", fmt.Errorf("github releases api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return "", fmt.Errorf("decode latest release response: %w", err)
	}
	return rel.TagName, nil
}

type ghTag struct {
	Name string `json:"name"`
}

func fetchTagPage(ctx context.Context, client *http.Client, opts Options, url string) ([]ghTag, string, error) {
	resp, err := doGet(ctx, client, opts, url, true)
	if err != nil {
		return nil, "", fmt.Errorf("fetch tags: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
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

func doGet(ctx context.Context, client *http.Client, opts Options, url string, apiHeaders bool) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= opts.maxRetries(); attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("User-Agent", opts.UserAgent)
		if apiHeaders {
			req.Header.Set("Accept", "application/vnd.github+json")
			if opts.Token != "" {
				req.Header.Set("Authorization", "Bearer "+opts.Token)
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

		if attempt < opts.maxRetries() && !sleepBackoff(ctx, attempt) {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func ExtractTarGz(ctx context.Context, r io.Reader, destRoot string, maxBytes int64, maxEntries int) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
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
			return fmt.Errorf("read tar entry: %w", err)
		}

		entries++
		if entries > maxEntries {
			return fmt.Errorf("archive has more than %d entries: %w", maxEntries, ErrArchiveTooLarge)
		}

		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeDir:
		default:
			// Skip everything else (pax headers, symlinks, hardlinks,
			// devices). Symlinked files such as serde's LICENSE-APACHE
			// are never parsed as source code, and failing the whole
			// archive over them forces an unnecessary git fallback.
			continue
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
			return fmt.Errorf("archive has multiple top-level dirs (%q, %q): %w", topLevel, first, ErrUnexpectedLayout)
		}
		if rest == "" {
			continue
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
		return fmt.Errorf("archive is empty: %w", ErrUnexpectedLayout)
	}
	return nil
}

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

func installAtomically(tmpDir, finalDir string, refresh bool) (string, error) {
	if !refresh {
		if _, err := os.Stat(finalDir); err == nil {
			return finalDir, nil
		}
	} else if _, err := os.Stat(finalDir); err == nil {
		aside := finalDir + ".old-" + filepath.Base(tmpDir)
		if err := os.Rename(finalDir, aside); err != nil {
			return "", fmt.Errorf("rename existing cache aside: %w", err)
		}
		defer os.RemoveAll(aside)
	}

	if err := os.Rename(tmpDir, finalDir); err != nil {
		if !refresh {
			if _, statErr := os.Stat(finalDir); statErr == nil {
				return finalDir, nil
			}
		}
		return "", fmt.Errorf("install cache dir %s: %w", finalDir, err)
	}
	return finalDir, nil
}

func newClient() *http.Client {
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
