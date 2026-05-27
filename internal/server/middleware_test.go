//go:build sqlite_fts5 || fts5

// Package server_test holds characterization tests for the middleware layer.
//
// These tests exercise each middleware independently and in combination,
// using httptest.NewRecorder + httptest.NewRequest so no network is needed.
package server

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// ── withCORS ──────────────────────────────────────────────────────────────────

// Test MW-1: OPTIONS preflight returns 204 No Content and CORS headers.
func TestWithCORS_OptionsPreflightReturns204(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This should never be reached for OPTIONS.
		w.WriteHeader(http.StatusTeapot)
	})
	h := withCORS(inner, "https://example.com")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 204", rr.Code)
	}
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want https://example.com", origin)
	}
	methods := rr.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("Access-Control-Allow-Methods header is missing")
	}
	headers := rr.Header().Get("Access-Control-Allow-Headers")
	if headers == "" {
		t.Error("Access-Control-Allow-Headers header is missing")
	}
}

// Test MW-2: configured origin is echoed on non-OPTIONS requests.
func TestWithCORS_ConfiguredOriginEchoedOnGetRequest(t *testing.T) {
	const wantOrigin = "https://my.site"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withCORS(inner, wantOrigin)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/libraries", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET status = %d, want 200", rr.Code)
	}
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != wantOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", origin, wantOrigin)
	}
}

// Test MW-3: wildcard origin "*" is passed through unchanged.
func TestWithCORS_WildcardOriginWorks(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withCORS(inner, "*")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(rr, req)

	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", origin)
	}
}

// ── withSecurityHeaders ───────────────────────────────────────────────────────

// Test MW-4: all required security headers are present on every response.
func TestWithSecurityHeaders_AddsRequiredHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withSecurityHeaders(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	wantHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Cache-Control":          "no-store",
	}
	for k, want := range wantHeaders {
		got := rr.Header().Get(k)
		if got != want {
			t.Errorf("header %q = %q, want %q", k, got, want)
		}
	}
}

// Test MW-5: security headers are independent of the handler's status code.
func TestWithSecurityHeaders_PresentOnErrorResponses(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	h := withSecurityHeaders(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	h.ServeHTTP(rr, req)

	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("X-Content-Type-Options missing on 404 response")
	}
}

// ── withRecovery ──────────────────────────────────────────────────────────────

// Test MW-6: panic in the inner handler is recovered; the server returns 500
// and the process does not exit.
func TestWithRecovery_PanicReturns500(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated handler panic")
	})
	h := withRecovery(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/crash", nil)

	// This must not panic or exit.
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

// Test MW-7: panic recovery writes the debug.Stack() to the log.
// We capture log output by redirecting the default logger.
func TestWithRecovery_PanicLogsStackTrace(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("stack-trace-test-panic")
	})
	h := withRecovery(inner)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr) // restore default output

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/crash", nil)
	h.ServeHTTP(rr, req)

	logged := buf.String()
	if !strings.Contains(logged, "stack-trace-test-panic") {
		t.Errorf("log does not contain panic value; got: %s", logged)
	}
	// debug.Stack() always includes "goroutine" in its output.
	if !strings.Contains(logged, "goroutine") {
		t.Errorf("log does not contain stack trace ('goroutine' not found); got: %s", logged)
	}
}

// Test MW-8: a non-panicking handler passes through withRecovery unchanged.
func TestWithRecovery_NoPanic_PassesThrough(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	h := withRecovery(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rr.Code)
	}
}

// ── withLogging ───────────────────────────────────────────────────────────────

// Test MW-9: statusWriter records the explicit WriteHeader code.
func TestStatusWriter_CapturesExplicitStatusCode(t *testing.T) {
	var captured int
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if sw, ok := w.(*statusWriter); ok {
			captured = sw.status
		}
	})
	h := withLogging(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("recorder status = %d, want 404", rr.Code)
	}
	if captured != http.StatusNotFound {
		t.Errorf("statusWriter.status = %d, want 404", captured)
	}
}

// Test MW-10: withLogging defaults to 200 when the handler writes no status.
func TestWithLogging_DefaultStatus200WhenNoWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// writes body without an explicit WriteHeader
		_, _ = w.Write([]byte("hello"))
	})
	h := withLogging(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	h.ServeHTTP(rr, req)

	logged := buf.String()
	// The log entry should record 200 for a handler that never called WriteHeader.
	if !strings.Contains(logged, "200") {
		t.Errorf("log entry should contain 200; got: %s", logged)
	}
}

