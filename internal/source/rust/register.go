package rust

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("rust/stdlib", NewStdlibFactory)
	registry.Default.Register("rust/tokio", NewTokioFactory)
	registry.Default.Register("rust/actix-web", NewActixWebFactory)
	registry.Default.Register("rust/serde", NewSerdeFactory)
	registry.Default.Register("rust/axum", NewAxumFactory)
	registry.Default.Register("rust/diesel", NewDieselFactory)
	registry.Default.Register("rust/clap", NewClapFactory)
	registry.Default.Register("rust/reqwest", NewReqwestFactory)
	registry.Default.Register("rust/sqlx", NewSQLxFactory)
	registry.Default.Register("rust/rayon", NewRayonFactory)
	registry.Default.Register("rust/tracing", NewTracingFactory)
	registry.Default.Register("rust/rocket", NewRocketFactory)
	registry.Default.Register("rust/hyper", NewHyperFactory)
	registry.Default.Register("rust/warp", NewWarpFactory)
	registry.Default.Register("rust/tide", NewTideFactory)
	registry.Default.Register("rust/poem", NewPoemFactory)
	registry.Default.Register("rust/leptos", NewLeptosFactory)
	registry.Default.Register("rust/yew", NewYewFactory)
	registry.Default.Register("rust/tauri", NewTauriFactory)
	registry.Default.Register("rust/dioxus", NewDioxusFactory)
	registry.Default.Register("rust/gotham", NewGothamFactory)
	registry.Default.Register("rust/salvo", NewSalvoFactory)
}

// NewStdlibFactory creates a Source for the Rust standard library.
func NewStdlibFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "rust-lang",
		Repo:        "rust",
		CrateName:   "std",
		CratePath:   "library/std",
		Version:     opts.Ref,
		Description: "The Rust standard library",
	}
	return New(repoPath, cfg), nil
}

// NewTokioFactory creates a Source for the Tokio async runtime.
func NewTokioFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "tokio-rs",
		Repo:        "tokio",
		CrateName:   "tokio",
		CratePath:   "tokio",
		Version:     opts.Ref,
		Description: "Asynchronous runtime for Rust providing I/O, networking, and scheduling",
	}
	return New(repoPath, cfg), nil
}

// NewActixWebFactory creates a Source for the Actix-web framework.
func NewActixWebFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "actix",
		Repo:        "actix-web",
		CrateName:   "actix-web",
		CratePath:   "actix-web",
		Version:     opts.Ref,
		Description: "Powerful, pragmatic, and fast web framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewSerdeFactory creates a Source for the Serde serialization library.
func NewSerdeFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "serde-rs",
		Repo:        "serde",
		CrateName:   "serde",
		CratePath:   "serde",
		Version:     opts.Ref,
		Description: "Serialization framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewAxumFactory creates a Source for the Axum web framework.
func NewAxumFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "tokio-rs",
		Repo:        "axum",
		CrateName:   "axum",
		CratePath:   "axum",
		Version:     opts.Ref,
		Description: "Ergonomic and modular web framework built with Tokio and Hyper",
	}
	return New(repoPath, cfg), nil
}

// NewDieselFactory creates a Source for the Diesel ORM.
func NewDieselFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "diesel-rs",
		Repo:        "diesel",
		CrateName:   "diesel",
		CratePath:   "diesel",
		Version:     opts.Ref,
		Description: "Safe, extensible ORM and query builder for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewClapFactory creates a Source for the Clap argument parser.
func NewClapFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "clap-rs",
		Repo:        "clap",
		CrateName:   "clap",
		CratePath:   "clap_builder",
		Version:     opts.Ref,
		Description: "Command Line Argument Parser for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewReqwestFactory creates a Source for the Reqwest HTTP client.
func NewReqwestFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "seanmonstar",
		Repo:        "reqwest",
		CrateName:   "reqwest",
		CratePath:   "",
		Version:     opts.Ref,
		Description: "Ergonomic, batteries-included HTTP client for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewSQLxFactory creates a Source for the SQLx async database library.
func NewSQLxFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "launchbadge",
		Repo:        "sqlx",
		CrateName:   "sqlx",
		CratePath:   "sqlx-core",
		Version:     opts.Ref,
		Description: "Async SQL toolkit with compile-time checked queries",
	}
	return New(repoPath, cfg), nil
}

// NewRayonFactory creates a Source for the Rayon data parallelism library.
func NewRayonFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "rayon-rs",
		Repo:        "rayon",
		CrateName:   "rayon",
		CratePath:   "", // rayon's main crate lives at the repo root (rayon-core is a subcrate)
		Version:     opts.Ref,
		Description: "Data parallelism library for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewTracingFactory creates a Source for the Tracing instrumentation library.
func NewTracingFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "tokio-rs",
		Repo:        "tracing",
		CrateName:   "tracing",
		CratePath:   "tracing",
		Version:     opts.Ref,
		Description: "Application-level tracing for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewRocketFactory creates a Source for the Rocket web framework.
func NewRocketFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "rwf2",
		Repo:        "Rocket",
		CrateName:   "rocket",
		CratePath:   "core/lib",
		Version:     opts.Ref,
		Description: "Web framework for Rust with focus on usability and security",
	}
	return New(repoPath, cfg), nil
}

// NewHyperFactory creates a Source for the Hyper HTTP library.
func NewHyperFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "hyperium",
		Repo:        "hyper",
		CrateName:   "hyper",
		CratePath:   "",
		Version:     opts.Ref,
		Description: "Fast and correct HTTP implementation for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewWarpFactory creates a Source for the Warp web framework.
func NewWarpFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "seanmonstar",
		Repo:        "warp",
		CrateName:   "warp",
		CratePath:   "", // crate at repo root; the adapter appends src/ itself
		Version:     opts.Ref,
		Description: "Composable web framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewTideFactory creates a Source for the Tide async web framework.
func NewTideFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "http-rs",
		Repo:        "tide",
		CrateName:   "tide",
		CratePath:   "", // crate at repo root; the adapter appends src/ itself
		Version:     opts.Ref,
		Description: "Async web framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewPoemFactory creates a Source for the Poem web framework.
func NewPoemFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "poem-web",
		Repo:        "poem",
		CrateName:   "poem",
		CratePath:   "poem",
		Version:     opts.Ref,
		Description: "Web framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewLeptosFactory creates a Source for the Leptos full-stack reactive framework.
func NewLeptosFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "leptos-rs",
		Repo:        "leptos",
		CrateName:   "leptos",
		CratePath:   "leptos",
		Version:     opts.Ref,
		Description: "Full-stack reactive framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewYewFactory creates a Source for the Yew frontend WASM framework.
func NewYewFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "yewstack",
		Repo:        "yew",
		CrateName:   "yew",
		CratePath:   "packages/yew",
		Version:     opts.Ref,
		Description: "Frontend WASM framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewTauriFactory creates a Source for the Tauri desktop app framework.
func NewTauriFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "tauri-apps",
		Repo:        "tauri",
		CrateName:   "tauri",
		CratePath:   "crates/tauri",
		Version:     opts.Ref,
		Description: "Desktop app framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewDioxusFactory creates a Source for the Dioxus cross-platform UI framework.
func NewDioxusFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "DioxusLabs",
		Repo:        "dioxus",
		CrateName:   "dioxus",
		CratePath:   "packages/dioxus",
		Version:     opts.Ref,
		Description: "Cross-platform UI framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewGothamFactory creates a Source for the Gotham web framework.
func NewGothamFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "gotham-rs",
		Repo:        "gotham",
		CrateName:   "gotham",
		CratePath:   "gotham",
		Version:     opts.Ref,
		Description: "Web framework for Rust",
	}
	return New(repoPath, cfg), nil
}

// NewSalvoFactory creates a Source for the Salvo web framework.
func NewSalvoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "salvo-rs",
		Repo:        "salvo",
		CrateName:   "salvo",
		CratePath:   "crates/core", // salvo-core package lives in crates/core
		Version:     opts.Ref,
		Description: "Web framework for Rust",
	}
	return New(repoPath, cfg), nil
}
