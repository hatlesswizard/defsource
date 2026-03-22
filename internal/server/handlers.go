package server

import (
	"log"
	"net/http"
	"strings"
	"unicode"

	defsource "github.com/hatlesswizard/defsource"
)

type handlers struct {
	client *defsource.Client
}

func (h *handlers) searchLibraries(w http.ResponseWriter, r *http.Request) {
	libraryName := strings.TrimSpace(r.URL.Query().Get("libraryName"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))

	if libraryName == "" || query == "" {
		writeError(w, http.StatusBadRequest, "libraryName and query are required")
		return
	}

	if len(libraryName) > 200 {
		writeError(w, http.StatusBadRequest, "libraryName must be 1-200 characters")
		return
	}
	if len(query) > 500 {
		writeError(w, http.StatusBadRequest, "query must be 1-500 characters")
		return
	}

	if strings.ContainsRune(query, 0) || strings.ContainsRune(libraryName, 0) {
		writeError(w, http.StatusBadRequest, "query contains invalid characters")
		return
	}

	if !hasAlphanumeric(query) {
		writeError(w, http.StatusBadRequest, "query must contain at least one alphanumeric character")
		return
	}

	results, err := h.client.ResolveLibrary(r.Context(), query, libraryName)
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

func (h *handlers) queryDocs(w http.ResponseWriter, r *http.Request) {
	libraryID := strings.TrimSpace(r.URL.Query().Get("libraryId"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	format := r.URL.Query().Get("format")

	if libraryID == "" || query == "" {
		writeError(w, http.StatusBadRequest, "libraryId and query are required")
		return
	}
	if len(libraryID) > 200 {
		writeError(w, http.StatusBadRequest, "libraryId must be 1-200 characters")
		return
	}
	if len(query) > 500 {
		writeError(w, http.StatusBadRequest, "query must be 1-500 characters")
		return
	}

	if strings.ContainsRune(query, 0) || strings.ContainsRune(libraryID, 0) {
		writeError(w, http.StatusBadRequest, "query contains invalid characters")
		return
	}

	if !hasAlphanumeric(query) {
		writeError(w, http.StatusBadRequest, "query must contain at least one alphanumeric character")
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "all"
	}
	if mode != "all" && mode != "any" {
		writeError(w, http.StatusBadRequest, "mode must be 'all' or 'any'")
		return
	}

	result, err := h.client.QueryDocs(r.Context(), libraryID, query, defsource.WithSearchMode(mode))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "library not found")
			return
		}
		log.Printf("queryDocs error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Ensure snippets is [] not null in JSON
	if result.Snippets == nil {
		result.Snippets = make([]defsource.DocSnippet, 0)
	}

	if format == "json" {
		writeJSON(w, http.StatusOK, result)
	} else {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		body := result.Text
		if strings.TrimSpace(body) == "" {
			body = "No results found for query '" + query + "' in library '" + libraryID + "'"
		}
		w.Write([]byte(body))
	}
}

func (h *handlers) listLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := h.client.ListLibraries(r.Context())
	if err != nil {
		log.Printf("listLibraries error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": libs})
}

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
