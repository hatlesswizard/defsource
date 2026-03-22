.PHONY: build crawl server test lint clean

build:
	CGO_ENABLED=1 go build -tags sqlite_fts5 -o bin/defsource-crawl ./cmd/defsource-crawl
	CGO_ENABLED=1 go build -tags sqlite_fts5 -o bin/defsource-server ./cmd/defsource-server

crawl: build
	./bin/defsource-crawl --source=wordpress --db=./data/defsource.db

server: build
	./bin/defsource-server --db=./data/defsource.db --addr=:8080

test:
	CGO_ENABLED=1 go test -tags sqlite_fts5 -v ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ data/defsource.db
