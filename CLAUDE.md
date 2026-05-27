# defSource

## Build
- `CGO_ENABLED=1 go build -tags sqlite_fts5 ./...` — required for all builds (SQLite FTS5)
- `CGO_ENABLED=1 go test -race -tags sqlite_fts5 -v ./...` — run tests
- `CGO_ENABLED=1 go vet -tags sqlite_fts5 ./...` — lint
- `CGO_ENABLED=1 go mod tidy -tags sqlite_fts5` — fix go.mod indirect markers

## What This Is
- Go library + HTTP API + CLI crawler for WordPress class documentation
- NOT an MCP server — users import the package or run the HTTP server
- Downloads the WordPress source from https://github.com/WordPress/WordPress (by version) and parses the PHP locally

## Architecture
- `defsource.go` / `types.go` — public API (ResolveLibrary, QueryDocs, ListEntities)
- `internal/source/wpgithub/` — PHP source parser:
  - `wpgithub.go` — Source interface implementation (ParseEntity, ParseMethod, DetectWrapper, ResolveWrapperURL, ParseSourceCode, wrapper chain resolution, PHP builtins map)
  - `parser.go` — tree-sitter AST walker (parseFile, walkNode, extractClass/Function/Method/Properties, detectWrapperAST) + PHPDoc regex parser (parsePhpDoc, findPrecedingDoc)
  - `discovery.go` — filesystem walk + codebase index (buildCodebaseIndex, entity priority by RefCount)
- `internal/crawler/` — concurrent crawler with rate-limited fetcher (ticker-based)
- `internal/store/sqlite/` — SQLite+FTS5 storage with BM25 search
- `internal/server/` — HTTP API server
- `internal/search/` — ranking, formatting, tokenizer

## Crawl Commands
- The crawler downloads `WordPress/WordPress` from GitHub itself (no manual clone, no `--repo-path`). Empty `--version` resolves the latest release; downloads are cached per-version under `--cache-dir`.
- Latest crawl: `./bin/defsource-crawl --source=wpgithub --db=./data/defsource.db`
- Specific version: `./bin/defsource-crawl --source=wpgithub --version=6.5.3 --db=./data/defsource.db`
- Force re-download: `./bin/defsource-crawl --source=wpgithub --version=6.5.3 --refresh`
- Resume interrupted (pins to stored version): `./bin/defsource-crawl --source=wpgithub --resume`
- Retry failures: `./bin/defsource-crawl --source=wpgithub --retry-failed`
- Server: `./bin/defsource-server --db=./data/full-crawl.db --addr=:9890`
- Set `GITHUB_TOKEN`/`GH_TOKEN` to raise the GitHub tags API rate limit used for latest-version resolution.
- `--download-method` (default `auto`) = tarball with `git clone --depth 1 --branch <tag>` fallback. Use `git` to skip the tarball (needed where `codeload.github.com` is blocked but `github.com` git access works); `tarball` to force HTTP-only. Auto only falls back on non-404, non-cancel errors.

## Key Gotchas
- PHP tree-sitter grammar node types can change between grammar versions — always verify node.Type() values in parser.go against the current `github.com/smacker/go-tree-sitter/php` grammar
- Properties use `INSERT OR REPLACE` (wpdb has duplicate property names)
- Wrapper detection: PHP builtins map in wpgithub.go (`phpBuiltins` var, 120+ entries) prevents wrapper chains from resolving into PHP built-ins
- FTS5 search uses AND by default; `mode=any` for OR semantics
- Fetcher uses time.Ticker for rate limiting (goroutine-safe, no mutex needed)
- Store writes protected by sync.Mutex (concurrent crawler workers)
- Entity slugs derived from URL fragment (after `#`), NOT from the parsed entity name (prevents collisions)
- The local fetcher (NewLocalFetcher) has NO rate limiting — workers can hammer the store at maximum speed; sqlite.mu is the sole bottleneck

## Common Mistakes to Avoid
- Do NOT treat defSource as an MCP server — it is a Go library with an HTTP API
- Do NOT propose React/frontend approaches — this is a pure Go backend project
- Do NOT hardcode workarounds for parsing issues — find and fix the root cause (debug tree-sitter node type names in parser.go by logging node.Type() values against actual PHP fixtures)
- Do NOT recommend skills/tools without verifying they exist in the current project
- Do NOT use `go build -o patchleaks` — this is defSource, not PatchLeaks; use `CGO_ENABLED=1 go build -tags sqlite_fts5 ./...`
- Do NOT omit `-tags sqlite_fts5` from any go command — builds will fail without it
- When changing configuration values, check BOTH code defaults AND config files
- Do NOT assume PHP tree-sitter grammar node types are stable — always verify against the current grammar version before modifying AST walking code

## Architecture Decisions
- **SQLite with FTS5** (not Postgres, not ElasticSearch): Single-file deployment, no external dependencies, BM25 ranking built-in. FTS5 provides full-text search with relevance scoring adequate for documentation queries.
- **time.Ticker for rate limiting**: Goroutine-safe without mutex. The ticker fires at a fixed interval; each fetch waits for a tick. Simpler and more correct than token-bucket or mutex-based approaches.
- **Single-instance crawler**: No distributed state needed. WordPress.org rate limits make parallel instances counterproductive. One process with concurrent workers and a shared ticker is optimal.
- **INSERT OR REPLACE for properties**: WordPress classes like `wpdb` have duplicate property names (e.g., `$col_meta` appears multiple times). INSERT OR REPLACE handles this without error.
- **3-level wrapper depth limit**: PHP wrapper functions can chain (e.g., `wp_cache_get` -> `WP_Object_Cache::get` -> underlying implementation). Limiting to 3 levels prevents infinite recursion while catching real wrappers.
- **Entity slugs from URL path**: Deriving slugs from `<h1>` text causes collisions (multiple classes can have similar display names). URL paths are guaranteed unique by WordPress.

## Workflow Step Reference (Canonical 5 Steps)
The enforced workflow has exactly 5 steps, auto-marked by hooks:
1. **EXECUTE** — auto-marks when any non-test `.go` file is edited (hook: `workflow-post-edit.sh`)
2. **SIMPLIFY** — auto-marks when a `code-simplifier` subagent stops (hook: `workflow-subagent-stop.sh`)
3. **MEMORY_CHECK** — auto-marks when a `memory-optimizer` subagent stops (hook: `workflow-subagent-stop.sh`)
4. **BUILD** — auto-marks when `go build` succeeds with no errors (hook: `workflow-post-build.sh`)
5. **COMPLETE** — auto-marks when BUILD succeeds and all other steps are done (hook: `workflow-post-build.sh`)

Build command for this project: `CGO_ENABLED=1 go build -tags sqlite_fts5 ./...`
