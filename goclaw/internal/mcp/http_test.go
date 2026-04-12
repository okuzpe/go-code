package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateHTTPURL(t *testing.T) {
	loop, err := url.Parse("http://127.0.0.1:9/mcp")
	require.NoError(t, err)
	require.NoError(t, ValidateHTTPURL(loop, false))

	loc, err := url.Parse("http://localhost:8080/mcp")
	require.NoError(t, err)
	require.NoError(t, ValidateHTTPURL(loc, false))

	remote, err := url.Parse("https://example.com/mcp")
	require.NoError(t, err)
	require.Error(t, ValidateHTTPURL(remote, false))
	require.NoError(t, ValidateHTTPURL(remote, true))
}

func TestHTTPSessionJSONRoundTrip(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var msg map[string]any
		_ = json.NewDecoder(r.Body).Decode(&msg)
		method, _ := msg["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]any{
					"protocolVersion": streamHTTPProtocolVersion,
					"capabilities":    map[string]any{},
					"serverInfo":      map[string]any{"name": "t", "version": "1"},
				},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]any{
					"tools": []map[string]any{
						{"name": "echo", "description": "e", "inputSchema": map[string]any{"type": "object"}},
					},
				},
			})
		case "tools/call":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": "pong"}},
					"isError": false,
				},
			})
		default:
			http.Error(w, "unknown method", http.StatusBadRequest)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	h, err := NewHTTPSession(srv.URL+"/mcp", map[string]string{"X-Test": "1"})
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, h.Initialize(ctx))

	tools, err := h.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "echo", tools[0].Name)

	out, isErr, err := h.CallTool(ctx, "echo", `{}`)
	require.NoError(t, err)
	require.False(t, isErr)
	require.Equal(t, "pong", out)
	require.NoError(t, h.Close())
}

func TestHTTPSessionSSEResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var msg map[string]any
		_ = json.NewDecoder(r.Body).Decode(&msg)
		method, _ := msg["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "sse-sess")
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			line, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]any{
					"protocolVersion": streamHTTPProtocolVersion,
					"capabilities":    map[string]any{},
					"serverInfo":      map[string]any{"name": "s", "version": "1"},
				},
			})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			line, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result":  map[string]any{"tools": []map[string]any{}},
			})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		default:
			http.Error(w, "unknown", http.StatusBadRequest)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	h, err := NewHTTPSession(srv.URL+"/mcp", nil)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, h.Initialize(ctx))
	tools, err := h.ListTools(ctx)
	require.NoError(t, err)
	require.Empty(t, tools)
}
