package typescript

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("typescript/stdlib", NewStdlibFactory)
	registry.Default.Register("typescript/angular", NewAngularFactory)
	registry.Default.Register("typescript/nestjs", NewNestJSFactory)
	registry.Default.Register("typescript/rxjs", NewRxJSFactory)
	registry.Default.Register("typescript/typeorm", NewTypeORMFactory)
	registry.Default.Register("typescript/prisma", NewPrismaFactory)
	registry.Default.Register("typescript/zod", NewZodFactory)
	registry.Default.Register("typescript/trpc", NewTRPCFactory)
	registry.Default.Register("typescript/vitest", NewVitestFactory)
	registry.Default.Register("typescript/drizzle", NewDrizzleFactory)
	registry.Default.Register("typescript/effect", NewEffectFactory)
	registry.Default.Register("typescript/date-fns", NewDateFnsFactory)
	registry.Default.Register("typescript/astro", NewAstroFactory)
	registry.Default.Register("typescript/hono", NewHonoFactory)
	registry.Default.Register("typescript/elysia", NewElysiaFactory)
	registry.Default.Register("typescript/fresh", NewFreshFactory)
	registry.Default.Register("typescript/blitz", NewBlitzFactory)
	registry.Default.Register("typescript/feathers", NewFeathersFactory)
	registry.Default.Register("typescript/foal", NewFoalFactory)
	registry.Default.Register("typescript/analog", NewAnalogFactory)
}

// NewStdlibFactory creates a Source for the TypeScript standard type definitions.
func NewStdlibFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:              "typescript/stdlib",
		Name:                   "TypeScript Standard Library",
		Description:            "Built-in type definitions for TypeScript and JavaScript",
		SourceURL:              "https://github.com/microsoft/TypeScript",
		Owner:                  "microsoft",
		Repo:                   "TypeScript",
		// lib/ holds the complete built type definitions (lib.dom.d.ts,
		// lib.es2015.*.d.ts, ...); src/lib only has unbuilt fragments.
		SourceDirs:             []string{"lib"},
		IncludeDeclarationFiles: true,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewAngularFactory creates a Source for the Angular framework.
func NewAngularFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/angular",
		Name:        "Angular",
		Description: "Platform for building mobile and desktop web applications",
		SourceURL:   "https://github.com/angular/angular",
		Owner:       "angular",
		Repo:        "angular",
		SourceDirs:  []string{"packages/core/src", "packages/common/src", "packages/router/src"},
		ExcludeDirs: []string{"testing", "schematics"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewNestJSFactory creates a Source for the NestJS framework.
func NewNestJSFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/nestjs",
		Name:        "NestJS",
		Description: "Progressive Node.js framework for building scalable server-side applications",
		SourceURL:   "https://github.com/nestjs/nest",
		Owner:       "nestjs",
		Repo:        "nest",
		// Nest packages keep their sources directly in the package root,
		// not under a src/ subdirectory.
		SourceDirs:  []string{"packages/core", "packages/common"},
		ExcludeDirs: []string{"test"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewRxJSFactory creates a Source for the RxJS reactive extensions library.
func NewRxJSFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/rxjs",
		Name:        "RxJS",
		Description: "Reactive Extensions Library for JavaScript using Observables",
		SourceURL:   "https://github.com/ReactiveX/rxjs",
		Owner:       "ReactiveX",
		Repo:        "rxjs",
		SourceDirs:  []string{"src/internal"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewTypeORMFactory creates a Source for the TypeORM database framework.
func NewTypeORMFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/typeorm",
		Name:        "TypeORM",
		Description: "ORM for TypeScript and JavaScript supporting multiple databases",
		SourceURL:   "https://github.com/typeorm/typeorm",
		Owner:       "typeorm",
		Repo:        "typeorm",
		SourceDirs:  []string{"src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewPrismaFactory creates a Source for the Prisma ORM.
func NewPrismaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/prisma",
		Name:        "Prisma",
		Description: "Next-generation ORM for Node.js and TypeScript",
		SourceURL:   "https://github.com/prisma/prisma",
		Owner:       "prisma",
		Repo:        "prisma",
		SourceDirs:  []string{"packages/client/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewZodFactory creates a Source for the Zod schema validation library.
func NewZodFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/zod",
		Name:        "Zod",
		Description: "TypeScript-first schema validation with static type inference",
		SourceURL:   "https://github.com/colinhacks/zod",
		Owner:       "colinhacks",
		Repo:        "zod",
		SourceDirs:  []string{"packages/zod/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewTRPCFactory creates a Source for the tRPC framework.
func NewTRPCFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/trpc",
		Name:        "tRPC",
		Description: "End-to-end typesafe APIs for TypeScript",
		SourceURL:   "https://github.com/trpc/trpc",
		Owner:       "trpc",
		Repo:        "trpc",
		SourceDirs:  []string{"packages/server/src", "packages/client/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewVitestFactory creates a Source for the Vitest testing framework.
func NewVitestFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/vitest",
		Name:        "Vitest",
		Description: "Blazing fast unit testing framework powered by Vite",
		SourceURL:   "https://github.com/vitest-dev/vitest",
		Owner:       "vitest-dev",
		Repo:        "vitest",
		SourceDirs:  []string{"packages/vitest/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewDrizzleFactory creates a Source for the Drizzle ORM.
func NewDrizzleFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/drizzle",
		Name:        "Drizzle",
		Description: "Headless TypeScript ORM with maximum type safety",
		SourceURL:   "https://github.com/drizzle-team/drizzle-orm",
		Owner:       "drizzle-team",
		Repo:        "drizzle-orm",
		SourceDirs:  []string{"drizzle-orm/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewEffectFactory creates a Source for the Effect library.
func NewEffectFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/effect",
		Name:        "Effect",
		Description: "Powerful TypeScript library for building complex synchronous and asynchronous programs",
		SourceURL:   "https://github.com/Effect-TS/effect",
		Owner:       "Effect-TS",
		Repo:        "effect",
		SourceDirs:  []string{"packages/effect/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewDateFnsFactory creates a Source for the date-fns library.
func NewDateFnsFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/date-fns",
		Name:        "date-fns",
		Description: "Modern JavaScript date utility library with comprehensive toolset",
		SourceURL:   "https://github.com/date-fns/date-fns",
		Owner:       "date-fns",
		Repo:        "date-fns",
		// v4.1 and earlier keep functions in src/; later tags moved to a
		// pkgs/ workspace. Missing roots are skipped with a warning.
		SourceDirs:  []string{"src", "pkgs/core/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewAstroFactory creates a Source for the Astro framework.
func NewAstroFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/astro",
		Name:        "Astro",
		Description: "Content web framework",
		SourceURL:   "https://github.com/withastro/astro",
		Owner:       "withastro",
		Repo:        "astro",
		SourceDirs:  []string{"packages/astro/src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewHonoFactory creates a Source for the Hono web framework.
func NewHonoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/hono",
		Name:        "Hono",
		Description: "Ultra-fast web framework",
		SourceURL:   "https://github.com/honojs/hono",
		Owner:       "honojs",
		Repo:        "hono",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewElysiaFactory creates a Source for the Elysia framework.
func NewElysiaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/elysia",
		Name:        "Elysia",
		Description: "Bun web framework",
		SourceURL:   "https://github.com/elysiajs/elysia",
		Owner:       "elysiajs",
		Repo:        "elysia",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewFreshFactory creates a Source for the Fresh framework.
func NewFreshFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/fresh",
		Name:        "Fresh",
		Description: "Deno web framework",
		SourceURL:   "https://github.com/denoland/fresh",
		Owner:       "denoland",
		Repo:        "fresh",
		// Fresh 2.x is a workspace; the framework lives in packages/fresh.
		SourceDirs:  []string{"packages/fresh/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewBlitzFactory creates a Source for the Blitz.js framework.
func NewBlitzFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/blitz",
		Name:        "Blitz.js",
		Description: "Full-stack toolkit",
		SourceURL:   "https://github.com/blitz-js/blitz",
		Owner:       "blitz-js",
		Repo:        "blitz",
		SourceDirs:  []string{"packages/blitz/src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewFeathersFactory creates a Source for the Feathers framework.
func NewFeathersFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/feathers",
		Name:        "Feathers",
		Description: "REST/real-time framework",
		SourceURL:   "https://github.com/feathersjs/feathers",
		Owner:       "feathersjs",
		Repo:        "feathers",
		SourceDirs:  []string{"packages/feathers/src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewFoalFactory creates a Source for the FoalTS framework.
func NewFoalFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/foal",
		Name:        "FoalTS",
		Description: "Full-stack Node framework",
		SourceURL:   "https://github.com/FoalTS/foal",
		Owner:       "FoalTS",
		Repo:        "foal",
		SourceDirs:  []string{"packages/core/src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}

// NewAnalogFactory creates a Source for the Analog framework.
func NewAnalogFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "typescript/analog",
		Name:        "Analog",
		Description: "Angular meta-framework",
		SourceURL:   "https://github.com/analogjs/analog",
		Owner:       "analogjs",
		Repo:        "analog",
		// There is no packages/analog package; the framework lives in
		// the platform, router, and content packages.
		SourceDirs:  []string{"packages/platform/src", "packages/router/src", "packages/content/src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(cfg, repoPath, sopts...), nil
}