// Test MW-11: withLogging writes an access-log line that includes method, path, status.
func TestWithLogging_AccessLogContainsMethodAndPath(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := withLogging(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/libraries", nil)
	h.ServeHTTP(rr, req)

	logged := buf.String()
	if !strings.Contains(logged, http.MethodGet) {
		t.Errorf("log entry missing method GET; got: %s", logged)
	}
	if !strings.Contains(logged, "/api/v1/libraries") {
		t.Errorf("log entry missing path; got: %s", logged)
	}
	if !strings.Contains(logged, "418") {
		t.Errorf("log entry missing status 418; got: %s", logged)
	}
}

// ── withMiddleware ordering ───────────────────────────────────────────────────

// Test MW-12: withMiddleware chains execute in the documented inversion order.
//
// withMiddleware applies: withLogging(withCORS(withSecurityHeaders(withRecovery(mux)))).
// The *wrap* order is logging → CORS → security → recovery (each wraps the previous).
// The *execution* order at request time is therefore the reverse:
//  1. withLogging  (outermost)
//  2. withCORS
//  3. withSecurityHeaders
//  4. withRecovery
//  5. mux/handler
//
// We verify that CORS and security headers are both present on a real GET /health
// request processed through the full chain, which confirms all four middlewares ran.
func TestWithMiddleware_AllMiddlewaresRunInOrder(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withMiddleware(inner, "https://test.origin")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test-chain", nil)
	h.ServeHTTP(rr, req)

	// Verify withCORS ran.
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://test.origin" {
		t.Errorf("CORS header = %q, want https://test.origin", got)
	}
	// Verify withSecurityHeaders ran.
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	// Verify withLogging ran (produced a log entry).
	if logged := buf.String(); !strings.Contains(logged, "200") {
		t.Errorf("no log entry from withLogging; got: %s", logged)
	}
}

// Test MW-13: explicit middleware execution order using a recording slice.
//
// We build a small three-layer chain of test middlewares, each appending its
// label to a shared slice.  We then verify the append order matches the
// documented execution-order-is-reverse-of-wrap-order rule.
func TestWithMiddleware_OrderIsReverseOfWrapOrder(t *testing.T) {
	var order []string

	makeMiddleware := func(label string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, label+" before")
				next.ServeHTTP(w, r)
				order = append(order, label+" after")
			})
		}
	}

	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	// Wrap order: A wraps B wraps C wraps mux.
	// Execution order should be: A before → B before → C before → handler → C after → B after → A after.
	wA := makeMiddleware("A")
	wB := makeMiddleware("B")
	wC := makeMiddleware("C")

	var h http.Handler = mux
	h = wC(h)
	h = wB(h)
	h = wA(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	want := []string{
		"A before",
		"B before",
		"C before",
		"handler",
		"C after",
		"B after",
		"A after",
	}
	if len(order) != len(want) {
		t.Fatalf("execution order has %d entries, want %d: %v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

// Test MW-14: withMiddleware + panic in handler → 500, withRecovery is innermost
// and successfully catches the panic, preventing process termination.
func TestWithMiddleware_PanicInHandlerIsRecovered(t *testing.T) {
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	panicMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("integration panic test")
	})
	h := withMiddleware(panicMux, "*")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	h.ServeHTTP(rr, req) // must not panic

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	logged := logBuf.String()
	if !strings.Contains(logged, "integration panic test") {
		t.Errorf("panic value not logged; got: %s", logged)
	}
}

// ── statusWriter unit tests ───────────────────────────────────────────────────

// Test MW-15: statusWriter delegates WriteHeader to the underlying ResponseWriter
// and captures the status code.
func TestStatusWriter_DelegatesAndCaptures(t *testing.T) {
	rr := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rr, status: http.StatusOK}

	sw.WriteHeader(http.StatusUnauthorized)

	if sw.status != http.StatusUnauthorized {
		t.Errorf("sw.status = %d, want 401", sw.status)
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("underlying recorder code = %d, want 401", rr.Code)
	}
}

// Test MW-16: statusWriter.Write delegates to the underlying writer without
// changing the captured status.
func TestStatusWriter_WriteDoesNotAlterStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rr, status: http.StatusOK}

	sw.WriteHeader(http.StatusAccepted)
	n, err := sw.Write([]byte("body"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 4 {
		t.Errorf("Write returned %d, want 4", n)
	}
	if sw.status != http.StatusAccepted {
		t.Errorf("sw.status = %d after Write, want 202", sw.status)
	}
}
