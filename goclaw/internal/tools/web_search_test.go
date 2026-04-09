package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStripHTMLTags(t *testing.T) {
	in := `<a href="x">Hello</a> <b>world</b>`
	got := stripHTMLTags(in)
	if got != "Hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestWebSearchExecuteMockServer(t *testing.T) {
	payload := map[string]any{
		"Answer": "instant",
		"Results": []map[string]any{
			{"Text": `<a href="https://example.com">Example</a> title`, "FirstURL": "https://example.com/"},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "testquery" {
			t.Fatalf("q=%q", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	tool := NewWebSearch(WebSearchOptions{
		Backend:    "ddg",
		DDGAPIBase: srv.URL,
	})
	res, err := tool.Execute(context.Background(), `{"query":"testquery"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "instant") {
		t.Fatalf("missing answer: %q", res.Content)
	}
	if !strings.Contains(res.Content, "Example") || strings.Contains(res.Content, "<a") {
		t.Fatalf("expected stripped HTML in results: %q", res.Content)
	}
	if !strings.Contains(res.Content, "https://example.com/") {
		t.Fatalf("missing url: %q", res.Content)
	}
}

func TestWebSearchEmptyFallbackIncludesSearchURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Abstract":"","RelatedTopics":null,"Results":null}`))
	}))
	defer srv.Close()
	htmlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("html fallback: want POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer htmlSrv.Close()

	tool := NewWebSearch(WebSearchOptions{
		Backend:         "ddg",
		DDGAPIBase:      srv.URL,
		DDGHTMLEndpoint: htmlSrv.URL,
	})
	res, err := tool.Execute(context.Background(), `{"query":"obscure xyz query"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal(res.Content)
	}
	if !strings.Contains(res.Content, "duckduckgo.com") {
		t.Fatalf("expected fallback URL: %q", res.Content)
	}
	if !strings.Contains(res.Content, "obscure") {
		t.Fatalf("expected query echoed in URL: %q", res.Content)
	}
	if !strings.Contains(res.Content, "no instant answer") {
		t.Fatalf("expected thin-response hint: %q", res.Content)
	}
}

func TestWebSearchEmptyResultsArrayFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Abstract":"","Answer":"","Definition":"","Results":[],"RelatedTopics":[]}`))
	}))
	defer srv.Close()
	htmlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer htmlSrv.Close()

	tool := NewWebSearch(WebSearchOptions{
		Backend:         "ddg",
		DDGAPIBase:      srv.URL,
		DDGHTMLEndpoint: htmlSrv.URL,
	})
	res, err := tool.Execute(context.Background(), `{"query":"zzz empty hits"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal(res.Content)
	}
	if !strings.Contains(res.Content, "duckduckgo.com") {
		t.Fatalf("expected fallback search link: %q", res.Content)
	}
	if !strings.Contains(res.Content, "zzz+empty+hits") && !strings.Contains(res.Content, "zzz") {
		t.Fatalf("expected query in fallback URL: %q", res.Content)
	}
}

func TestWebSearchDDGHTMLFallbackExtractsTitles(t *testing.T) {
	jsonSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Abstract":"","Answer":"","Results":[],"RelatedTopics":[]}`))
	}))
	defer jsonSrv.Close()
	htmlPage := `<!DOCTYPE html><html><body>` +
		`<a class="result__a" href="//example.com/news/1">First Spanish Headline</a>` +
		`<a class="result__a" href="https://example.com/two">Second Headline Here</a>` +
		`</body></html>`
	htmlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("want POST, got %s", r.Method)
		}
		_ = r.ParseForm()
		if r.FormValue("q") != "noticias de hoy" {
			t.Fatalf("q=%q", r.FormValue("q"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(htmlPage))
	}))
	defer htmlSrv.Close()

	tool := NewWebSearch(WebSearchOptions{
		Backend:         "ddg",
		DDGAPIBase:      jsonSrv.URL,
		DDGHTMLEndpoint: htmlSrv.URL,
	})
	res, err := tool.Execute(context.Background(), `{"query":"noticias de hoy"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal(res.Content)
	}
	if !strings.Contains(res.Content, "First Spanish Headline") {
		t.Fatalf("missing first title: %q", res.Content)
	}
	if !strings.Contains(res.Content, "Second Headline Here") {
		t.Fatalf("missing second title: %q", res.Content)
	}
	if !strings.Contains(res.Content, "https://example.com/news/1") && !strings.Contains(res.Content, "example.com/news/1") {
		t.Fatalf("missing first url: %q", res.Content)
	}
	if strings.Contains(res.Content, "no instant answer") {
		t.Fatalf("should use HTML hits, not thin fallback: %q", res.Content)
	}
}

func TestWebSearchDDGHTMLFallbackIncludesSnippetFromResultBody(t *testing.T) {
	jsonSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Abstract":"","Results":[],"RelatedTopics":[]}`))
	}))
	defer jsonSrv.Close()
	htmlPage := `<div class="links_main links_deep result__body">` +
		`<h2 class="result__title"><a class="result__a" href="https://news.example/top">Main headline from SERP</a></h2>` +
		`<a class="result__snippet" href="https://news.example/top">Descriptive snippet with <b>keywords</b> for context.</a>` +
		`</div>`
	htmlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(htmlPage))
	}))
	defer htmlSrv.Close()

	tool := NewWebSearch(WebSearchOptions{
		Backend:         "ddg",
		DDGAPIBase:      jsonSrv.URL,
		DDGHTMLEndpoint: htmlSrv.URL,
	})
	res, err := tool.Execute(context.Background(), `{"query":"q"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal(res.Content)
	}
	if !strings.Contains(res.Content, "Main headline from SERP") {
		t.Fatalf("missing title: %q", res.Content)
	}
	if !strings.Contains(res.Content, "Descriptive snippet") || !strings.Contains(res.Content, "keywords") {
		t.Fatalf("expected snippet in tool output: %q", res.Content)
	}
}

func TestWebSearchBraveSuccess(t *testing.T) {
	braveBody := `{"web":{"results":[{"title":"Brave hit","description":"snippet text","url":"https://brave.example/hit"}]}}`
	braveSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Subscription-Token") != "test-token" {
			t.Fatalf("missing subscription token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(braveBody))
	}))
	defer braveSrv.Close()

	tool := NewWebSearch(WebSearchOptions{
		Backend:       "brave",
		BraveAPIKey:   "test-token",
		FallbackDDG:   false,
		BraveEndpoint: braveSrv.URL + "/res/v1/web/search",
	})
	res, err := tool.Execute(context.Background(), `{"query":"q1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal(res.Content)
	}
	if !strings.Contains(res.Content, "Brave hit") || !strings.Contains(res.Content, "brave.example") {
		t.Fatalf("unexpected content: %q", res.Content)
	}
}

