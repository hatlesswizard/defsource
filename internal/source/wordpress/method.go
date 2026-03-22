package wordpress

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/hatlesswizard/defsource/internal/source"
)

func (w *WordPressSource) ParseMethod(_ context.Context, url string, body []byte) (*source.Method, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	method := &source.Method{
		URL: url,
	}

	// Extract signature from <h1>
	h1 := strings.TrimSpace(doc.Find("article h1, main h1").First().Text())
	if h1 == "" {
		return nil, fmt.Errorf("no h1 found on method page %s", url)
	}
	method.Signature = h1
	method.Name = extractMethodName(h1)
	method.Slug = strings.ToLower(method.Name)

	// Extract description: prefer meta description, then h2#description, then first content paragraph
	if meta := doc.Find("meta[name=description]").AttrOr("content", ""); meta != "" {
		trimmed := strings.TrimSpace(meta)
		if !isNavigationText(trimmed) {
			method.Description = trimmed
		}
	}
	if method.Description == "" {
		var descParts []string
		doc.Find("h2#description").Each(func(i int, s *goquery.Selection) {
			s.Parent().Find("p").Each(func(j int, p *goquery.Selection) {
				text := strings.TrimSpace(p.Text())
				if text != "" && !isNavigationText(text) {
					descParts = append(descParts, text)
				}
			})
		})
		method.Description = strings.Join(descParts, " ")
	}
	if method.Description == "" {
		doc.Find("article .wp-block-post-content > p, main section > p").Each(func(i int, s *goquery.Selection) {
			if i == 0 {
				text := strings.TrimSpace(s.Text())
				if text != "" && !isNavigationText(text) {
					method.Description = text
				}
			}
		})
	}

	// Extract parameters
	method.Parameters = parseParameters(doc)

	// Extract return type and description
	method.ReturnType, method.ReturnDesc = parseReturn(doc)

	// Extract source code with fallback selectors
	method.SourceCode = extractSourceCodeAfterHeading(doc, "source")
	if method.SourceCode == "" {
		if codeEl := doc.Find(".wp-block-wporg-code-reference-source pre code").First(); codeEl.Length() > 0 {
			method.SourceCode = strings.TrimSpace(codeEl.Text())
		}
	}
	if method.SourceCode == "" {
		if codeEl := doc.Find("section pre code").First(); codeEl.Length() > 0 {
			method.SourceCode = strings.TrimSpace(codeEl.Text())
		}
	}

	// Extract "since" version from changelog
	method.Since = parseChangelog(doc)

	// Extract relations (uses / used_by)
	method.Relations = parseRelations(doc)

	return method, nil
}

