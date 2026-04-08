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

	tool := &WebSearchTool{
		client:  srv.Client(),
		apiBase: srv.URL,
	}
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

	tool := &WebSearchTool{
		client:  srv.Client(),
		apiBase: srv.URL,
	}
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
}
