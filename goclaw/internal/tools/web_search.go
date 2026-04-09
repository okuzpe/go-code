package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	braveSearchAPIURL  = "https://api.search.brave.com/res/v1/web/search"
	serpAPISearchURL   = "https://serpapi.com/search.json"
	defaultDDGAPIBase  = "https://api.duckduckgo.com"
	webSearchUserAgent = "goclaw/0.1 (+https://github.com/okuzpe/goclaw)"
)

// WebSearchOptions configures the web_search tool (backends and API keys).
type WebSearchOptions struct {
	// Backend is "ddg", "brave", or "serpapi".
	Backend     string
	BraveAPIKey string
	SerpAPIKey  string
	// FallbackDDG retries via DuckDuckGo when the primary backend fails or returns no usable results.
	FallbackDDG bool
	// DDGAPIBase overrides the DuckDuckGo API base URL (for tests).
	DDGAPIBase string
	// BraveEndpoint overrides the Brave Search API URL (for tests).
	BraveEndpoint string
	// SerpAPIEndpoint overrides the SerpAPI search URL (for tests).
	SerpAPIEndpoint string
	// DDGHTMLEndpoint overrides the DuckDuckGo HTML search POST URL (for tests). Empty uses production html.duckduckgo.com.
	DDGHTMLEndpoint string
}

// WebSearchTool queries web search backends (DuckDuckGo, Brave Search API, or SerpAPI).
type WebSearchTool struct {
	client *http.Client

	backend         string
	braveAPIKey     string
	serpAPIKey      string
	fallbackDDG     bool
	ddgAPIBase      string
	braveEndpoint   string
	serpAPIEndpoint string
	ddgHTMLEndpoint string
}

// NewWebSearch returns a web_search tool with the given options.
func NewWebSearch(opts WebSearchOptions) *WebSearchTool {
	backend := strings.ToLower(strings.TrimSpace(opts.Backend))
	if backend == "" {
		backend = "ddg"
	}
	ddgBase := strings.TrimSpace(opts.DDGAPIBase)
	if ddgBase == "" {
		ddgBase = defaultDDGAPIBase
	}
	return &WebSearchTool{
		client:          &http.Client{Timeout: WebSearchTimeoutSec * time.Second},
		backend:         backend,
		braveAPIKey:     strings.TrimSpace(opts.BraveAPIKey),
		serpAPIKey:      strings.TrimSpace(opts.SerpAPIKey),
		fallbackDDG:     opts.FallbackDDG,
		ddgAPIBase:      ddgBase,
		braveEndpoint:   strings.TrimSpace(opts.BraveEndpoint),
		serpAPIEndpoint: strings.TrimSpace(opts.SerpAPIEndpoint),
		ddgHTMLEndpoint: strings.TrimSpace(opts.DDGHTMLEndpoint),
	}
}

var _ Tool = (*WebSearchTool)(nil)

func (WebSearchTool) Name() string { return "web_search" }

func (WebSearchTool) Description() string {
	return "Search the web using the configured backend (DuckDuckGo, Brave Search API, or SerpAPI; see web_search_backend in settings). " +
		"For DuckDuckGo, the instant-answer JSON API is tried first; when it returns no hits, an HTML result list is fetched so titles and URLs are available. " +
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

	switch t.backend {
	case "ddg":
		return t.searchDuckDuckGo(ctx, q), nil
	case "brave":
		res := t.searchBrave(ctx, q)
		if t.shouldFallbackToDDG(res) {
			return t.searchDuckDuckGo(ctx, q), nil
		}
		return res, nil
	case "serpapi":
		res := t.searchSerpAPI(ctx, q)
		if t.shouldFallbackToDDG(res) {
			return t.searchDuckDuckGo(ctx, q), nil
		}
		return res, nil
	default:
		return t.searchDuckDuckGo(ctx, q), nil
	}
}

func (t *WebSearchTool) shouldFallbackToDDG(res Result) bool {
	if !t.fallbackDDG {
		return false
	}
	if res.IsError {
		return true
	}
	return strings.TrimSpace(res.Content) == ""
}

