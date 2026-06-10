package javascript

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("javascript/moleculer", NewMoleculerFactory)
}

// NewMoleculerFactory creates a Source for the Moleculer microservices framework.
func NewMoleculerFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/moleculer",
		Name:        "Moleculer",
		Description: "Progressive microservices framework for Node.js",
		SourceURL:   "https://github.com/moleculerjs/moleculer",
		SourceDirs:  []string{"src"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}
