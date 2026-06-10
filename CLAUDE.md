# defSource

## Build
- `CGO_ENABLED=1 go build -tags sqlite_fts5 ./...` — required for all builds (SQLite FTS5)
- `CGO_ENABLED=1 go test -race -tags sqlite_fts5 -v ./...` — run tests
- `CGO_ENABLED=1 go vet -tags sqlite_fts5 ./...` — lint
- `CGO_ENABLED=1 go mod tidy -tags sqlite_fts5` — fix go.mod indirect markers

## What This Is
- Go library + HTTP API + CLI crawler for source code documentation across 11 languages and 247 frameworks
- NOT an MCP server — users import the package or run the HTTP server
- Downloads source repos from GitHub (by version tag), parses source code locally via tree-sitter, and indexes documentation into SQLite+FTS5

## Supported Languages
Go, Python, Rust, Java, JavaScript, TypeScript, Ruby, C#, C, C++, PHP — each with 15-25 registered frameworks.

## Architecture
- `defsource.go` / `types.go` — public API (ResolveLibrary, QueryDocs, QueryDocsByLanguage, ListEntities, ListLanguages)
- `internal/source/` — pluggable source adapters per language:
  - `source.go` — Source interface + shared types (Entity, Method, DocSnippet, etc.)
  - `kinds.go` — canonical entity kind constants (class, struct, function, interface, etc.)
  - `registry/` — factory registry for dynamic source instantiation by language/framework
  - `allsources/` — side-effect import that registers all language adapters
  - `php/` — PHP parser (tree-sitter + PHPDoc): WordPress, Laravel, Symfony, etc.
  - `golang/` — Go parser (tree-sitter + godoc)
  - `python/` — Python parser (tree-sitter + pydoc)
  - `rust/` — Rust parser (tree-sitter + rustdoc)
  - `java/` — Java parser (tree-sitter + JavaDoc)
  - `javascript/` — JavaScript parser (tree-sitter + JSDoc)
  - `typescript/` — TypeScript parser (tree-sitter + JSDoc)
  - `ruby/` — Ruby parser (tree-sitter + YARD)
  - `csharp/` — C# parser (tree-sitter + XML doc)
  - `clang/` — C parser (tree-sitter + Doxygen)
  - `cpp/` — C++ parser (tree-sitter + Doxygen)
- `internal/docparser/` — unified doc comment parsing (9 formats: phpdoc, jsdoc, javadoc, godoc, rustdoc, pydoc, doxygen, xmldoc, yard)
- `internal/treesitter/` — sync.Pool of tree-sitter parsers for all 11 languages
- `internal/download/` — GitHub repo download with tarball/git fallback, atomic install, retry
- `internal/crawler/` — concurrent crawler with source-agnostic worker pool
- `internal/store/sqlite/` — SQLite+FTS5 storage with BM25 search, multi-language schema
- `internal/server/` — HTTP API server
- `internal/search/` — ranking, formatting, tokenizer

## Crawl Commands
- List all sources: `./bin/defsource-crawl --list-sources`
- Crawl single source: `./bin/defsource-crawl --source=python/django --db=./data/defsource.db`
- Crawl all frameworks for a language: `./bin/defsource-crawl --language=python --db=./data/defsource.db`
- Crawl everything: `./bin/defsource-crawl --all --yes --db=./data/defsource.db`
- WordPress (backward compat): `./bin/defsource-crawl --source=php/wordpress --db=./data/defsource.db`
- Resume interrupted: `./bin/defsource-crawl --source=python/django --resume`
- Retry failures: `./bin/defsource-crawl --source=python/django --retry-failed`
- Server: `./bin/defsource-server --db=./data/full-crawl.db --addr=:9890`
- Set `GITHUB_TOKEN`/`GH_TOKEN` to raise the GitHub tags API rate limit.
- `--download-method` (default `auto`) = tarball with `git clone --depth 1 --branch <tag>` fallback.

## HTTP API Endpoints
- `GET /api/v1/languages` — list languages with framework counts
- `GET /api/v1/libraries?language=python` — list libraries, optionally filtered by language
- `GET /api/v1/libraries/search?libraryName=...&query=...` — search libraries by name
- `GET /api/v1/docs?libraryId=...&query=...&mode=all&format=json` — query docs for a single library
- `GET /api/v1/docs/language?language=...&query=...&mode=all&format=json` — cross-library search within a language
- `GET /api/v1/entities?libraryId=...` — list entities for a library
- `GET /health` — health check

## Key Gotchas
- Tree-sitter grammar node types can change between grammar versions — always verify node.Type() values against the current grammar
- Properties use `INSERT OR REPLACE` (WordPress wpdb has duplicate property names)
- Wrapper detection: each language adapter has a builtins map to prevent wrapper chains from resolving into language built-ins
- FTS5 search uses AND by default; `mode=any` for OR semantics
- Store writes protected by sync.Mutex (concurrent crawler workers)
- Entity slugs derived from URL fragment (after `#`), NOT from the parsed entity name
- Doc comment pointers (`*docparser.DocComment`) can be nil — always nil-guard before accessing fields

## Common Mistakes to Avoid
- Do NOT treat defSource as an MCP server — it is a Go library with an HTTP API
- Do NOT propose React/frontend approaches — this is a pure Go backend project
- Do NOT omit `-tags sqlite_fts5` from any go command — builds will fail without it
- Do NOT hardcode language-specific logic in shared code (crawler, store) — use the Source interface
- Do NOT assume source repo layouts are standard — verify `SourceRoots`/`SourceDirs` against actual repo structure when adding frameworks
- When adding a new framework, test a real crawl (`--source=lang/name`) not just unit tests

## Architecture Decisions
- **SQLite with FTS5** (not Postgres, not ElasticSearch): Single-file deployment, no external dependencies, BM25 ranking built-in.
- **Pluggable Source interface**: Each language implements the same 8-method interface. New languages are added by creating a package under `internal/source/` and registering factories in `init()`.
- **Registry pattern**: Language adapters self-register via `init()` into `registry.Default`. The crawler discovers sources by ID or language prefix.
- **Tree-sitter parser pool**: `sync.Pool` per language avoids parser allocation overhead. All 11 grammars loaded at startup.
- **Unified doc parser**: 9 format-specific parsers behind a common `DocComment` struct. Shared across all language adapters.
- **3-level wrapper depth limit**: Wrapper functions can chain across languages. Limiting to 3 levels prevents infinite recursion.
- **Language-aware download validation**: After downloading a repo, the crawler validates it contains source files for the expected language extension.
