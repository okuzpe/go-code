package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebFetchTool fetches http(s) URLs with SSRF protections and size limits.
type WebFetchTool struct {
	client *http.Client
}

// NewWebFetch returns a web_fetch tool.
func NewWebFetch() *WebFetchTool {
	return &WebFetchTool{
		client: &http.Client{
			Timeout: WebFetchTimeoutSec * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= MaxRedirects {
					return fmt.Errorf("stopped after %d redirects", MaxRedirects)
				}
				return validateRedirectURL(req.URL)
			},
		},
	}
}

var _ Tool = (*WebFetchTool)(nil)

func (WebFetchTool) Name() string { return "web_fetch" }

func (WebFetchTool) Description() string {
	return "Fetch public http(s) content. HTML responses are reduced to plain text when possible. " +
		"Responses are capped at 1 MiB of text; optional max_bytes is clamped to that cap. " +
		"Up to 5 redirects are followed; each hop is re-checked against SSRF rules (RFC1918 loopback, link-local, metadata IPs blocked). " +
		"Private IPs and metadata endpoints are blocked on the initial URL and after redirects."
}

func (WebFetchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "http or https URL (public hosts only; SSRF-protected; redirects re-validated)",
			},
			"max_bytes": map[string]any{
				"type":        "integer",
				"description": "Maximum response bytes to read (optional; capped at 1 MiB including HTML-to-text extraction)",
			},
		},
		"required": []string{"url"},
	}
}

type webFetchInput struct {
	URL      string `json:"url"`
	MaxBytes int    `json:"max_bytes"`
}

// Execute implements Tool.
func (t *WebFetchTool) Execute(ctx context.Context, input string) (Result, error) {
	var in webFetchInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: "", IsError: true}, fmt.Errorf("invalid json input: %w", err)
	}
	in.URL = strings.TrimSpace(in.URL)
	if in.URL == "" {
		return Result{Content: "url is required", IsError: true}, nil
	}

	u, err := validateHTTPURLForFetch(in.URL)
	if err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}

	maxBytes := in.MaxBytes
	if maxBytes <= 0 || maxBytes > maxWebFetchBytes {
		maxBytes = maxWebFetchBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{Content: fmt.Sprintf("build request: %v", err), IsError: true}, nil
	}
	req.Header.Set("User-Agent", "goclaw/0.1 (+https://github.com/okuzpe/goclaw)")

	resp, err := t.client.Do(req)
	if err != nil {
		return Result{Content: fmt.Sprintf("fetch: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	if err := validateRedirectURL(resp.Request.URL); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{Content: fmt.Sprintf("http %d", resp.StatusCode), IsError: true}, nil
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(strings.ToLower(ct), "text/") &&
		!strings.Contains(strings.ToLower(ct), "json") &&
		!strings.Contains(strings.ToLower(ct), "xml") &&
		!strings.Contains(strings.ToLower(ct), "javascript") {
		return Result{Content: "refusing non-text content-type: " + ct, IsError: true}, nil
	}

	lim := io.LimitReader(resp.Body, int64(maxBytes)+1)
	body, err := io.ReadAll(lim)
	if err != nil {
		return Result{Content: fmt.Sprintf("read body: %v", err), IsError: true}, nil
	}
	truncated := len(body) > maxBytes
	if truncated {
		body = body[:maxBytes]
	}
	out := string(body)
	if isLikelyHTMLResponse(ct, body) {
		if plain, ok := htmlResponseToPlainText(body); ok {
			out = plain
		}
	}
	if truncated {
		out += "\n\n[truncated]"
	}
	return Result{Content: out, IsError: false}, nil
}
