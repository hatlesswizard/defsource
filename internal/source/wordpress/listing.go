package wordpress

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/hatlesswizard/defsource/internal/source"
)

func (w *WordPressSource) DiscoverEntities(ctx context.Context, fetch source.FetchFunc) ([]string, error) {
	var allURLs []string

	// Fetch page 1 to discover total page count
	firstPageURL := BaseURL + "/reference/classes/"
	body, err := fetch(ctx, firstPageURL)
	if err != nil {
		return nil, fmt.Errorf("fetch classes listing page 1: %w", err)
	}

	urls, totalPages, err := parseListingPage(body)
	if err != nil {
		return nil, fmt.Errorf("parse classes listing page 1: %w", err)
	}
	allURLs = append(allURLs, urls...)
	log.Printf("Page 1: found %d class URLs", len(urls))

	// Fetch remaining pages
	for page := 2; page <= totalPages; page++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pageURL := fmt.Sprintf("%s/reference/classes/page/%d/", BaseURL, page)
		body, err := fetch(ctx, pageURL)
		if err != nil {
			log.Printf("WARNING: fetch classes listing page %d failed: %v, continuing with collected URLs", page, err)
			continue
		}

		urls, _, err := parseListingPage(body)
		if err != nil {
			log.Printf("WARNING: parse classes listing page %d failed: %v, continuing with collected URLs", page, err)
			continue
		}
		allURLs = append(allURLs, urls...)
		log.Printf("Page %d/%d: found %d class URLs (total: %d)", page, totalPages, len(urls), len(allURLs))
	}

	// Sort so WP_* classes come first — they're the most important and
	// must be crawled before a timeout kills the process.
	sort.SliceStable(allURLs, func(i, j int) bool {
		si := extractSlugFromURL(allURLs[i])
		sj := extractSlugFromURL(allURLs[j])
		pi := classPriority(si)
		pj := classPriority(sj)
		if pi != pj {
			return pi < pj
		}
		return si < sj
	})

	// Count priorities
	var t0, t1, t2 int
	for _, u := range allURLs {
		switch classPriority(extractSlugFromURL(u)) {
		case 0:
			t0++
		case 1:
			t1++
		default:
			t2++
		}
	}
	log.Printf("Discovery complete: %d entities across %d pages (tier-0: %d, tier-1: %d, tier-2: %d)",
		len(allURLs), totalPages, t0, t1, t2)

	return allURLs, nil
}

func extractSlugFromURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		return url[idx+1:]
	}
	return url
}

func classPriority(slug string) int {
	lower := strings.ToLower(slug)

	// Tier 0: Most-used WordPress classes (must crawl first)
	topTier := map[string]bool{
		"wp_query": true, "wp_post": true, "wp_user": true,
		"wp_error": true, "wp_term": true, "wp_comment": true,
		"wp_widget": true, "wp_theme": true, "wp_hook": true,
		"wp_rewrite": true, "wp_roles": true, "wp_role": true,
		"wp_rest_request": true, "wp_rest_response": true, "wp_rest_server": true,
		"wp_rest_controller": true,
		"wp_taxonomy": true, "wp_post_type": true,
		"wp_scripts": true, "wp_styles": true,
		"wp_customize_manager": true, "wp_customize_control": true,
		"wp_nav_menu_item": true, "wp_http": true,
		"wp_filesystem_base": true, "wp_filesystem_direct": true,
		"wp_session_tokens": true, "wp_meta_query": true,
		"wp_tax_query": true, "wp_date_query": true,
		"wp_block": true, "wp_block_type": true,
		"wpdb": true,
	}
	if topTier[lower] {
		return 0 // Highest priority
	}
	if strings.HasPrefix(lower, "wp_") {
		return 1 // Other WP_* classes
	}
	return 2 // Everything else
}

// parseListingPage extracts class URLs and total page count from a listing page.
func parseListingPage(body []byte) (urls []string, totalPages int, err error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("parse HTML: %w", err)
	}

	// Extract class URLs
	doc.Find(".wp-block-wporg-code-short-title a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && strings.Contains(href, "/reference/classes/") {
			if !strings.HasPrefix(href, "http") {
				href = BaseURL + href
			}
			urls = append(urls, href)
		}
	})

	// Extract total page count from pagination
	totalPages = 1
	doc.Find("nav.wp-block-query-pagination a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if _, after, ok := strings.Cut(href, "/page/"); ok {
			after = strings.TrimSuffix(after, "/")
			if n, err := strconv.Atoi(after); err == nil && n > totalPages {
				totalPages = n
			}
		}
	})

	if len(urls) == 0 {
		return nil, 0, fmt.Errorf("no class URLs found on page — HTML structure may have changed")
	}

	return urls, totalPages, nil
}
