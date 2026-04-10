package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatStreamText(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("no flusher")
		}
		writeSSE := func(payload any) {
			b, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		writeSSE(map[string]any{
			"choices": []any{map[string]any{"delta": map[string]any{"content": "Hi"}}},
		})
		writeSSE(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 3, "completion_tokens": 1},
		})
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)

	client := NewOpenAICompat("test-key", srv.URL+"/v1")
	ctx := context.Background()
	req := Request{
		Model:    "test-model",
		Messages: []Message{PlainMessage("user", "ping")},
		MaxTokens: 100,
	}
	events, errc := client.Stream(ctx, req)

	var gotText strings.Builder
	var usage Usage
	var sawDone bool
	for ev := range events {
		switch e := ev.(type) {
		case TextDelta:
			gotText.WriteString(e.Text)
		case Usage:
			usage = e
		case Done:
			sawDone = true
		}
	}
	if err := <-errc; err != nil {
		t.Fatalf("stream err: %v", err)
	}
	if gotText.String() != "Hi" {
		t.Fatalf("text = %q, want Hi", gotText.String())
	}
	if usage.InputTokens != 3 || usage.OutputTokens != 1 {
		t.Fatalf("usage = %+v", usage)
	}
	if !sawDone {
		t.Fatal("expected Done event")
	}
}

func TestOpenAICompatStreamToolCalls(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		writeSSE := func(payload any) {
			b, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		writeSSE(map[string]any{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{map[string]any{
						"index": 0.0,
						"id":    "call_abc",
						"type":  "function",
						"function": map[string]any{
							"name":      "read_file",
							"arguments": "",
						},
					}},
				},
			}},
		})
		writeSSE(map[string]any{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{map[string]any{
						"index": 0.0,
						"function": map[string]any{
							"arguments": `{"path":"a.go"}`,
						},
					}},
				},
			}},
		})
		writeSSE(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "tool_calls"}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
		})
	}))
	t.Cleanup(srv.Close)

	client := NewOpenAICompat("k", srv.URL+"/v1")
	ctx := context.Background()
	req := Request{
		Model: "m",
		Messages: []Message{PlainMessage("user", "read a.go")},
		Tools: []ToolSpec{
			{Name: "read_file", Description: "read", InputSchema: map[string]any{"type": "object"}},
		},
	}
	events, errc := client.Stream(ctx, req)

	var tu ToolUse
	var sawDone bool
	for ev := range events {
		switch e := ev.(type) {
		case ToolUse:
			tu = e
		case Done:
			sawDone = true
		}
	}
	if err := <-errc; err != nil {
		t.Fatalf("stream err: %v", err)
	}
	if tu.ID != "call_abc" || tu.Name != "read_file" || !strings.Contains(tu.Input, "a.go") {
		t.Fatalf("tool use = %+v", tu)
	}
	if !sawDone {
		t.Fatal("expected Done")
	}
}

func TestBuildOpenAIChatMessagesToolRoundTrip(t *testing.T) {
	t.Parallel()
	req := Request{
		System: "sys",
		Messages: []Message{
			PlainMessage("user", "go"),
			{
				Role: "assistant",
				ToolCalls: []ToolCallRecord{
					{ID: "call_1", Name: "read_file", Input: `{"path":"x"}`},
				},
			},
			{
				Role: "user",
				ToolResults: []ToolResultRecord{
					{ToolUseID: "call_1", Content: "file body"},
				},
			},
		},
	}
	msgs := buildOpenAIChatMessages(req)
	if len(msgs) != 4 {
		t.Fatalf("len %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != "sys" {
		t.Fatalf("system: %+v", msgs[0])
	}
	if msgs[1].Role != "user" || msgs[1].Content != "go" {
		t.Fatalf("user: %+v", msgs[1])
	}
	if msgs[2].Role != "assistant" || len(msgs[2].ToolCalls) != 1 {
		t.Fatalf("assistant: %+v", msgs[2])
	}
	if msgs[3].Role != "tool" || msgs[3].ToolCallID != "call_1" {
		t.Fatalf("tool: %+v", msgs[3])
	}
}
