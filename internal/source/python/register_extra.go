package python

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("python/reflex", NewReflexFactory)
	registry.Default.Register("python/masonite", NewMasoniteFactory)
}

// NewReflexFactory creates a Source for the Reflex full-stack web framework.
func NewReflexFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/reflex",
		Name:        "Reflex",
		Description: "Full-stack web apps in pure Python",
		SourceURL:   "https://github.com/reflex-dev/reflex",
		SourceRoots: []string{"reflex/"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewMasoniteFactory creates a Source for the Masonite framework.
func NewMasoniteFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/masonite",
		Name:        "Masonite",
		Description: "Laravel-like Python web framework",
		SourceURL:   "https://github.com/MasoniteFramework/masonite",
		SourceRoots: []string{"src/masonite/"},
		TrustScore:  0.85,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}