func TestWebSearchBraveEmptyFallsBackToDDG(t *testing.T) {
	ddgPayload := map[string]any{"Answer": "from ddg"}
	ddgRaw, _ := json.Marshal(ddgPayload)
	ddgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(ddgRaw)
	}))
	defer ddgSrv.Close()

	braveSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"web":{"results":[]}}`))
	}))
	defer braveSrv.Close()

	tool := NewWebSearch(WebSearchOptions{
		Backend:       "brave",
		BraveAPIKey:   "k",
		FallbackDDG:   true,
		BraveEndpoint: braveSrv.URL + "/res/v1/web/search",
		DDGAPIBase:    ddgSrv.URL,
	})
	res, err := tool.Execute(context.Background(), `{"query":"fallbackq"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal(res.Content)
	}
	if !strings.Contains(res.Content, "from ddg") {
		t.Fatalf("expected DDG fallback content: %q", res.Content)
	}
}

func TestWebSearchSerpAPISuccess(t *testing.T) {
	serpBody := `{"organic_results":[{"title":"G hit","snippet":"desc","link":"https://google.example/x"}]}`
	serpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "serp-key" || r.URL.Query().Get("engine") != "google" {
			t.Fatalf("bad query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(serpBody))
	}))
	defer serpSrv.Close()

	tool := NewWebSearch(WebSearchOptions{
		Backend:         "serpapi",
		SerpAPIKey:      "serp-key",
		FallbackDDG:     false,
		SerpAPIEndpoint: serpSrv.URL,
	})
	res, err := tool.Execute(context.Background(), `{"query":"findme"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal(res.Content)
	}
	if !strings.Contains(res.Content, "G hit") || !strings.Contains(res.Content, "google.example") {
		t.Fatalf("unexpected content: %q", res.Content)
	}
}

func TestWebSearchBraveMissingKeyNoFallback(t *testing.T) {
	tool := NewWebSearch(WebSearchOptions{
		Backend:     "brave",
		FallbackDDG: false,
	})
	res, err := tool.Execute(context.Background(), `{"query":"x"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(res.Content, "brave_search_api_key") {
		t.Fatalf("want error about api key, got %#v", res)
	}
}
