// This stub go.mod fences the data/ directory (downloaded source repos,
// SQLite databases) off from the parent module so that `go build ./...`
// and `go test ./...` never descend into cached third-party code.
module defsource-data-fence

go 1.22
