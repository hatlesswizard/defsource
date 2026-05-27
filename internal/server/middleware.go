package server

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// withMiddleware composes the full middleware chain around h.
//
// Wrap order (top = first call to withMiddleware, bottom = last):
//
//	withLogging       ← applied first → outermost at request time
//	withCORS          ← applied second
//	withSecurityHeaders ← applied third
//	withRecovery      ← applied last  → innermost at request time
//
// Execution order at request time is the REVERSE of wrap order because each
// call wraps the result of the previous call:
//
//  1. withLogging        (outermost: measures total latency + captures status)
//  2. withCORS           (adds CORS headers before forwarding)
//  3. withSecurityHeaders (adds security headers before forwarding)
//  4. withRecovery       (innermost: catches panics from the mux and handlers)
//  5. mux/handler        (actual handler logic)
//
// IMPORTANT: Any new middleware added here must account for this inversion.
// Inserting h = withXxx(h) at the top of the function places withXxx as the
// new outermost layer (last to execute); inserting at the bottom places it
// between withRecovery and the mux (second-innermost). Adding middleware
// outside withLogging means it runs before latency measurement begins.
func withMiddleware(h http.Handler, corsOrigin string) http.Handler {
	h = withLogging(h)
	h = withCORS(h, corsOrigin)
	h = withSecurityHeaders(h)
	h = withRecovery(h)
	return h
}

// withCORS adds Access-Control headers to every response and short-circuits
// OPTIONS preflight requests with 204 No Content.
func withCORS(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withSecurityHeaders adds conservative security-related HTTP response headers
// to every response to reduce common client-side attack surface.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// withRecovery is the innermost middleware. It catches any panic that escapes
// a handler, logs the panic value together with the full goroutine stack trace
// for diagnosis, and returns HTTP 500 Internal Server Error to the client.
// Including the stack trace is critical: without file and line information,
// pinpointing the panic source in a multi-file codebase requires adding extra
// instrumentation after the fact.
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v\n%s", err, debug.Stack())
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code written by
// the handler so that withLogging can record it in the access log.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code and delegates to the underlying writer.
func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

// withLogging is the outermost middleware. It records the HTTP method, path,
// response status, and elapsed time for every request.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

// writeJSON sets Content-Type to application/json, writes the given HTTP status
// code, and encodes v as JSON into the response body. HTML escaping is disabled
// so that URLs and other text in documentation snippets are not mangled.
// If encoding fails after the status code has been committed, the error is
// logged; the client will receive a truncated or empty body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

// writeError sends a JSON error response of the form {"error": msg} with the
// given HTTP status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
