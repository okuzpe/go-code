// Package mockserver provides a deterministic Anthropic-compatible HTTP server for tests.
// It speaks the Anthropic SSE streaming protocol so AnthropicClient works without changes.
// Usage:
//
//	srv := mockserver.New(scenarios)
//	defer srv.Close()
//	client := llm.NewAnthropic("test-key", srv.URL)
package mockserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// ToolReply streams one tool_use block (instead of assistant text).
type ToolReply struct {
	ID    string
	Name  string
	Input string // JSON object as string, e.g. `{"path":"x"}`
}

// Scenario defines one canned request/response pair.
type Scenario struct {
	// Match is a substring matched against the last request message (text or tool_result bodies).
	// Empty string matches any message (use as default/fallback).
	Match string
	// Response is the assistant text to stream back when no tools are used.
	Response string
	// StatusCode overrides the HTTP status (default 200). Use 500 to test error paths.
	StatusCode int
	// Tool streams a single tool_use response when Tools is empty.
	Tool *ToolReply
	// Tools streams multiple tool_use blocks in one assistant message when non-empty.
	Tools []*ToolReply
}

// Server is a test HTTP server that replays canned scenarios as SSE streams.
type Server struct {
	*httptest.Server
	scenarios []Scenario
}

// New starts a mock server with the given scenarios and returns it.
// Scenarios are evaluated in order; first match wins.
func New(scenarios []Scenario) *Server {
	mux := http.NewServeMux()
	s := &Server{scenarios: scenarios}
	mux.HandleFunc("/v1/messages", s.handleMessages)
	s.Server = httptest.NewServer(mux)
	return s
}

type wireMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages []wireMessage `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	fp := lastMessageFingerprint(req.Messages)

	var matched *Scenario
	for i := range s.scenarios {
		sc := &s.scenarios[i]
		if sc.Match == "" || strings.Contains(fp, sc.Match) {
			matched = sc
			break
		}
	}
	if matched == nil {
		matched = &Scenario{Response: "I don't know how to respond to that."}
	}

	if matched.StatusCode != 0 && matched.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf(`{"error":{"type":"error","message":"mock error %d"}}`, matched.StatusCode),
			matched.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	switch {
	case len(matched.Tools) > 0:
		writeMultiToolUseStream(w, matched.Tools)
	case matched.Tool != nil:
		writeToolUseStream(w, matched.Tool, 0)
		writeToolMessageFooter(w, 1)
	default:
		writeTextStream(w, matched.Response)
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func lastMessageFingerprint(msgs []wireMessage) string {
	if len(msgs) == 0 {
		return ""
	}
	return fingerprintFromContent(msgs[len(msgs)-1].Content)
}

func fingerprintFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		_ = json.Unmarshal(raw, &s)
		return s
	}
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return string(raw)
	}
	var parts []string
	for _, b := range blocks {
		switch b["type"] {
		case "text":
			parts = append(parts, fmt.Sprint(b["text"]))
		case "tool_result":
			parts = append(parts, fmt.Sprint(b["content"]))
		}
	}
	return strings.Join(parts, "\n")
}

func writeTextStream(w http.ResponseWriter, text string) {
	writeSSE(w, "content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": 0,
		"content_block": map[string]any{
			"type": "text",
			"text": "",
		},
	})

	writeSSE(w, "content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": 0,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	})

	writeSSE(w, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": 0,
	})

	writeSSE(w, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
		},
		"usage": map[string]int{
			"input_tokens":  10,
			"output_tokens": len(strings.Fields(text)),
		},
	})

	writeSSE(w, "message_stop", map[string]any{
		"type": "message_stop",
	})
}

func writeToolUseStream(w http.ResponseWriter, tr *ToolReply, index int) {
	id := tr.ID
	if id == "" {
		id = fmt.Sprintf("toolu_mock_%02d", index)
	}
	writeSSE(w, "content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]any{
			"type": "tool_use",
			"id":   id,
			"name": tr.Name,
		},
	})

	writeSSE(w, "content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": index,
		"delta": map[string]any{
			"type":         "input_json_delta",
			"partial_json": tr.Input,
		},
	})

	writeSSE(w, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": index,
	})
}

func writeMultiToolUseStream(w http.ResponseWriter, tools []*ToolReply) {
	n := 0
	for i, tr := range tools {
		if tr == nil {
			continue
		}
		writeToolUseStream(w, tr, i)
		n++
	}
	writeToolMessageFooter(w, max(1, n))
}

func writeToolMessageFooter(w http.ResponseWriter, nTools int) { // nTools >= 1
	writeSSE(w, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason": "tool_use",
		},
		"usage": map[string]int{
			"input_tokens":  10,
			"output_tokens": 5 * nTools,
		},
	})

	writeSSE(w, "message_stop", map[string]any{
		"type": "message_stop",
	})
}

func writeSSE(w http.ResponseWriter, event string, payload any) {
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
}
