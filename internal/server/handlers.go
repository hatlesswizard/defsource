// Package server provides the HTTP API server for defSource.
// It exposes endpoints for library search, documentation query, entity listing,
// and health checks. All handlers are wired through the withMiddleware chain
// defined in middleware.go.
package server

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode"

	defsource "github.com/hatlesswizard/defsource"
	"github.com/hatlesswizard/defsource/internal/store"
)

// handlers holds a reference to the defSource client used by all HTTP handlers.
type handlers struct {
	client *defsource.Client
}

// validParams holds the validated, trimmed query parameters shared across handlers.
type validParams struct {
	// id is the trimmed library identifier or name parameter (libraryId / libraryName).
	id string
	// query is the trimmed search query string.
	query string
}

// validateQueryRequest validates and trims the two required string parameters
// common to searchLibraries and queryDocs: an identifier param (idKey) and a
// query param ("query"). It applies the following checks in order:
//
//  1. Trim whitespace from both values.
//  2. Require both values to be non-empty.
//  3. Enforce a maximum length of idMaxLen on the id value and 500 on the query.
//  4. Reject both values if either contains a null byte (\x00).
//  5. Require the query to contain at least one alphanumeric character.
//
// On any validation failure, validateQueryRequest writes a 400 Bad Request
// response and returns (validParams{}, false). On success it returns the
// trimmed params and true; the caller must not write any further response on
// failure (false) because the error has already been sent.
func validateQueryRequest(w http.ResponseWriter, r *http.Request, idKey string, idMaxLen int) (validParams, bool) {
	id := strings.TrimSpace(r.URL.Query().Get(idKey))
	query := strings.TrimSpace(r.URL.Query().Get("query"))

	if id == "" || query == "" {
		writeError(w, http.StatusBadRequest, idKey+" and query are required")
		return validParams{}, false
	}

	if len(id) > idMaxLen {
		writeError(w, http.StatusBadRequest, idKey+" must be 1-"+itoa(idMaxLen)+" characters")
		return validParams{}, false
	}
	if len(query) > 500 {
		writeError(w, http.StatusBadRequest, "query must be 1-500 characters")
		return validParams{}, false
	}

	if strings.ContainsRune(query, 0) || strings.ContainsRune(id, 0) {
		writeError(w, http.StatusBadRequest, "query contains invalid characters")
		return validParams{}, false
	}

	if !hasAlphanumeric(query) {
		writeError(w, http.StatusBadRequest, "query must contain at least one alphanumeric character")
		return validParams{}, false
	}

	return validParams{id: id, query: query}, true
}

