package typescript

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("typescript/strapi", NewStrapiFactory)
	registry.Default.Register("typescript/nitro", NewNitroFactory)
	registry.Default.Register("typescript/solid", NewSolidFactory)
	// The following products are written in TypeScript even though they are
	// commonly thought of as JavaScript frameworks; the JavaScript adapter
	// (which only parses .js) cannot index them.
	registry.Default.Register("typescript/jest", NewJestFactory)
	registry.Default.Register("typescript/nextjs", NewNextJSFactory)
	registry.Default.Register("typescript/socket-io", NewSocketIOFactory)
	registry.Default.Register("typescript/adonis", NewAdonisFactory)
	registry.Default.Register("typescript/nuxt", NewNuxtFactory)
	registry.Default.Register("typescript/electron", NewElectronFactory)
}

// NewStrapiFactory creates a Source for the Strapi headless CMS.
func NewStrapiFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/strapi",
		Name:        "Strapi",
		Description: "Headless CMS framework for TypeScript",
		SourceURL:   "https://github.com/strapi/strapi",
		Owner:       "strapi",
		Repo:        "strapi",
		SourceDirs:  []string{"packages/core/strapi/src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewNitroFactory creates a Source for the Nitro server toolkit.
func NewNitroFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/nitro",
		Name:        "Nitro",
		Description: "Server toolkit powering Nuxt",
		SourceURL:   "https://github.com/nitrojs/nitro",
		Owner:       "nitrojs",
		Repo:        "nitro",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewSolidFactory creates a Source for the SolidJS reactive UI library.
// Replaces Wasp, whose compiler is written in Haskell and cannot be
// indexed by the TypeScript adapter.
func NewSolidFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/solid",
		Name:        "SolidJS",
		Description: "Simple and performant reactivity library for building user interfaces",
		SourceURL:   "https://github.com/solidjs/solid",
		Owner:       "solidjs",
		Repo:        "solid",
		SourceDirs:  []string{"packages/solid/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewJestFactory creates a Source for the Jest testing framework.
func NewJestFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/jest",
		Name:        "Jest",
		Description: "Delightful JavaScript testing framework with a focus on simplicity",
		SourceURL:   "https://github.com/jestjs/jest",
		Owner:       "jestjs",
		Repo:        "jest",
		SourceDirs:  []string{"packages/jest-core/src", "packages/expect/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewNextJSFactory creates a Source for the Next.js framework.
func NewNextJSFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/nextjs",
		Name:        "Next.js",
		Description: "React framework for production with hybrid static and server rendering",
		SourceURL:   "https://github.com/vercel/next.js",
		Owner:       "vercel",
		Repo:        "next.js",
		SourceDirs:  []string{"packages/next/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewSocketIOFactory creates a Source for the Socket.IO library.
func NewSocketIOFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/socket-io",
		Name:        "Socket.IO",
		Description: "Bidirectional and low-latency communication for real-time applications",
		SourceURL:   "https://github.com/socketio/socket.io",
		Owner:       "socketio",
		Repo:        "socket.io",
		SourceDirs:  []string{"packages/socket.io/lib"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewAdonisFactory creates a Source for the AdonisJS framework.
func NewAdonisFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/adonis",
		Name:        "AdonisJS",
		Description: "Full-stack MVC framework for Node.js",
		SourceURL:   "https://github.com/adonisjs/core",
		Owner:       "adonisjs",
		Repo:        "core",
		SourceDirs:  []string{"src", "providers"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewNuxtFactory creates a Source for the Nuxt framework.
func NewNuxtFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/nuxt",
		Name:        "Nuxt",
		Description: "Vue SSR framework",
		SourceURL:   "https://github.com/nuxt/nuxt",
		Owner:       "nuxt",
		Repo:        "nuxt",
		SourceDirs:  []string{"packages/nuxt/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewElectronFactory creates a Source for the Electron framework.
func NewElectronFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/electron",
		Name:        "Electron",
		Description: "Desktop app framework using web technologies",
		SourceURL:   "https://github.com/electron/electron",
		Owner:       "electron",
		Repo:        "electron",
		SourceDirs:  []string{"lib"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}
