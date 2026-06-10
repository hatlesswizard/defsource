package golang

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("go/fasthttp", NewFastHTTPFactory)
	registry.Default.Register("go/gorilla-mux", NewGorillaMuxFactory)
}

// NewFastHTTPFactory creates a Source for the FastHTTP high-performance HTTP package.
func NewFastHTTPFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/fasthttp",
		Name:            "FastHTTP",
		Description:     "High-performance HTTP package for Go",
		SourceURL:       "https://github.com/valyala/fasthttp",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "valyala/fasthttp",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewGorillaMuxFactory creates a Source for the Gorilla Mux HTTP router.
func NewGorillaMuxFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/gorilla-mux",
		Name:            "Gorilla Mux",
		Description:     "HTTP router and URL matcher for Go",
		SourceURL:       "https://github.com/gorilla/mux",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "gorilla/mux",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}
