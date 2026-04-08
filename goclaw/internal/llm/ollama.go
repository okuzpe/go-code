package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// OllamaClient implements Client against a local Ollama instance.
// Default host: http://localhost:11434
type OllamaClient struct {
	host string
	http *http.Client
}

// NewOllama creates a client. host defaults to http://localhost:11434.
func NewOllama(host string) *OllamaClient {
	if host == "" {
		host = "http://localhost:11434"
	}
	return &OllamaClient{
		host: strings.TrimRight(host, "/"),
		http: &http.Client{Timeout: 10 * time.Minute},
	}
}

// Stream sends a request to /api/chat and streams back events.
func (c *OllamaClient) Stream(ctx context.Context, req Request) (<-chan Event, <-chan error) {
	events := make(chan Event, 32)
	errc := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errc)
		if err := c.stream(ctx, req, events); err != nil {
			errc <- err
		}
	}()

	return events, errc
}

// --- Ollama wire types -------------------------------------------------------

type ollamaRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []map[string]any `json:"tools,omitempty"`
	Options  ollamaOptions    `json:"options,omitempty"`
}

// ollamaToolCallWire matches Ollama /api/chat tool_calls elements.
type ollamaToolCallWire struct {
	Type     string `json:"type,omitempty"` // "function" per Ollama/OpenAI-style tool calling
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

type ollamaMessage struct {
	Role      string               `json:"role"`
	Content   string               `json:"content"`
	ToolName  string               `json:"tool_name,omitempty"` // role "tool": which function this result belongs to
	ToolCalls []ollamaToolCallWire `json:"tool_calls,omitempty"`
}

type ollamaOptions struct {
	NumCtx int `json:"num_ctx,omitempty"` // context window size
}

// ollamaChunk is one NDJSON line from the streaming response.
type ollamaChunk struct {
	Model   string        `json:"model"`
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
	// token counts (only present when done=true)
	PromptEvalCount int `json:"prompt_eval_count"`
	EvalCount       int `json:"eval_count"`
}

func (c *OllamaClient) stream(ctx context.Context, req Request, out chan<- Event) error {
	body := ollamaRequest{
		Model:  req.Model,
		Stream: true,
	}

	// Prepend system message if provided.
	if req.System != "" {
		body.Messages = append(body.Messages, ollamaMessage{
			Role:    "system",
			Content: req.System,
		})
	}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, messageToOllama(m)...)
	}
	if tls := toolSpecsToOllama(req.Tools); len(tls) > 0 {
		body.Tools = tls
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := doHTTPWithRetry(ctx, c.http, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.host+"/api/chat", bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return fmt.Errorf("ollama: %w", wrapOllamaDialErr(c.host, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&apiErr) //nolint:errcheck
		return fmt.Errorf("ollama %d: %s", resp.StatusCode, apiErr.Error)
	}

	// Ollama streams NDJSON: one JSON object per line.
	scanner := bufio.NewScanner(resp.Body)
	if len(req.Tools) == 0 {
		return c.streamTextOnly(scanner, out)
	}
	return c.streamWithTools(scanner, out, req.Tools)
}

// streamTextOnly emits TextDelta per chunk (no tool contract).
func (c *OllamaClient) streamTextOnly(scanner *bufio.Scanner, out chan<- Event) error {
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var chunk ollamaChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			out <- TextDelta{Text: chunk.Message.Content}
		}
		if chunk.Done {
			out <- Usage{
				InputTokens:  chunk.PromptEvalCount,
				OutputTokens: chunk.EvalCount,
			}
			out <- Done{}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading stream: %w", err)
	}
	return nil
}

// streamWithTools buffers assistant content and prefers native tool_calls; if the model
// emits tool JSON in content (common with some local models), parse it at stream end.
func (c *OllamaClient) streamWithTools(scanner *bufio.Scanner, out chan<- Event, specs []ToolSpec) error {
	var contentBuf strings.Builder
	var lastNative []ollamaToolCallWire
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var chunk ollamaChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if len(chunk.Message.ToolCalls) > 0 {
			lastNative = make([]ollamaToolCallWire, len(chunk.Message.ToolCalls))
			copy(lastNative, chunk.Message.ToolCalls)
		}
		if chunk.Message.Content != "" {
			contentBuf.WriteString(chunk.Message.Content)
		}
		if chunk.Done {
			if tus := allNativeToolUses(lastNative); len(tus) > 0 {
				for _, tu := range tus {
					out <- tu
				}
			} else if tu, ok := parseOllamaContentAsToolUse(contentBuf.String(), specs); ok {
				out <- tu
			} else if s := contentBuf.String(); s != "" {
				out <- TextDelta{Text: s}
			}
			out <- Usage{
				InputTokens:  chunk.PromptEvalCount,
				OutputTokens: chunk.EvalCount,
			}
			out <- Done{}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading stream: %w", err)
	}
	return nil
}

func allNativeToolUses(calls []ollamaToolCallWire) []ToolUse {
	var out []ToolUse
	for i, tc := range calls {
		name := strings.TrimSpace(tc.Function.Name)
		if name == "" {
			continue
		}
		args := normalizeOllamaToolArgs(tc.Function.Arguments)
		out = append(out, ToolUse{
			ID:    fmt.Sprintf("ollama-tool-%d", i),
			Name:  name,
			Input: args,
		})
	}
	return out
}

func normalizeOllamaToolArgs(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return "{}"
	}
	return s
}

// parseOllamaContentAsToolUse handles models that print {"name":"tool","arguments":{...}} as plain text.
func parseOllamaContentAsToolUse(body string, specs []ToolSpec) (ToolUse, bool) {
	body = strings.TrimSpace(body)
	if body == "" || !strings.HasPrefix(body, "{") {
		return ToolUse{}, false
	}
	var v struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return ToolUse{}, false
	}
	if strings.TrimSpace(v.Name) == "" {
		return ToolUse{}, false
	}
	allowed := make(map[string]struct{}, len(specs))
	for _, sp := range specs {
		allowed[sp.Name] = struct{}{}
	}
	if _, ok := allowed[v.Name]; !ok {
		return ToolUse{}, false
	}
	args := normalizeOllamaToolArgs(v.Arguments)
	return ToolUse{ID: "ollama-json-0", Name: v.Name, Input: args}, true
}

func toolSpecsToOllama(specs []ToolSpec) []map[string]any {
	out := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		params, ok := s.InputSchema.(map[string]any)
		if !ok || params == nil {
			params = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        s.Name,
				"description": s.Description,
				"parameters":  params,
			},
		})
	}
	return out
}

// wrapOllamaDialErr adds actionable context when the local Ollama daemon is down or misconfigured.
func wrapOllamaDialErr(host string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Err != nil {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return fmt.Errorf("cannot reach Ollama at %s (connection refused): start the server (e.g. `ollama serve`) or set OLLAMA_HOST — %w", host, err)
		}
	}
	if strings.Contains(strings.ToLower(err.Error()), "connection refused") {
		return fmt.Errorf("cannot reach Ollama at %s (connection refused): start the server (e.g. `ollama serve`) or set OLLAMA_HOST — %w", host, err)
	}
	return err
}
