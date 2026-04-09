package tools

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// defaultDDGHTMLSearchURL is DuckDuckGo's HTML search form (full result list). The JSON instant-answer API
// often returns nothing for open-ended queries (e.g. news); this endpoint supplies titles and links.
const defaultDDGHTMLSearchURL = "https://html.duckduckgo.com/html/"

// searchDuckDuckGoHTML runs a POST to DuckDuckGo's HTML search and formats up to MaxWebSearchResults hits.
// Returns empty string on transport errors, non-OK status, or when no result anchors are found.
func (t *WebSearchTool) searchDuckDuckGoHTML(ctx context.Context, q string) string {
	endpoint := strings.TrimSpace(t.ddgHTMLEndpoint)
	if endpoint == "" {
		endpoint = defaultDDGHTMLSearchURL
	}
	form := url.Values{}
	form.Set("q", q)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", webSearchUserAgent)

	resp, err := t.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return ""
	}
	hits := parseDuckDuckGoHTMLResults(raw, MaxWebSearchResults)
	if len(hits) == 0 {
		return ""
	}
	var b strings.Builder
	n := 0
	for _, hit := range hits {
		if n >= MaxWebSearchResults {
			break
		}
		title := stripHTMLTags(strings.TrimSpace(hit.title))
		if title == "" {
			continue
		}
		snip := stripHTMLTags(strings.TrimSpace(hit.snippet))
		block := trimSnippet(title, MaxSearchSnippet)
		if snip != "" && !textRedundantWithTitle(title, snip) {
			block += "\n" + trimSnippet(snip, MaxSearchSnippet)
		}
		if len(block) > MaxSearchSnippet {
			block = trimSnippet(block, MaxSearchSnippet)
		}
		u := strings.TrimSpace(hit.href)
		writeTopic(&b, block, u, &n)
	}
	return strings.TrimSpace(b.String())
}

type ddgHTMLHit struct {
	href    string
	title   string
	snippet string
}

// parseDuckDuckGoHTMLResults prefers structured result blocks (title + snippet); falls back to result__a-only markup.
func parseDuckDuckGoHTMLResults(body []byte, max int) []ddgHTMLHit {
	hits := parseDuckDuckGoHTMLResultBodies(body, max)
	if len(hits) > 0 {
		return hits
	}
	return parseDuckDuckGoHTMLResultAnchors(body, max)
}

func parseDuckDuckGoHTMLResultBodies(body []byte, max int) []ddgHTMLHit {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var out []ddgHTMLHit
	seen := make(map[string]struct{})
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(out) >= max {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" {
			if cls := htmlAttribute(n, "class"); strings.Contains(cls, "result__body") {
				hit := extractHitFromResultBody(n)
				hit.href = normalizeDuckDuckGoResultHref(hit.href)
				if hit.title != "" && hit.href != "" {
					if _, dup := seen[hit.href]; !dup {
						seen[hit.href] = struct{}{}
						out = append(out, hit)
					}
				}
				if len(out) >= max {
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

func extractHitFromResultBody(root *html.Node) ddgHTMLHit {
	var hit ddgHTMLHit
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			cls := htmlAttribute(n, "class")
			href := strings.TrimSpace(htmlAttribute(n, "href"))
			if strings.Contains(cls, "result__a") {
				if t := anchorInnerText(n); t != "" {
					hit.title = t
				}
				if href != "" {
					hit.href = href
				}
			}
			if strings.Contains(cls, "result__snippet") {
				if s := anchorInnerText(n); s != "" && hit.snippet == "" {
					hit.snippet = s
				}
				if hit.href == "" && href != "" {
					hit.href = href
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return hit
}

func textRedundantWithTitle(title, snippet string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	s := strings.ToLower(strings.TrimSpace(snippet))
	if t == "" || s == "" {
		return true
	}
	if s == t {
		return true
	}
	if strings.HasPrefix(s, t) || strings.HasPrefix(t, s) {
		return true
	}
	return false
}

func parseDuckDuckGoHTMLResultAnchors(body []byte, max int) []ddgHTMLHit {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var out []ddgHTMLHit
	seen := make(map[string]struct{})
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			if class := htmlAttribute(n, "class"); class != "" && strings.Contains(class, "result__a") {
				href := htmlAttribute(n, "href")
				title := anchorInnerText(n)
				href = normalizeDuckDuckGoResultHref(href)
				if title != "" && href != "" {
					if _, ok := seen[href]; ok {
						// skip duplicates
					} else {
						seen[href] = struct{}{}
						out = append(out, ddgHTMLHit{href: href, title: title})
						if len(out) >= max {
							return
						}
					}
				}
			}
		}
		if len(out) >= max {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

func htmlAttribute(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func anchorInnerText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	return strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
}

func normalizeDuckDuckGoResultHref(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	if strings.HasPrefix(h, "//") {
		return "https:" + h
	}
	if strings.HasPrefix(h, "/") && !strings.HasPrefix(h, "//") {
		return "https://duckduckgo.com" + h
	}
	u, err := url.Parse(h)
	if err != nil {
		return h
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	return u.String()
}
