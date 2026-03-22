package crawler

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

type Fetcher struct {
	client      *http.Client
	userAgent   string
	maxRetry    int
	rateLimiter <-chan time.Time
	ticker      *time.Ticker
}

func NewFetcher(requestsPerSecond int, maxRetry int, userAgent string) *Fetcher {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 5
	}
	ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
	return &Fetcher{
		client:      &http.Client{Timeout: 30 * time.Second},
		userAgent:   userAgent,
		maxRetry:    maxRetry,
		rateLimiter: ticker.C,
		ticker:      ticker,
	}
}

func (f *Fetcher) Close() { f.ticker.Stop() }

// Fetch downloads a URL with rate limiting, retries, and exponential backoff.
func (f *Fetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	// Wait for rate limiter tick
	select {
	case <-f.rateLimiter:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	var lastErr error
	for attempt := 0; attempt <= f.maxRetry; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("User-Agent", f.userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read body: %w", err)
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			return body, nil
		case resp.StatusCode == http.StatusTooManyRequests:
			lastErr = fmt.Errorf("rate limited (429) on %s", url)
			continue
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("server error (%d) on %s", resp.StatusCode, url)
			continue
		case resp.StatusCode == http.StatusNotFound:
			return nil, fmt.Errorf("page not found (404): %s", url)
		default:
			return nil, fmt.Errorf("unexpected status %d on %s", resp.StatusCode, url)
		}
	}

	return nil, fmt.Errorf("all %d retries failed for %s: %w", f.maxRetry+1, url, lastErr)
}
