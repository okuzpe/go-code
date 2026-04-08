package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOllamaChatRequestContainsToolName verifies /api/chat JSON uses tool_name for role tool
// (regression: using "name" breaks tool round-trips on Ollama).
func TestOllamaChatRequestContainsToolName(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		b, _ := io.ReadAll(r.Body)
		captured = b
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.WriteString(w, `{"model":"m","message":{"role":"assistant","content":"ok"},"done":true}`+"\n")
	}))
	defer srv.Close()

	client := NewOllama(srv.URL)
	req := Request{
		Model: "m",
		Messages: []Message{
			{Role: "user", ToolResults: []ToolResultRecord{
				{ToolUseID: "x", ToolName: "web_search", Content: "results"},
			}},
		},
	}
	events, errc := client.Stream(context.Background(), req)
	for range events {
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(captured, []byte(`"tool_name"`)) {
		t.Fatalf("request body missing tool_name: %s", captured)
	}
	if bytes.Contains(captured, []byte(`"role":"tool"`)) && !bytes.Contains(captured, []byte(`"web_search"`)) {
		t.Fatalf("expected tool name in payload: %s", captured)
	}
}

// TestOllamaNativeToolCallsStream emits ToolUse from streamed tool_calls on final chunk.
func TestOllamaNativeToolCallsStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		line1 := `{"model":"m","message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.txt\"}"}}]},"done":false}` + "\n"
		line2 := `{"model":"m","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":1,"eval_count":2}` + "\n"
		io.Copy(w, strings.NewReader(line1+line2))
	}))
	defer srv.Close()

	client := NewOllama(srv.URL)
	req := Request{
		Model: "m",
		Messages: []Message{PlainMessage("user", "read a.txt")},
		Tools: []ToolSpec{{Name: "read_file", InputSchema: map[string]any{
			"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}},
		}}},
	}
	events, errc := client.Stream(context.Background(), req)
	var saw bool
	for e := range events {
		if tu, ok := e.(ToolUse); ok && tu.Name == "read_file" {
			saw = true
			if tu.Input == "" {
				t.Fatal("empty tool input")
			}
		}
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if !saw {
		t.Fatal("expected ToolUse event")
	}
}

func TestOllamaChatMarshalsToolCallType(t *testing.T) {
	msgs := messageToOllama(Message{
		Role: "assistant",
		ToolCalls: []ToolCallRecord{
			{ID: "1", Name: "foo", Input: `{"x":1}`},
		},
	})
	raw, err := json.Marshal(msgs[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"type":"function"`) {
		t.Fatalf("want type function in tool_calls: %s", raw)
	}
}
