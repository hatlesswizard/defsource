# defsource

`defsource` is a Go library for crawling, indexing, and searching source-code documentation. It parses documentation from upstream sources into a single SQLite database backed by FTS5, then exposes a small client API for resolving libraries and querying documentation snippets with BM25 relevance ranking. Results can be returned as structured data or as pre-formatted, token-budgeted text suitable for feeding to downstream tools.

[![Go Reference](https://pkg.go.dev/badge/github.com/hatlesswizard/defsource.svg)](https://pkg.go.dev/github.com/hatlesswizard/defsource)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

## Features

- Full-text search over indexed documentation using SQLite FTS5 with BM25 ranking.
- `all` (AND) and `any` (OR) search modes, selectable per query.
- Library resolution that matches a name and re-ranks candidates against a query.
- Structured snippet results plus a pre-formatted, token-budgeted text rendering.
- Pluggable storage: open a SQLite-backed client, or inject any `store.Store` implementation for testing.
- Wrapper/delegation chain resolution so a snippet can carry the underlying source it forwards to.

## Requirements

- Go 1.25.3 or newer.
- CGO enabled (`CGO_ENABLED=1`) with a working C compiler, because the SQLite driver is implemented via cgo.
- The `sqlite_fts5` build tag, which enables FTS5 full-text search support in the SQLite driver.

## Installation

```bash
go get github.com/hatlesswizard/defsource
```

Because the library depends on cgo and FTS5, build and test commands must enable CGO and pass the `sqlite_fts5` tag:

```bash
CGO_ENABLED=1 go build -tags sqlite_fts5 ./...
```

## Library Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hatlesswizard/defsource"
)

func main() {
	// Open (or create) a SQLite database backed by FTS5.
	client, err := defsource.New("docs.db", defsource.WithTokenBudget(4000))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// List the libraries that have been indexed into the database.
	libraries, err := client.ListLibraries(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, lib := range libraries {
		fmt.Printf("%s (%s) — %d snippets\n", lib.Name, lib.ID, lib.SnippetCount)
	}

	if len(libraries) == 0 {
		return
	}

	// Query documentation for the first library, using OR (any) semantics.
	result, err := client.QueryDocs(ctx, libraries[0].ID, "query posts by author",
		defsource.WithSearchMode("any"))
	if err != nil {
		log.Fatal(err)
	}

	// Each snippet carries structured fields...
	for _, snippet := range result.Snippets {
		fmt.Printf("%s %s\n", snippet.EntityName, snippet.MethodName)
	}

	// ...and result.Text holds a pre-formatted, token-budgeted rendering.
	fmt.Println(result.Text)
}
```

The client also exposes `ResolveLibrary` (match a library by name and re-rank against a query) and `ListEntities` (summarise the classes and functions in a library). See the [package reference](https://pkg.go.dev/github.com/hatlesswizard/defsource) for the full API.

## Testing

```bash
CGO_ENABLED=1 go test -tags sqlite_fts5 ./...
```

## License

This project is licensed under the GNU General Public License v3.0. See the [LICENSE](LICENSE) file for the full text.
