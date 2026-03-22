# defsource

Go library for crawling, indexing, and searching API documentation with FTS5 full-text search.

[![Go Reference](https://pkg.go.dev/badge/github.com/hatlesswizard/defsource.svg)](https://pkg.go.dev/github.com/hatlesswizard/defsource)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/hatlesswizard/defsource)](https://goreportcard.com/report/github.com/hatlesswizard/defsource)

---

## Features

- Full-text search with SQLite FTS5 and BM25 ranking
- Pluggable source adapters (WordPress reference docs included)
- Concurrent crawler with rate limiting, retries, and resume support
- Token-budgeted output formatting (LLM-friendly)
- Wrapper method resolution (traces delegation chains up to 3 levels)
- Priority-based crawl ordering (critical classes first)
- HTTP REST API server
- CLI tools for crawling and serving

## Installation

```bash
go get github.com/hatlesswizard/defsource
```

**Important:** This library uses CGO via `go-sqlite3`. You must have CGO enabled and include the `sqlite_fts5` build tag:

```bash
CGO_ENABLED=1 go build -tags sqlite_fts5 ./...
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hatlesswizard/defsource"
)

func main() {
	client, err := defsource.New("./data/defsource.db")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// List all indexed libraries
	libs, err := client.ListLibraries(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, lib := range libs {
		fmt.Printf("%s (%s) - %d snippets\n", lib.Name, lib.ID, lib.SnippetCount)
	}

	// Query documentation
	if len(libs) > 0 {
		result, err := client.QueryDocs(ctx, libs[0].ID, "get posts")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(result.Text)
	}
}
```

## API Reference

### Constructor and Lifecycle

#### `New`

```go
func New(dbPath string, opts ...Option) (*Client, error)
```

Creates a new defSource client backed by a SQLite database at `dbPath`. The database file is created if it does not exist. Returns an error if the database cannot be opened or initialized.

| Parameter | Type | Description |
|-----------|------|-------------|
| `dbPath` | `string` | Filesystem path to the SQLite database file |
| `opts` | `...Option` | Zero or more functional options to configure the client |

**Defaults:**

- Token budget: 8000

#### `Close`

```go
func (c *Client) Close() error
```

Releases all database resources held by the client. Always call `Close` when you are done using the client (typically via `defer`).

#### `Store`

```go
func (c *Client) Store() store.Store
```

Returns the underlying `store.Store` interface. This is intended for use by the crawler and other internal subsystems that need direct store access.

---

### Query Methods

#### `QueryDocs`

```go
func (c *Client) QueryDocs(ctx context.Context, libraryID, query string, opts ...QueryOption) (*DocResult, error)
```

Primary search method. Performs an FTS5 full-text search against the specified library and returns documentation snippets ranked by BM25 relevance. Results are capped at 20 snippets. The `Text` field of the returned `DocResult` contains pre-formatted markdown output, trimmed to fit within the configured token budget.

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Context for cancellation and deadlines |
| `libraryID` | `string` | The ID of the library to search within |
| `query` | `string` | The search query string |
| `opts` | `...QueryOption` | Optional query configuration (e.g., search mode) |

**Returns:** `*DocResult` containing matched snippets and formatted text, or an error if the library is not found or the search fails.

```go
result, err := client.QueryDocs(ctx, "wordpress", "register post type")
fmt.Println(result.Text) // pre-formatted markdown
```

#### `ResolveLibrary`

```go
func (c *Client) ResolveLibrary(ctx context.Context, query, libraryName string) ([]Library, error)
```

Searches for libraries matching the given name and ranks them by relevance to the query. Returns up to 5 results. Ranking considers name similarity and relevance to the query context. Snippet counts are computed on the fly if not already stored.

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Context for cancellation and deadlines |
| `query` | `string` | The user's search query, used for ranking |
| `libraryName` | `string` | The library name or partial name to search for |

**Returns:** A slice of up to 5 `Library` values ranked by relevance, or an error.

```go
libs, err := client.ResolveLibrary(ctx, "custom post type", "wordpress")
```

#### `ListLibraries`

```go
func (c *Client) ListLibraries(ctx context.Context) ([]Library, error)
```

Returns all indexed libraries. For each library, the snippet count is computed on the fly if not already stored (by counting entities plus their methods).

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Context for cancellation and deadlines |

**Returns:** A slice of all `Library` values, or an error.

```go
libs, err := client.ListLibraries(ctx)
```

#### `ListEntities`

```go
func (c *Client) ListEntities(ctx context.Context, libraryID string) ([]EntityInfo, error)
```

Returns all entities (classes, functions, etc.) for a given library, including the count of methods on each entity.

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Context for cancellation and deadlines |
| `libraryID` | `string` | The ID of the library to list entities for |

**Returns:** A slice of `EntityInfo` values, or an error.

```go
entities, err := client.ListEntities(ctx, "wordpress")
```

---

### Options

#### `WithTokenBudget`

```go
func WithTokenBudget(budget int) Option
```

Sets the maximum approximate token count for `QueryDocs` responses. The formatted markdown text in `DocResult.Text` will be truncated to stay within this budget. Default is **8000** tokens. Use this to control response size when integrating with LLMs that have context limits.

```go
client, err := defsource.New("defsource.db", defsource.WithTokenBudget(4000))
```

#### `WithSearchMode`

```go
func WithSearchMode(mode string) QueryOption
```

Sets the FTS5 search mode for a `QueryDocs` call.

| Mode | Behavior | Description |
|------|----------|-------------|
| `"all"` | AND (default) | All query terms must appear in the result |
| `"any"` | OR | Any query term may appear in the result |

```go
result, err := client.QueryDocs(ctx, "wordpress", "post meta", defsource.WithSearchMode("any"))
```

---

### Types

#### `Library`

Represents an indexed documentation source.

```go
type Library struct {
    ID           string    `json:"id"`            // Unique identifier for the library
    Name         string    `json:"name"`          // Human-readable library name
    Description  string    `json:"description"`   // Short description of the library
    SourceURL    string    `json:"source_url"`    // URL of the original documentation source
    Version      string    `json:"version"`       // Library version string
    TrustScore   float64   `json:"trust_score"`   // Confidence score (0.0-1.0) for the source
    SnippetCount int       `json:"snippet_count"` // Total number of indexed snippets (entities + methods)
    CrawledAt    time.Time `json:"crawled_at"`    // Timestamp of the last completed crawl
}
```

#### `DocResult`

The response returned by `QueryDocs`.

```go
type DocResult struct {
    Library  string       `json:"library"`  // ID of the queried library
    Query    string       `json:"query"`    // The original search query
    Snippets []DocSnippet `json:"snippets"` // Matched documentation snippets ranked by relevance
    Text     string       `json:"text"`     // Pre-formatted markdown output (token-budgeted)
}
```

#### `DocSnippet`

A single documentation entry representing either a class/entity or a specific method.

```go
type DocSnippet struct {
    EntityName    string      `json:"entity_name"`              // Name of the parent entity (class, function group)
    MethodName    string      `json:"method_name,omitempty"`    // Method name, empty if this is an entity-level snippet
    Signature     string      `json:"signature,omitempty"`      // Full method signature
    Description   string      `json:"description"`              // Human-readable description
    Parameters    []Parameter `json:"parameters,omitempty"`     // Method parameters (empty for entity-level snippets)
    ReturnType    string      `json:"return_type,omitempty"`    // Return type string
    ReturnDesc    string      `json:"return_desc,omitempty"`    // Description of the return value
    SourceCode    string      `json:"source_code"`              // Source code of the entity or method
    WrappedSource string      `json:"wrapped_source,omitempty"` // Source code of the delegated-to method (wrapper resolution)
    WrappedMethod string      `json:"wrapped_method,omitempty"` // Name of the method this wraps/delegates to
    URL           string      `json:"url"`                      // URL to the original documentation page
    Relevance     float64     `json:"relevance"`                // BM25 relevance score from FTS5
    Relations     []Relation  `json:"relations,omitempty"`      // Relationships to other methods (uses/used_by)
}
```

#### `Parameter`

Describes a function or method parameter.

```go
type Parameter struct {
    Name        string `json:"name"`        // Parameter name
    Type        string `json:"type"`        // Parameter type (e.g., "string", "int", "array")
    Required    bool   `json:"required"`    // Whether the parameter is required
    Description string `json:"description"` // Human-readable description of the parameter
}
```

#### `Relation`

Describes a relationship between methods.

```go
type Relation struct {
    Kind        string `json:"kind"`                  // Relationship type: "uses" or "used_by"
    TargetName  string `json:"target_name"`           // Name of the related method
    TargetURL   string `json:"target_url,omitempty"`  // URL to the related method's documentation
    Description string `json:"description,omitempty"` // Description of the relationship
}
```

#### `EntityInfo`

Summary information about an entity, returned by `ListEntities`.

```go
type EntityInfo struct {
    Name        string `json:"name"`         // Entity name (e.g., "WP_Query")
    Slug        string `json:"slug"`         // URL-safe slug for the entity
    Kind        string `json:"kind"`         // Entity kind (e.g., "class", "function")
    Description string `json:"description"`  // Short description
    MethodCount int    `json:"method_count"` // Number of methods belonging to this entity
    URL         string `json:"url"`          // URL to the original documentation page
}
```

---

### Usage Examples

#### Basic Query

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hatlesswizard/defsource"
)

func main() {
	client, err := defsource.New("./data/defsource.db")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	result, err := client.QueryDocs(context.Background(), "wordpress", "register post type")
	if err != nil {
		log.Fatal(err)
	}

	// Print pre-formatted markdown (token-budgeted)
	fmt.Println(result.Text)

	// Or iterate over individual snippets
	for _, s := range result.Snippets {
		fmt.Printf("[%.2f] %s::%s\n", s.Relevance, s.EntityName, s.MethodName)
	}
}
```

#### Library Discovery

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hatlesswizard/defsource"
)

func main() {
	client, err := defsource.New("./data/defsource.db")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Find the best matching library
	libs, err := client.ResolveLibrary(ctx, "custom post type", "wordpress")
	if err != nil {
		log.Fatal(err)
	}
	if len(libs) == 0 {
		log.Fatal("no matching library found")
	}

	fmt.Printf("Using library: %s (trust: %.1f)\n", libs[0].Name, libs[0].TrustScore)

	// Query docs in the resolved library
	result, err := client.QueryDocs(ctx, libs[0].ID, "custom post type")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Text)
}
```

#### Custom Configuration

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hatlesswizard/defsource"
)