func (t *WebSearchTool) searchBrave(ctx context.Context, q string) Result {
	if t.braveAPIKey == "" {
		return Result{
			Content: "brave search backend requires brave_search_api_key in settings or BRAVE_SEARCH_API_KEY in the environment",
			IsError: true,
		}
	}
	braveURL := t.braveEndpoint
	if braveURL == "" {
		braveURL = braveSearchAPIURL
	}
	u, err := url.Parse(braveURL)
	if err != nil {
		return Result{Content: "internal url error", IsError: true}
	}
	qry := u.Query()
	qry.Set("q", q)
	qry.Set("count", strconv.Itoa(MaxWebSearchResults))
	u.RawQuery = qry.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{Content: fmt.Sprintf("build request: %v", err), IsError: true}
	}
	req.Header.Set("User-Agent", webSearchUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.braveAPIKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return Result{Content: fmt.Sprintf("brave search: %v", err), IsError: true}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return Result{Content: fmt.Sprintf("read response: %v", err), IsError: true}
	}
	if resp.StatusCode != http.StatusOK {
		return Result{Content: fmt.Sprintf("brave search http %d: %s", resp.StatusCode, trimSnippet(string(raw), 512)), IsError: true}
	}

	var body struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				URL         string `json:"url"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return Result{Content: fmt.Sprintf("parse brave results: %v", err), IsError: true}
	}

	var b strings.Builder
	n := 0
	for _, r := range body.Web.Results {
		if n >= MaxWebSearchResults {
			break
		}
		line := strings.TrimSpace(r.Title)
		if r.Description != "" {
			if line != "" {
				line += " — "
			}
			line += strings.TrimSpace(r.Description)
		}
		writeTopic(&b, stripHTMLTags(line), r.URL, &n)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return Result{Content: "", IsError: false}
	}
	return Result{Content: out, IsError: false}
}

func (t *WebSearchTool) searchSerpAPI(ctx context.Context, q string) Result {
	if t.serpAPIKey == "" {
		return Result{
			Content: "serpapi backend requires serpapi_api_key in settings or SERPAPI_API_KEY in the environment",
			IsError: true,
		}
	}
	serpURL := t.serpAPIEndpoint
	if serpURL == "" {
		serpURL = serpAPISearchURL
	}
	u, err := url.Parse(serpURL)
	if err != nil {
		return Result{Content: "internal url error", IsError: true}
	}
	qry := u.Query()
	qry.Set("engine", "google")
	qry.Set("q", q)
	qry.Set("api_key", t.serpAPIKey)
	qry.Set("num", strconv.Itoa(MaxWebSearchResults))
	u.RawQuery = qry.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{Content: fmt.Sprintf("build request: %v", err), IsError: true}
	}
	req.Header.Set("User-Agent", webSearchUserAgent)

	resp, err := t.client.Do(req)
	if err != nil {
		return Result{Content: fmt.Sprintf("serpapi search: %v", err), IsError: true}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return Result{Content: fmt.Sprintf("read response: %v", err), IsError: true}
	}
	if resp.StatusCode != http.StatusOK {
		return Result{Content: fmt.Sprintf("serpapi search http %d: %s", resp.StatusCode, trimSnippet(string(raw), 512)), IsError: true}
	}

	var body struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Snippet string `json:"snippet"`
			Link    string `json:"link"`
		} `json:"organic_results"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return Result{Content: fmt.Sprintf("parse serpapi results: %v", err), IsError: true}
	}

	var b strings.Builder
	n := 0
	for _, r := range body.OrganicResults {
		if n >= MaxWebSearchResults {
			break
		}
		line := strings.TrimSpace(r.Title)
		if r.Snippet != "" {
			if line != "" {
				line += " — "
			}
			line += strings.TrimSpace(r.Snippet)
		}
		writeTopic(&b, stripHTMLTags(line), r.Link, &n)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return Result{Content: "", IsError: false}
	}
	return Result{Content: out, IsError: false}
}

func (t *WebSearchTool) searchDuckDuckGo(ctx context.Context, q string) Result {
	base := t.ddgAPIBase
	if base == "" {
		base = defaultDDGAPIBase
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return Result{Content: "internal url error", IsError: true}
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
		return Result{Content: fmt.Sprintf("build request: %v", err), IsError: true}
	}
	req.Header.Set("User-Agent", webSearchUserAgent)

	resp, err := t.client.Do(req)
	if err != nil {
		return Result{Content: fmt.Sprintf("search: %v", err), IsError: true}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{Content: fmt.Sprintf("search http %d", resp.StatusCode), IsError: true}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return Result{Content: fmt.Sprintf("read response: %v", err), IsError: true}
	}

	var ddg struct {
		Abstract      string            `json:"Abstract"`
		AbstractURL   string            `json:"AbstractURL"`
		Answer        string            `json:"Answer"`
		Definition    string            `json:"Definition"`
		DefinitionURL string            `json:"DefinitionURL"`
		Heading       string            `json:"Heading"`
		RelatedTopics []json.RawMessage `json:"RelatedTopics"`
		Results       []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(raw, &ddg); err != nil {
		return Result{Content: fmt.Sprintf("parse results: %v", err), IsError: true}
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
		if htmlHits := t.searchDuckDuckGoHTML(ctx, q); htmlHits != "" {
			out = htmlHits
		} else {
			searchURL := "https://duckduckgo.com/?q=" + url.QueryEscape(q)
			out = "no instant answer, definitions, or result links from DuckDuckGo for this query. " +
				"Try a shorter or more specific query, use web_fetch on a documentation URL you already know, " +
				"or open a full search: " + searchURL
		}
	}
	return Result{Content: out, IsError: false}
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