// extractMethodName extracts "query" from "WP_Query::query( string|array $query ): WP_Post[]|int[]".
func extractMethodName(signature string) string {
	if _, after, ok := strings.Cut(signature, "::"); ok {
		for i, ch := range after {
			if ch == '(' || ch == ' ' {
				return after[:i]
			}
		}
		return after
	}
	if before, _, ok := strings.Cut(signature, "("); ok {
		parts := strings.Fields(before)
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return signature
}

// parseParameters extracts parameters from the #parameters section.
func parseParameters(doc *goquery.Document) []source.Parameter {
	var params []source.Parameter

	doc.Find("h2#parameters").Each(func(i int, heading *goquery.Selection) {
		container := heading.Parent()

		// WordPress uses <dl>/<dt>/<dd> structure for parameters
		container.Find("dt").Each(func(j int, dt *goquery.Selection) {
			param := source.Parameter{}

			param.Name = strings.TrimSpace(dt.Find("code").First().Text())

			dtText := strings.TrimSpace(dt.Text())
			remaining := strings.TrimPrefix(dtText, param.Name)
			remaining = strings.TrimSpace(remaining)

			lowerRemaining := strings.ToLower(remaining)
			if strings.Contains(lowerRemaining, "required") {
				param.Required = true
				remaining = remaining[:strings.Index(lowerRemaining, "required")] + remaining[strings.Index(lowerRemaining, "required")+8:]
			} else if strings.Contains(lowerRemaining, "optional") {
				remaining = remaining[:strings.Index(lowerRemaining, "optional")] + remaining[strings.Index(lowerRemaining, "optional")+8:]
			}

			param.Type = strings.TrimSpace(remaining)

			dd := dt.Next()
			if dd.Length() > 0 {
				param.Description = strings.TrimSpace(dd.Text())
			}

			if param.Name != "" {
				params = append(params, param)
			}
		})

		// Fallback: table-based parameters
		if len(params) == 0 {
			container.Find("table tr").Each(func(j int, tr *goquery.Selection) {
				tds := tr.Find("td")
				if tds.Length() >= 2 {
					param := source.Parameter{}
					param.Name = strings.TrimSpace(tds.Eq(0).Find("code").Text())
					param.Type = strings.TrimSpace(tds.Eq(0).Text())
					param.Type = strings.TrimPrefix(param.Type, param.Name)
					param.Type = strings.TrimSpace(param.Type)
					param.Description = strings.TrimSpace(tds.Eq(1).Text())
					param.Required = strings.Contains(strings.ToLower(tds.Eq(0).Text()), "required")
					if param.Name != "" {
						params = append(params, param)
					}
				}
			})
		}
	})

	return params
}

// parseReturn extracts return type and description from #return section.
func parseReturn(doc *goquery.Document) (string, string) {
	var retType, retDesc string

	doc.Find("h2#return").Each(func(i int, heading *goquery.Selection) {
		// Search siblings AFTER the heading, not inside it
		siblings := heading.NextAll()

		// Look for return type in code/span elements after the heading
		codeEl := siblings.Find("code, span.return-type").First()
		if codeEl.Length() > 0 {
			retType = strings.TrimSpace(codeEl.Text())
		}

		// Get description from paragraphs after the heading
		siblings.Filter("p, dl, dd").Each(func(j int, el *goquery.Selection) {
			text := strings.TrimSpace(el.Text())
			if text != "" && retDesc == "" {
				// Remove the return type from the beginning if present
				text = strings.TrimPrefix(text, retType)
				text = strings.TrimSpace(text)
				if text != "" {
					retDesc = text
				}
			}
		})

		// Fallback: parse from container text excluding heading
		if retType == "" {
			container := heading.Parent()
			allText := strings.TrimSpace(container.Text())
			headingText := strings.TrimSpace(heading.Text())
			allText = strings.TrimPrefix(allText, headingText)
			allText = strings.TrimSpace(allText)
			lines := strings.SplitN(allText, "\n", 2)
			retType = strings.TrimSpace(lines[0])
			if len(lines) > 1 {
				retDesc = strings.TrimSpace(lines[1])
			}
		}
	})

	return retType, retDesc
}

// parseChangelog extracts the earliest "since" version from the changelog.
func parseChangelog(doc *goquery.Document) string {
	var since string
	doc.Find("h2#changelog").Each(func(i int, heading *goquery.Selection) {
		container := heading.Parent()
		container.Find("table tr").Each(func(j int, tr *goquery.Selection) {
			tds := tr.Find("td")
			if tds.Length() >= 2 {
				desc := strings.TrimSpace(tds.Eq(1).Text())
				if strings.Contains(strings.ToLower(desc), "introduced") {
					version := strings.TrimSpace(tds.Eq(0).Text())
					if version != "" {
						since = version
					}
				}
			}
		})
	})
	return since
}

// parseRelations extracts "Uses" and "Used by" from the #related section.
func parseRelations(doc *goquery.Document) []source.Relation {
	var relations []source.Relation

	doc.Find("h2#related").Each(func(i int, heading *goquery.Selection) {
		container := heading.Parent()

		container.Find("table").Each(func(j int, table *goquery.Selection) {
			headerText := strings.TrimSpace(table.Find("th").First().Text())
			kind := "uses"
			if strings.Contains(strings.ToLower(headerText), "used by") {
				kind = "used_by"
			}

			table.Find("tbody tr").Each(func(k int, tr *goquery.Selection) {
				tds := tr.Find("td")
				if tds.Length() < 2 {
					return
				}

				rel := source.Relation{Kind: kind}
				link := tds.Eq(0).Find("a").First()
				rel.TargetName = strings.TrimSpace(link.Text())
				rel.TargetURL, _ = link.Attr("href")
				if rel.TargetURL != "" && !strings.HasPrefix(rel.TargetURL, "http") {
					rel.TargetURL = BaseURL + rel.TargetURL
				}
				rel.Description = strings.TrimSpace(tds.Eq(1).Text())

				if rel.TargetName != "" {
					relations = append(relations, rel)
				}
			})
		})
	})

	return relations
}

// ParseSourceCode extracts just the source code from a page.
func (w *WordPressSource) ParseSourceCode(body []byte) (string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("parse HTML: %w", err)
	}
	code := extractSourceCodeAfterHeading(doc, "source")
	if code == "" {
		return "", fmt.Errorf("no source code found on page")
	}
	return code, nil
}
