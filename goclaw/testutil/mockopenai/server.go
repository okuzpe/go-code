// Package mockopenai provides a deterministic OpenAI Chat Completions–compatible HTTP server
// for tests. It streams SSE in the shape expected by llm.OpenAICompatClient.
//
//	srv := mockopenai.New(scenarios)
//	defer srv.Close()
//	client := llm.NewOpenAICompat("test-key", srv.URL+"/v1")
package mockopenai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// ToolReply streams one tool call in the assistant message.
type ToolReply struct {
	ID    string
	Name  string
	Input string // JSON object as string, e.g. `{"path":"x"}`
}

// Scenario defines one canned request/response pair.
type Scenario struct {
	Match      string // substring matched against fingerprint of the last user-visible message
	Response   string
	StatusCode int
	Tool       *ToolReply
	Tools      []*ToolReply
}

// Server is a test HTTP server that replays canned scenarios as OpenAI-style SSE streams.
type Server struct {
	*httptest.Server
	scenarios []Scenario
}

// New starts a mock server. Scenarios are evaluated in order; first match wins.
func New(scenarios []Scenario) *Server {
	mux := http.NewServeMux()
	s := &Server{scenarios: scenarios}
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	s.Server = httptest.NewServer(mux)
	return s
}

type chatReqMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Messages []chatReqMsg `json:"messages"`
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	fp := fingerprintFromMessages(req.Messages)

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
		http.Error(w, fmt.Sprintf(`{"error":{"message":"mock error %d"}}`, matched.StatusCode), matched.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	writeSSE := func(payload any) {
		b, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
		if flusher != nil {
			flusher.Flush()
		}
	}

	switch {
	case len(matched.Tools) > 0:
		writeMultiToolStream(writeSSE, matched.Tools)
	case matched.Tool != nil:
		writeToolStream(writeSSE, matched.Tool, 0)
		writeSSE(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "tool_calls"}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
		})
	default:
		if matched.Response != "" {
			writeSSE(map[string]any{
				"choices": []any{map[string]any{"delta": map[string]any{"content": matched.Response}}},
			})
		}
		writeSSE(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": len(strings.Fields(matched.Response))},
		})
	}
	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func fingerprintFromMessages(msgs []chatReqMsg) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		c := strings.TrimSpace(msgs[i].Content)
		if c != "" {
			return c
		}
	}
	return ""
}

func writeToolStream(writeSSE func(any), tr *ToolReply, index int) {
	id := tr.ID
	if id == "" {
		id = fmt.Sprintf("call_mock_%02d", index)
	}
	writeSSE(map[string]any{
		"choices": []any{map[string]any{
			"delta": map[string]any{
				"tool_calls": []any{map[string]any{
					"index": float64(index),
					"id":    id,
					"type":  "function",
					"function": map[string]any{
						"name":      tr.Name,
						"arguments": "",
					},
				}},
			},
		}},
	})
	args := strings.TrimSpace(tr.Input)
	if args == "" {
		args = "{}"
	}
	writeSSE(map[string]any{
		"choices": []any{map[string]any{
			"delta": map[string]any{
				"tool_calls": []any{map[string]any{
					"index":    float64(index),
					"function": map[string]any{"arguments": args},
				}},
			},
		}},
	})
}

func writeMultiToolStream(writeSSE func(any), tools []*ToolReply) {
	idx := 0
	for _, tr := range tools {
		if tr == nil {
			continue
		}
		writeToolStream(writeSSE, tr, idx)
		idx++
	}
	writeSSE(map[string]any{
		"choices": []any{map[string]any{"finish_reason": "tool_calls"}},
		"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5 * max(1, idx)},
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
