package rust

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("rust/loco", NewLocoFactory)
	registry.Default.Register("rust/ntex", NewNtexFactory)
	registry.Default.Register("rust/pavex", NewPavexFactory)
}

// NewLocoFactory creates a Source for the Loco Rails-like framework.
func NewLocoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "loco-rs",
		Repo:        "loco",
		CrateName:   "loco",
		CratePath:   "", // crate at repo root; the adapter appends src/ itself
		Version:     opts.Ref,
		Description: "Rails-like framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewNtexFactory creates a Source for the Ntex composable networking services framework.
func NewNtexFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "ntex-rs",
		Repo:        "ntex",
		CrateName:   "ntex",
		CratePath:   "ntex",
		Version:     opts.Ref,
		Description: "Composable networking services framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewPavexFactory creates a Source for the Pavex backend framework.
func NewPavexFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "LukeMathWalker",
		Repo:        "pavex",
		CrateName:   "pavex",
		// At the released tags the crate lives under runtime/pavex
		// (the development branch later moved it to libs/pavex).
		CratePath:   "runtime/pavex",
		Version:     opts.Ref,
		Description: "Backend framework with compile-time dependency injection for Rust",
	}
	return New(repoPath, cfg), nil
}
