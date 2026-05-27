// Package server provides the HTTP API server for defSource.
// Use New to construct an *http.Server wired to a defSource client, then start
// it with ListenAndServe. The server applies the withMiddleware chain (logging,
// CORS, security headers, panic recovery) to all routes.
package server

import (
	"net/http"
	"time"

	defsource "github.com/hatlesswizard/defsource"
)

// New creates an HTTP server wired to the defSource client.
// corsOrigin is used verbatim as the Access-Control-Allow-Origin response header.
// All routes are wrapped with the full middleware chain: logging (outermost),
// CORS, security headers, and panic recovery (innermost). See middleware.go for
// the complete execution-order description.
func New(client *defsource.Client, addr string, corsOrigin string) *http.Server {
	mux := http.NewServeMux()
	h := &handlers{client: client}

	mux.HandleFunc("GET /api/v1/libraries/search", h.searchLibraries)
	mux.HandleFunc("GET /api/v1/docs", h.queryDocs)
	mux.HandleFunc("GET /api/v1/libraries", h.listLibraries)
	mux.HandleFunc("GET /api/v1/entities", h.listEntities)
	mux.HandleFunc("GET /health", h.health)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})

	return &http.Server{
		Addr:         addr,
		Handler:      withMiddleware(mux, corsOrigin),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
