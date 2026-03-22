package wordpress

import "github.com/hatlesswizard/defsource/internal/source"

const (
	BaseURL   = "https://developer.wordpress.org"
	LibraryID = "/wordpress/classes"
)

// Compile-time interface check.
var _ source.Source = (*WordPressSource)(nil)

type WordPressSource struct{}

func New() *WordPressSource {
	return &WordPressSource{}
}

func (w *WordPressSource) ID() string { return LibraryID }

func (w *WordPressSource) Meta() source.LibraryMeta {
	return source.LibraryMeta{
		Name:        "WordPress Classes Reference",
		Description: "Complete reference for all WordPress PHP classes including properties, methods, and source code",
		SourceURL:   BaseURL + "/reference/classes/",
		Version:     "6.7",
		TrustScore:  0.9,
	}
}
