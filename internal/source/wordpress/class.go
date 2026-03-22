package wordpress

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/hatlesswizard/defsource/internal/source"
)

func (w *WordPressSource) ParseEntity(_ context.Context, url string, body []byte) (*source.Entity, []string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("parse HTML: %w", err)
	}

	entity := &source.Entity{
		URL: url,
	}

	// Derive slug from URL path for uniqueness (e.g. /reference/classes/wp_query/ → wp_query)
	entity.Slug = slugFromURL(url)
	entity.Kind = "class"

	// Extract class name from <h1>, cleaning up "class X {}" format
	h1Text := strings.TrimSpace(doc.Find("article h1, main h1").First().Text())
	if h1Text == "" {
		return nil, nil, fmt.Errorf("no h1 found on page %s", url)
	}
	entity.Name = cleanClassName(h1Text)

	// Extract description: prefer meta description, fallback to paragraphs before first h2
	entity.Description = extractEntityDescription(doc)

	// Extract source file path: look for file reference links
	if fileLink := doc.Find("a[href*='/reference/files/']").First(); fileLink.Length() > 0 {
		entity.SourceFile = strings.TrimSpace(fileLink.Text())
	}
	if entity.SourceFile == "" {
		doc.Find("h2#source").Each(func(i int, s *goquery.Selection) {
			nextSection := s.Parent()
			filePath := nextSection.Find("code").First().Text()
			if filePath != "" && strings.HasSuffix(filePath, ".php") {
				entity.SourceFile = strings.TrimSpace(filePath)
			}
		})
	}

	// Extract source code with fallback selectors
	entity.SourceCode = extractSourceCodeAfterHeading(doc, "source")
	if entity.SourceCode == "" {
		if codeEl := doc.Find(".wp-block-wporg-code-reference-source pre code").First(); codeEl.Length() > 0 {
			entity.SourceCode = strings.TrimSpace(codeEl.Text())
		}
	}
	if entity.SourceCode == "" {
		if codeEl := doc.Find("section pre code").First(); codeEl.Length() > 0 {
			entity.SourceCode = strings.TrimSpace(codeEl.Text())
		}
	}

	// Extract properties: find by heading ID or by heading text content
	extractProperties(doc, entity)

	// Extract method URLs from the methods table
	seen := make(map[string]bool)
	var methodURLs []string
	addMethodURL := func(href string) {
		if !strings.HasPrefix(href, "http") {
			href = BaseURL + href
		}
		if !seen[href] {
			seen[href] = true
			methodURLs = append(methodURLs, href)
		}
	}

	doc.Find("h2#methods").Each(func(i int, s *goquery.Selection) {
		container := s.Parent()
		container.Find("table a, .wp-block-table a").Each(func(j int, link *goquery.Selection) {
			href, exists := link.Attr("href")
			if exists && strings.Contains(href, "/reference/classes/") {
				addMethodURL(href)
			}
		})
	})

	// Fallback: scan all table links for method references
	if len(methodURLs) == 0 {
		entitySlug := entity.Slug
		doc.Find("table a").Each(func(j int, link *goquery.Selection) {
			href, exists := link.Attr("href")
			if exists && strings.Contains(href, "/reference/classes/"+entitySlug+"/") {
				if !strings.HasPrefix(href, "http") {
					href = BaseURL + href
				}
				if href != url && !strings.HasSuffix(href, entitySlug+"/") {
					addMethodURL(href)
				}
			}
		})
	}

	return entity, methodURLs, nil
}

// slugFromURL derives a unique slug from the URL path.
// e.g. "https://developer.wordpress.org/reference/classes/wp_query/" → "wp_query"
func slugFromURL(url string) string {
	path := strings.TrimSuffix(strings.TrimPrefix(url, BaseURL+"/reference/classes/"), "/")
	// Handle nested paths (e.g. "requests/transport/curl" → "requests-transport-curl")
	path = strings.ReplaceAll(path, "/", "-")
	return strings.ToLower(path)
}

// cleanClassName strips "class " prefix and " {}" suffix from h1 text.
func cleanClassName(h1 string) string {
	name := h1
	name = strings.TrimPrefix(name, "class ")
	name = strings.TrimPrefix(name, "Class ")
	name = strings.TrimSuffix(name, " {}")
	name = strings.TrimSuffix(name, "{}")
	return strings.TrimSpace(name)
}

// extractEntityDescription gets description from meta tag or pre-h2 paragraphs.
func extractEntityDescription(doc *goquery.Document) string {
	// Prefer meta description for clean text
	if meta := doc.Find("meta[name=description]").AttrOr("content", ""); meta != "" {
		return strings.TrimSpace(meta)
	}

	// Fallback: paragraphs before the first h2
	var descParts []string
	doc.Find("article .wp-block-post-content, main section").First().Children().Each(func(i int, s *goquery.Selection) {
		if goquery.NodeName(s) == "h2" {
			return // stop at first heading
		}
		if goquery.NodeName(s) == "p" {
			text := strings.TrimSpace(s.Text())
			if text != "" && !isNavigationText(text) && len(descParts) < 3 {
				descParts = append(descParts, text)
			}
		}
	})
	return strings.Join(descParts, " ")
}

