package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicClientCountInputTokens(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages/count_tokens" {
			t.Fatalf("path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"claude-test"`) {
			t.Fatalf("body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"input_tokens": 42}`))
	}))
	defer srv.Close()

	c := NewAnthropic("test-key", srv.URL)
	n, err := c.CountInputTokens(context.Background(), Request{
		Model:    "claude-test",
		System:   "sys",
		Messages: []Message{PlainMessage("user", "hello")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 42 {
		t.Fatalf("got %d", n)
	}
}

func TestAnthropicClientCountInputTokensHTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"bad"}}`))
	}))
	defer srv.Close()

	c := NewAnthropic("k", srv.URL)
	_, err := c.CountInputTokens(context.Background(), Request{
		Model:    "m",
		Messages: []Message{PlainMessage("user", "x")},
	})
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("want error, got %v", err)
	}
}
