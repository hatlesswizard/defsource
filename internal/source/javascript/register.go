package javascript

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

// NOTE: Jest, Next.js, Socket.IO, AdonisJS, Nuxt, and Electron are written in
// TypeScript and are registered by the typescript package instead — the
// JavaScript adapter only parses .js sources.
func init() {
	registry.Default.Register("javascript/nodejs", NewNodeJSFactory)
	registry.Default.Register("javascript/react", NewReactFactory)
	registry.Default.Register("javascript/express", NewExpressFactory)
	registry.Default.Register("javascript/lodash", NewLodashFactory)
	registry.Default.Register("javascript/axios", NewAxiosFactory)
	registry.Default.Register("javascript/d3", NewD3Factory)
	registry.Default.Register("javascript/fastify", NewFastifyFactory)
	registry.Default.Register("javascript/mocha", NewMochaFactory)
	registry.Default.Register("javascript/webpack", NewWebpackFactory)
	registry.Default.Register("javascript/koa", NewKoaFactory)
	registry.Default.Register("javascript/hapi", NewHapiFactory)
	registry.Default.Register("javascript/sails", NewSailsFactory)
	registry.Default.Register("javascript/meteor", NewMeteorFactory)
	registry.Default.Register("javascript/remix", NewRemixFactory)
	registry.Default.Register("javascript/restify", NewRestifyFactory)
	registry.Default.Register("javascript/ws", NewWSFactory)
	// SvelteKit ships JavaScript sources with JSDoc type annotations,
	// so it is indexed by the JavaScript adapter despite its TypeScript
	// developer experience.
	registry.Default.Register("javascript/sveltekit", NewSvelteKitFactory)
}

// NewNodeJSFactory creates a Source for the Node.js standard library.
func NewNodeJSFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/nodejs",
		Name:        "Node.js",
		Description: "Server-side JavaScript runtime built on V8",
		SourceURL:   "https://github.com/nodejs/node",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewReactFactory creates a Source for the React UI library.
func NewReactFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/react",
		Name:        "React",
		Description: "Declarative UI library for building user interfaces",
		SourceURL:   "https://github.com/facebook/react",
		SourceDirs:  []string{"packages/react/src", "packages/react-dom/src"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewExpressFactory creates a Source for the Express.js web framework.
func NewExpressFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/express",
		Name:        "Express",
		Description: "Fast, unopinionated, minimalist web framework for Node.js",
		SourceURL:   "https://github.com/expressjs/express",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewLodashFactory creates a Source for the Lodash utility library.
func NewLodashFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/lodash",
		Name:        "Lodash",
		Description: "Modern JavaScript utility library delivering modularity and performance",
		SourceURL:   "https://github.com/lodash/lodash",
		// At release tags (4.17.x) the per-method modules live at the
		// repo root; an empty SourceDirs scans the root directory.
		SourceDirs: nil,
		BlobRef:    opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewAxiosFactory creates a Source for the Axios HTTP client.
func NewAxiosFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/axios",
		Name:        "Axios",
		Description: "Promise-based HTTP client for the browser and Node.js",
		SourceURL:   "https://github.com/axios/axios",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewD3Factory creates a Source for the D3.js visualization library.
// The d3/d3 umbrella repo only re-exports submodules, so this indexes the
// d3-selection core module, which carries the canonical DOM-selection API.
func NewD3Factory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/d3",
		Name:        "D3.js (d3-selection)",
		Description: "Data-Driven Documents: core DOM selection and data-join module",
		SourceURL:   "https://github.com/d3/d3-selection",
		SourceDirs:  []string{"src"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewFastifyFactory creates a Source for the Fastify web framework.
func NewFastifyFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/fastify",
		Name:        "Fastify",
		Description: "Fast and low overhead web framework for Node.js",
		SourceURL:   "https://github.com/fastify/fastify",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewMochaFactory creates a Source for the Mocha testing framework.
func NewMochaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/mocha",
		Name:        "Mocha",
		Description: "Feature-rich JavaScript test framework running on Node.js",
		SourceURL:   "https://github.com/mochajs/mocha",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewWebpackFactory creates a Source for the Webpack module bundler.
func NewWebpackFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/webpack",
		Name:        "Webpack",
		Description: "Static module bundler for modern JavaScript applications",
		SourceURL:   "https://github.com/webpack/webpack",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewKoaFactory creates a Source for the Koa web framework.
func NewKoaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/koa",
		Name:        "Koa",
		Description: "Next-gen Express web framework for Node.js",
		SourceURL:   "https://github.com/koajs/koa",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewHapiFactory creates a Source for the Hapi web framework.
func NewHapiFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/hapi",
		Name:        "Hapi",
		Description: "Enterprise web framework for Node.js",
		SourceURL:   "https://github.com/hapijs/hapi",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewSailsFactory creates a Source for the Sails.js MVC framework.
func NewSailsFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/sails",
		Name:        "Sails.js",
		Description: "MVC framework for Node.js",
		SourceURL:   "https://github.com/balderdashy/sails",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewMeteorFactory creates a Source for the Meteor framework.
func NewMeteorFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/meteor",
		Name:        "Meteor",
		Description: "Full-stack real-time framework for JavaScript",
		SourceURL:   "https://github.com/meteor/meteor",
		SourceDirs:  []string{"packages/"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewRemixFactory creates a Source for the Remix framework.
func NewRemixFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/remix",
		Name:        "Remix",
		Description: "Full-stack React framework",
		SourceURL:   "https://github.com/remix-run/remix",
		SourceDirs:  []string{"packages/remix-server-runtime/"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewRestifyFactory creates a Source for the Restify framework.
func NewRestifyFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/restify",
		Name:        "Restify",
		Description: "REST framework for Node.js",
		SourceURL:   "https://github.com/restify/node-restify",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewWSFactory creates a Source for the ws WebSocket library.
// Replaces Polka, whose single-file source carries no JSDoc to index.
func NewWSFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/ws",
		Name:        "ws",
		Description: "Fast, well-documented WebSocket client and server for Node.js",
		SourceURL:   "https://github.com/websockets/ws",
		SourceDirs:  []string{"lib"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}

// NewSvelteKitFactory creates a Source for the SvelteKit framework.
func NewSvelteKitFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := LibraryConfig{
		ID:          "javascript/sveltekit",
		Name:        "SvelteKit",
		Description: "Svelte app framework",
		SourceURL:   "https://github.com/sveltejs/kit",
		SourceDirs:  []string{"packages/kit/src"},
		BlobRef:     opts.Ref,
	}
	return New(repoPath, WithConfig(cfg)), nil
}