func main() {
	// Create a client with a smaller token budget
	client, err := defsource.New(
		"./data/defsource.db",
		defsource.WithTokenBudget(4000),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Use OR mode to find results matching any term
	result, err := client.QueryDocs(
		context.Background(),
		"wordpress",
		"meta query tax_query",
		defsource.WithSearchMode("any"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d snippets\n", len(result.Snippets))
	fmt.Println(result.Text)
}
```

## CLI Tools

### defsource-crawl

Crawls a documentation source and stores the results in a SQLite database.

**Build:**

```bash
CGO_ENABLED=1 go build -tags sqlite_fts5 -o bin/defsource-crawl ./cmd/defsource-crawl
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-source` | `wordpress` | Documentation source to crawl |
| `-db` | `./data/defsource.db` | Path to SQLite database |
| `-workers` | `10` | Number of concurrent workers |
| `-rps` | `10` | Requests per second (rate limit) |
| `-resume` | `false` | Resume the last interrupted crawl |
| `-retry-failed` | `false` | Retry transient failures from the last crawl |

**Examples:**

```bash
# Full crawl with default settings
./bin/defsource-crawl --source=wordpress --db=./data/defsource.db

# Fast crawl with more workers
./bin/defsource-crawl --source=wordpress --workers=20 --rps=20

# Resume an interrupted crawl
./bin/defsource-crawl --source=wordpress --resume

# Retry only the pages that failed last time
./bin/defsource-crawl --source=wordpress --retry-failed
```

The crawler supports graceful shutdown via SIGINT (Ctrl+C) or SIGTERM. An interrupted crawl can be continued later with `--resume`.

### defsource-server

Serves the indexed documentation over an HTTP REST API.

**Build:**

```bash
CGO_ENABLED=1 go build -tags sqlite_fts5 -o bin/defsource-server ./cmd/defsource-server
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-db` | `./data/defsource.db` | Path to SQLite database |
| `-addr` | `:8080` | Server listen address |

**Environment Variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `DEFSOURCE_CORS_ORIGIN` | `*` | Allowed CORS origin header value |

**Examples:**

```bash
# Start with defaults
./bin/defsource-server

# Custom port and database
./bin/defsource-server --db=./mydata/docs.db --addr=:3000

# Restrict CORS to a specific origin
DEFSOURCE_CORS_ORIGIN=https://example.com ./bin/defsource-server --addr=:8080
```

The server supports graceful shutdown via SIGINT or SIGTERM with a 10-second drain timeout.

## HTTP API

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/libraries` | List all indexed libraries |
| GET | `/api/v1/libraries/search` | Search for libraries by name |
| GET | `/api/v1/docs` | Query documentation with full-text search |
| GET | `/api/v1/entities` | List entities for a library |
| GET | `/health` | Health check |

### GET /api/v1/libraries

Returns all indexed libraries.

```bash
curl http://localhost:8080/api/v1/libraries
```

**Response:**

```json
{
  "libraries": [
    {
      "id": "wordpress",
      "name": "WordPress",
      "description": "WordPress Class Reference",
      "source_url": "https://developer.wordpress.org",
      "version": "6.x",
      "trust_score": 0.95,
      "snippet_count": 4200,
      "crawled_at": "2025-01-15T10:30:00Z"
    }
  ]
}
```

### GET /api/v1/libraries/search

Search for libraries by name, ranked by relevance to a query.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `libraryName` | Yes | Library name or partial name (max 200 chars) |
| `query` | Yes | Search query for ranking (max 500 chars) |

```bash
curl "http://localhost:8080/api/v1/libraries/search?libraryName=wordpress&query=post+type"
```

**Response:**

```json
{
  "results": [
    {
      "id": "wordpress",
      "name": "WordPress",
      "description": "WordPress Class Reference",
      "source_url": "https://developer.wordpress.org",
      "version": "6.x",
      "trust_score": 0.95,
      "snippet_count": 4200,
      "crawled_at": "2025-01-15T10:30:00Z"
    }
  ]
}
```

### GET /api/v1/docs

Query documentation with full-text search. Returns markdown by default or structured JSON.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `libraryId` | Yes | Library ID to search within (max 200 chars) |
| `query` | Yes | Search query string (max 500 chars) |
| `mode` | No | Search mode: `all` (AND, default) or `any` (OR) |
| `format` | No | Response format: omit for markdown, `json` for structured JSON |

```bash
# Markdown response (default)
curl "http://localhost:8080/api/v1/docs?libraryId=wordpress&query=register+post+type"

# JSON response
curl "http://localhost:8080/api/v1/docs?libraryId=wordpress&query=register+post+type&format=json"

# OR mode
curl "http://localhost:8080/api/v1/docs?libraryId=wordpress&query=meta+query+tax_query&mode=any"
```

**Markdown response:** Returns `Content-Type: text/markdown; charset=utf-8` with the pre-formatted documentation text.

**JSON response:**

```json
{
  "library": "wordpress",
  "query": "register post type",
  "snippets": [
    {
      "entity_name": "WP_Post_Type",
      "method_name": "register_post_type",
      "signature": "register_post_type( string $post_type, array $args = array() )",
      "description": "Registers a post type.",
      "parameters": [
        {
          "name": "$post_type",
          "type": "string",
          "required": true,
          "description": "Post type key."
        }
      ],
      "return_type": "WP_Post_Type|WP_Error",
      "return_desc": "The registered post type object or an error.",
      "source_code": "...",
      "url": "https://developer.wordpress.org/reference/functions/register_post_type/",
      "relevance": 12.5,
      "relations": []
    }
  ],
  "text": "# register_post_type\n..."
}
```

### GET /api/v1/entities

List all entities (classes, functions) for a library, with method counts.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `libraryId` | Yes | Library ID to list entities for |

```bash
curl "http://localhost:8080/api/v1/entities?libraryId=wordpress"
```

**Response:**

```json
{
  "entities": [
    {
      "name": "WP_Query",
      "slug": "wp_query",
      "kind": "class",
      "description": "The WordPress Query class.",
      "method_count": 42,
      "url": "https://developer.wordpress.org/reference/classes/wp_query/"
    }
  ]
}
```

### GET /health

Health check endpoint.

```bash
curl http://localhost:8080/health
```

**Response:**

```json
{
  "status": "ok"
}
```

## Architecture

defSource follows a pipeline architecture: documentation sources are discovered, crawled concurrently with rate limiting, parsed into structured entities and methods, and stored in a SQLite database with FTS5 full-text indexes. At query time, FTS5 performs BM25-ranked searches, and results are formatted into token-budgeted markdown suitable for LLM consumption.

```
Discover -> Crawl -> Parse -> Store (SQLite+FTS5) -> Search -> Format
```

The system uses a pluggable source adapter pattern. Each source implements a `Source` interface that defines how to discover pages, parse entities, and extract methods. This makes it straightforward to add support for new documentation sources without modifying the core crawling or search infrastructure.

## Supported Sources

### WordPress Class Reference

Crawls the [WordPress Developer Reference](https://developer.wordpress.org/reference/) and indexes:

- Classes and their methods (e.g., `WP_Query`, `WP_Post`, `WP_REST_Controller`)
- Function signatures, parameters, return types, and descriptions
- Source code for both methods and their wrapper targets
- Cross-references between methods (uses/used_by relationships)
- Priority-based ordering ensures critical classes (like `WP_Query`) are crawled first

### Extensibility

New documentation sources can be added by implementing the `Source` interface in the `internal/source` package. The interface defines methods for discovering entity URLs, parsing entity pages, and extracting method details.

## Development

```bash
# Build both CLI tools
make build

# Run tests
make test

# Crawl WordPress docs (builds first)
make crawl

# Start the HTTP server (builds first)
make server

# Run linter
make lint

# Clean build artifacts and database
make clean
```

## Requirements

- **Go** 1.25.3 or later (as specified in `go.mod`)
- **CGO** must be enabled (`CGO_ENABLED=1`) -- required by `go-sqlite3`
- **Build tag** `sqlite_fts5` must be included (`-tags sqlite_fts5`)
- A C compiler (GCC or Clang) for CGO compilation

## License

MIT -- see [LICENSE](LICENSE) for details.