// itoa converts a non-negative integer to its decimal string representation.
// It is a lightweight local helper used only for constructing error messages;
// it avoids importing strconv for a single use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// searchLibraries handles GET /api/v1/libraries/search.
// Required query parameters: libraryName, query.
// Returns a JSON object {"results": [...]} with matching libraries.
func (h *handlers) searchLibraries(w http.ResponseWriter, r *http.Request) {
	params, ok := validateQueryRequest(w, r, "libraryName", 200)
	if !ok {
		return
	}

	results, err := h.client.ResolveLibrary(r.Context(), params.query, params.id)
	if err != nil {
		log.Printf("searchLibraries error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if len(results) == 0 {
		writeError(w, http.StatusNotFound, "no libraries found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// queryDocs handles GET /api/v1/docs.
// Required query parameters: libraryId, query.
// Optional parameters: format ("json" or "markdown", default "markdown"), mode ("all" or "any", default "all").
// Returns documentation snippets in the requested format.
func (h *handlers) queryDocs(w http.ResponseWriter, r *http.Request) {
	params, ok := validateQueryRequest(w, r, "libraryId", 200)
	if !ok {
		return
	}

	format := r.URL.Query().Get("format")

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "all"
	}
	if mode != "all" && mode != "any" {
		writeError(w, http.StatusBadRequest, "mode must be 'all' or 'any'")
		return
	}

	result, err := h.client.QueryDocs(r.Context(), params.id, params.query, defsource.WithSearchMode(mode))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "library not found")
			return
		}
		log.Printf("queryDocs error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// A nil slice serialises to JSON null; initialise to empty slice to always
	// return [] instead, consistent with callers' expectations.
	if result.Snippets == nil {
		result.Snippets = make([]defsource.DocSnippet, 0)
	}

	if format == "json" {
		writeJSON(w, http.StatusOK, result)
	} else {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		body := result.Text
		if strings.TrimSpace(body) == "" {
			body = "No results found for query '" + params.query + "' in library '" + params.id + "'"
		}
		if _, err := w.Write([]byte(body)); err != nil {
			log.Printf("queryDocs: write response error: %v", err)
		}
	}
}

// queryDocsByLanguage handles GET /api/v1/docs/language.
// Required query parameters: language, query.
// Optional parameters: format ("json" or "markdown", default "markdown"), mode ("all" or "any", default "all").
func (h *handlers) queryDocsByLanguage(w http.ResponseWriter, r *http.Request) {
	params, ok := validateQueryRequest(w, r, "language", 50)
	if !ok {
		return
	}

	format := r.URL.Query().Get("format")

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "all"
	}
	if mode != "all" && mode != "any" {
		writeError(w, http.StatusBadRequest, "mode must be 'all' or 'any'")
		return
	}

	result, err := h.client.QueryDocsByLanguage(r.Context(), params.id, params.query, defsource.WithSearchMode(mode))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no libraries found for language")
			return
		}
		log.Printf("queryDocsByLanguage error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if result.Snippets == nil {
		result.Snippets = make([]defsource.DocSnippet, 0)
	}

	if format == "json" {
		writeJSON(w, http.StatusOK, result)
	} else {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		body := result.Text
		if strings.TrimSpace(body) == "" {
			body = "No results found for query '" + params.query + "' in language '" + params.id + "'"
		}
		if _, err := w.Write([]byte(body)); err != nil {
			log.Printf("queryDocsByLanguage: write response error: %v", err)
		}
	}
}

// listLibraries handles GET /api/v1/libraries.
// Optional query parameter: language (filter by programming language).
// Returns a JSON object {"libraries": [...]} listing indexed libraries.
func (h *handlers) listLibraries(w http.ResponseWriter, r *http.Request) {
	language := strings.TrimSpace(r.URL.Query().Get("language"))

	var libs []defsource.Library
	var err error
	if language != "" {
		libs, err = h.client.ListLibrariesByLanguage(r.Context(), language)
	} else {
		libs, err = h.client.ListLibraries(r.Context())
	}
	if err != nil {
		log.Printf("listLibraries error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": libs})
}

// listLanguages handles GET /api/v1/languages.
// Returns a JSON object {"languages": [...]} listing available languages with framework counts.
func (h *handlers) listLanguages(w http.ResponseWriter, r *http.Request) {
	langs, err := h.client.ListLanguages(r.Context())
	if err != nil {
		log.Printf("listLanguages error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"languages": langs})
}

// listEntities handles GET /api/v1/entities.
// Required query parameter: libraryId.
// Returns a JSON object {"entities": [...]} with all entities for the library.
func (h *handlers) listEntities(w http.ResponseWriter, r *http.Request) {
	libraryID := strings.TrimSpace(r.URL.Query().Get("libraryId"))
	if libraryID == "" {
		writeError(w, http.StatusBadRequest, "libraryId is required")
		return
	}
	if strings.ContainsRune(libraryID, 0) {
		writeError(w, http.StatusBadRequest, "libraryId contains invalid characters")
		return
	}
	entities, err := h.client.ListEntities(r.Context(), libraryID)
	if err != nil {
		log.Printf("listEntities error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list entities")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entities": entities})
}

// health handles GET /health.
// Returns {"status": "ok"} with HTTP 200 when the server is reachable.
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// hasAlphanumeric returns true if s contains at least one letter or digit.
func hasAlphanumeric(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