// extractProperties finds properties by heading ID, heading text, or WordPress-specific tables.
func extractProperties(doc *goquery.Document, entity *source.Entity) {
	extractPropsFromContainer := func(container *goquery.Selection) {
		container.Find("dt, tr").Each(func(j int, row *goquery.Selection) {
			prop := source.Property{}
			prop.Name = strings.TrimSpace(row.Find("code").First().Text())
			if prop.Name == "" {
				prop.Name = strings.TrimSpace(row.Find("td").First().Text())
			}
			dd := row.Next()
			if dd.Is("dd") {
				prop.Description = strings.TrimSpace(dd.Text())
			} else {
				prop.Description = strings.TrimSpace(row.Find("td").Last().Text())
			}
			if prop.Name != "" {
				entity.Properties = append(entity.Properties, prop)
			}
		})
	}

	// Strategy 1: Find by heading ID
	doc.Find("h2#properties, h2#members").Each(func(i int, s *goquery.Selection) {
		extractPropsFromContainer(s.Parent())
	})

	// Strategy 2: Find h2 by text content
	if len(entity.Properties) == 0 {
		doc.Find("h2").Each(func(i int, s *goquery.Selection) {
			text := strings.ToLower(strings.TrimSpace(s.Text()))
			if strings.Contains(text, "properties") || strings.Contains(text, "members") {
				extractPropsFromContainer(s.Parent())
			}
		})
	}

	// Strategy 3: WordPress-specific table class
	if len(entity.Properties) == 0 {
		doc.Find(".wp-block-wporg-code-reference-table table").Each(func(i int, table *goquery.Selection) {
			// Check if preceding heading mentions properties/members
			prev := table.Parent().Prev()
			headingText := strings.ToLower(strings.TrimSpace(prev.Text()))
			if !strings.Contains(headingText, "propert") && !strings.Contains(headingText, "member") {
				return
			}
			table.Find("tr").Each(func(j int, tr *goquery.Selection) {
				tds := tr.Find("td")
				if tds.Length() < 2 {
					return
				}
				prop := source.Property{}
				prop.Name = strings.TrimSpace(tds.Eq(0).Find("code").Text())
				if prop.Name == "" {
					prop.Name = strings.TrimSpace(tds.Eq(0).Text())
				}
				prop.Description = strings.TrimSpace(tds.Eq(1).Text())
				if prop.Name != "" {
					entity.Properties = append(entity.Properties, prop)
				}
			})
		})
	}

	// Strategy 4: Parse $-prefixed variables from source code
	if len(entity.Properties) == 0 && entity.SourceCode != "" {
		for _, line := range strings.Split(entity.SourceCode, "\n") {
			trimmed := strings.TrimSpace(line)
			// Match lines like: public $post_type = 'post';
			// or: var $query_vars = array();
			for _, prefix := range []string{"public ", "protected ", "private ", "var "} {
				if !strings.HasPrefix(trimmed, prefix) {
					continue
				}
				rest := strings.TrimPrefix(trimmed, prefix)
				// Handle static keyword
				rest = strings.TrimPrefix(rest, "static ")
				if !strings.HasPrefix(rest, "$") {
					continue
				}
				// Extract variable name
				name := rest[1:] // strip $
				for i, ch := range name {
					if ch == ' ' || ch == '=' || ch == ';' {
						name = name[:i]
						break
					}
				}
				if name != "" {
					prop := source.Property{Name: "$" + name}
					entity.Properties = append(entity.Properties, prop)
				}
			}
		}
	}
}

// isNavigationText returns true if the text is WordPress navigation/boilerplate.
func isNavigationText(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "view all references") ||
		strings.Contains(lower, "view on trac") ||
		strings.Contains(lower, "view on github") ||
		strings.Contains(lower, "you must log in") ||
		strings.Contains(lower, "contribute a note") ||
		strings.Contains(lower, "user contributed notes")
}

// extractSourceCodeAfterHeading finds the source code block following a heading ID.
func extractSourceCodeAfterHeading(doc *goquery.Document, headingID string) string {
	var code string
	selector := fmt.Sprintf("h2#%s", headingID)
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		container := s.Parent()
		container.Find("pre code, code.language-php").Each(func(j int, codeEl *goquery.Selection) {
			text := strings.TrimSpace(codeEl.Text())
			if len(text) > len(code) {
				code = text
			}
		})
	})
	return code
}
