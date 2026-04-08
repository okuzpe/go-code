package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// WebSearchTool queries DuckDuckGo's public JSON endpoint (no API key).
type WebSearchTool struct {
	client  *http.Client
	apiBase string // scheme + host + optional path prefix; default https://api.duckduckgo.com
}

// NewWebSearch returns a web_search tool.
func NewWebSearch() *WebSearchTool {
	return &WebSearchTool{
		client:  &http.Client{Timeout: WebSearchTimeoutSec * time.Second},
		apiBase: "https://api.duckduckgo.com",
	}
}

var _ Tool = (*WebSearchTool)(nil)

func (WebSearchTool) Name() string { return "web_search" }

func (WebSearchTool) Description() string {
	return "Search the web via DuckDuckGo (JSON API: instant answer, definitions, Results list, related topics when available). " +
		"Do not use for tasks you can solve with a structured plan from general knowledge alone. " +
		"Prefer this tool for external documentation, version facts, or APIs you cannot recall reliably. " +
		"If the response is thin, use a more specific query, try web_fetch on a known URL, or follow the suggested search link in the tool output."
}

func (WebSearchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query",
			},
		},
		"required": []string{"query"},
	}
}

type webSearchInput struct {
	Query string `json:"query"`
}

// Execute implements Tool.
func (t *WebSearchTool) Execute(ctx context.Context, input string) (Result, error) {
	var in webSearchInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: "", IsError: true}, fmt.Errorf("invalid json input: %w", err)
	}
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return Result{Content: "query is required", IsError: true}, nil
	}

	base := t.apiBase
	if base == "" {
		base = "https://api.duckduckgo.com"
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return Result{Content: "internal url error", IsError: true}, nil
	}
	if endpoint.Path == "" {
		endpoint.Path = "/"
	}
	qry := endpoint.Query()
	qry.Set("q", q)
	qry.Set("format", "json")
	qry.Set("no_html", "1")
	qry.Set("t", "goclaw")
	endpoint.RawQuery = qry.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Result{Content: fmt.Sprintf("build request: %v", err), IsError: true}, nil
	}
	req.Header.Set("User-Agent", "goclaw/0.1 (+https://github.com/okuzpe/goclaw)")

	resp, err := t.client.Do(req)
	if err != nil {
		return Result{Content: fmt.Sprintf("search: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{Content: fmt.Sprintf("search http %d", resp.StatusCode), IsError: true}, nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return Result{Content: fmt.Sprintf("read response: %v", err), IsError: true}, nil
	}

	var ddg struct {
		Abstract      string `json:"Abstract"`
		AbstractURL   string `json:"AbstractURL"`
		Answer        string `json:"Answer"`
		Definition    string `json:"Definition"`
		DefinitionURL string `json:"DefinitionURL"`
		Heading       string `json:"Heading"`
		RelatedTopics []json.RawMessage `json:"RelatedTopics"`
		Results       []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(raw, &ddg); err != nil {
		return Result{Content: fmt.Sprintf("parse results: %v", err), IsError: true}, nil
	}

	var b strings.Builder
	n := 0
	if s := strings.TrimSpace(ddg.Answer); s != "" {
		b.WriteString(trimSnippet(s, MaxSearchSnippet))
		b.WriteString("\n\n")
		n++
	}
	if s := strings.TrimSpace(ddg.Definition); s != "" {
		b.WriteString(trimSnippet(s, MaxSearchSnippet))
		if ddg.DefinitionURL != "" {
			b.WriteString("\n")
			b.WriteString(ddg.DefinitionURL)
		}
		b.WriteString("\n\n")
		n++
	}
	if ddg.Abstract != "" {
		b.WriteString(trimSnippet(ddg.Abstract, MaxSearchSnippet))
		if ddg.AbstractURL != "" {
			b.WriteString("\n")
			b.WriteString(ddg.AbstractURL)
		}
		b.WriteString("\n\n")
		n++
	}
	for _, r := range ddg.Results {
		if n >= MaxWebSearchResults {
			break
		}
		writeTopic(&b, stripHTMLTags(r.Text), r.FirstURL, &n)
	}
	for _, rt := range ddg.RelatedTopics {
		if n >= MaxWebSearchResults {
			break
		}
		var topic struct {
			Text string `json:"Text"`
			URL  string `json:"FirstURL"`
		}
		if err := json.Unmarshal(rt, &topic); err != nil {
			var nested struct {
				Topics []struct {
					Text string `json:"Text"`
					URL  string `json:"FirstURL"`
				} `json:"Topics"`
			}
			if json.Unmarshal(rt, &nested) == nil {
				for _, tt := range nested.Topics {
					if n >= MaxWebSearchResults {
						break
					}
					writeTopic(&b, tt.Text, tt.URL, &n)
				}
			}
			continue
		}
		writeTopic(&b, topic.Text, topic.URL, &n)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		searchURL := "https://duckduckgo.com/?q=" + url.QueryEscape(q)
		out = "no instant answer, definitions, or result links from DuckDuckGo for this query. " +
			"Try a shorter or more specific query, use web_fetch on a documentation URL you already know, " +
			"or open a full search: " + searchURL
	}
	return Result{Content: out, IsError: false}, nil
}

var htmlTagPattern = regexp.MustCompile(`(?i)<[^>]+>`)

func stripHTMLTags(s string) string {
	s = htmlTagPattern.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func writeTopic(b *strings.Builder, text, u string, n *int) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	b.WriteString(trimSnippet(text, MaxSearchSnippet))
	if u != "" {
		b.WriteString("\n")
		b.WriteString(u)
	}
	b.WriteString("\n\n")
	*n++
}

func trimSnippet(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "…"
}
