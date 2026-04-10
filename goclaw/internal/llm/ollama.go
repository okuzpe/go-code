package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"time"
)

// OllamaClient implements Client against a local Ollama instance.
// Default host: http://localhost:11434
type OllamaClient struct {
	host string
	http *http.Client

	// toolsUnsupportedOnce logs once when we fall back to chat-only after Ollama rejects tools.
	toolsUnsupportedOnce sync.Once
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
	NumCtx     int `json:"num_ctx,omitempty"`     // context window size
	NumPredict int `json:"num_predict,omitempty"` // max tokens to generate (maps from Request.MaxTokens)
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
	return c.streamWithWireTools(ctx, req, out, true)
}

// streamWithWireTools posts /api/chat. If Ollama returns 400 because the model does not support
// the tools API (common for llama3:latest), retries once without tools so local chat still works.
func (c *OllamaClient) streamWithWireTools(ctx context.Context, req Request, out chan<- Event, wireTools bool) error {
	body := buildOllamaChatRequest(req, wireTools)

	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := doHTTPWithRetry(ctx, c.http, func() (*http.Request, error) {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.host+"/api/chat", bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		return httpReq, nil
	})
	if err != nil {
		return fmt.Errorf("ollama: %w", wrapOllamaDialErr(c.host, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("ollama %d: read error body: %w", resp.StatusCode, readErr)
		}
		msg := parseOllamaErrorMessage(errBody)
		if wireTools && len(req.Tools) > 0 && ollamaReportsToolsUnsupported(resp.StatusCode, msg) {
			c.toolsUnsupportedOnce.Do(func() {
				slog.Warn("ollama: model does not support tool calling; using chat-only requests (no agent tools). For read_file/bash/etc. use a tools-capable model such as qwen2.5-coder:7b")
			})
			return c.streamWithWireTools(ctx, req, out, false)
		}
		return fmt.Errorf("ollama %d: %s", resp.StatusCode, msg)
	}

	scanner := bufio.NewScanner(resp.Body)
	toolsOnWire := wireTools && len(toolSpecsToOllama(req.Tools)) > 0
	if !toolsOnWire {
		return c.streamTextOnly(scanner, out)
	}
	return c.streamWithTools(scanner, out, req.Tools)
}

func buildOllamaChatRequest(req Request, wireTools bool) ollamaRequest {
	body := ollamaRequest{
		Model:  req.Model,
		Stream: true,
	}
	if req.MaxTokens > 0 {
		body.Options.NumPredict = req.MaxTokens
	}
	if req.System != "" {
		body.Messages = append(body.Messages, ollamaMessage{
			Role:    "system",
			Content: req.System,
		})
	}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, messageToOllama(m)...)
	}
	if wireTools {
		if tls := toolSpecsToOllama(req.Tools); len(tls) > 0 {
			body.Tools = tls
		}
	}
	return body
}

func parseOllamaErrorMessage(body []byte) string {
	var apiErr struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &apiErr); err == nil && strings.TrimSpace(apiErr.Error) != "" {
		return apiErr.Error
	}
	return strings.TrimSpace(string(body))
}

func ollamaReportsToolsUnsupported(status int, msg string) bool {
	if status != http.StatusBadRequest {
		return false
	}
	m := strings.ToLower(msg)
	return strings.Contains(m, "does not support tools") ||
		strings.Contains(m, "doesn't support tools") ||
		strings.Contains(m, "no support for tools")
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

// streamWithTools buffers all assistant content until the stream ends, then decides
// whether the content is a tool call or plain text. If it's a tool call, only the
// ToolUse event is emitted (no raw JSON in the chat). If it's text, it's emitted as
// a single TextDelta. The TUI shows a "thinking" spinner during the buffering phase.
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
			// Text is buffered — NOT streamed yet. We only emit it at
			// the end once we know it's not a tool call.
		}
		if chunk.Done {
			emittedTool := false

			if tus := allNativeToolUses(lastNative); len(tus) > 0 {
				// Native tool calls — emit them, suppress raw text.
				for _, tu := range tus {
					out <- tu
				}
				emittedTool = true
			} else {
				fullContent := contentBuf.String()
				// Try parsing content as a tool call (various formats).
				if tu, ok := parseOllamaContentAsToolUse(fullContent, specs); ok {
					out <- tu
					emittedTool = true
				} else if tu, _, ok := extractEmbeddedToolCall(fullContent, specs); ok {
					out <- tu
					emittedTool = true
				}
			}

			// Only show the text to the user if it was NOT a tool call.
			if !emittedTool {
				if s := contentBuf.String(); s != "" {
					out <- TextDelta{Text: s}
				}
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

// parseOllamaContentAsToolUse handles models that print {"name":"tool","arguments":{...}} as plain text,
// optionally wrapped in markdown code fences (```json ... ```).
func parseOllamaContentAsToolUse(body string, specs []ToolSpec) (ToolUse, bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return ToolUse{}, false
	}

	// Strip markdown code fences if present (```json ... ``` or ``` ... ```).
	body = stripCodeFences(body)

	if !strings.HasPrefix(body, "{") {
		return ToolUse{}, false
	}

	// Try format 1: {"name":"tool","arguments":{...}}
	var v struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(body), &v); err == nil && strings.TrimSpace(v.Name) != "" {
		allowed := make(map[string]struct{}, len(specs))
		for _, sp := range specs {
			allowed[sp.Name] = struct{}{}
		}
		if _, ok := allowed[v.Name]; ok {
			args := normalizeOllamaToolArgs(v.Arguments)
			return ToolUse{ID: "ollama-json-0", Name: v.Name, Input: args}, true
		}
	}

	// Try format 2: direct tool arguments {"query":"..."} — check if body is valid JSON
	// and matches any single-tool scenario (model just emitted the args directly).
	// Skip this format as it's too ambiguous without knowing which tool.

	return ToolUse{}, false
}

// stripCodeFences removes markdown code fences (```lang\n...\n```) from around content.
// Returns the unwrapped content. If no fences are found, returns the input unchanged.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Find the end of the opening fence line.
	firstNewline := strings.Index(s, "\n")
	if firstNewline < 0 {
		return s
	}
	inner := s[firstNewline+1:]
	// Find the closing fence.
	lastFence := strings.LastIndex(inner, "```")
	if lastFence >= 0 {
		inner = inner[:lastFence]
	}
	return strings.TrimSpace(inner)
}

// extractEmbeddedToolCall scans prose text for embedded tool calls that local
// models sometimes emit. Patterns supported:
//
//	tool_name {"key":"value"}
//	tool_name({"key":"value"})
//	... prose ... {"name":"tool_name","arguments":{...}} ... prose ...
//	```json\n{"name":"tool_name","arguments":{...}}\n``` (inside code fences)
//
// Returns the ToolUse, any surrounding prose text, and whether a match was found.
func extractEmbeddedToolCall(body string, specs []ToolSpec) (ToolUse, string, bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return ToolUse{}, body, false
	}

	// Strip code fences if the whole body is wrapped.
	stripped := stripCodeFences(body)

	// Build a set of known tool names.
	allowed := make(map[string]struct{}, len(specs))
	for _, sp := range specs {
		allowed[sp.Name] = struct{}{}
	}

	// Strategy 1: Search for {"name":"tool_name"...} JSON objects anywhere in the text.
	// This catches models that emit the full call spec as JSON embedded in prose.
	for _, candidate := range []string{stripped, body} {
		for toolName := range allowed {
			// Look for {"name":"tool_name" pattern (with various quoting/spacing).
			patterns := []string{
				`"name":"` + toolName + `"`,
				`"name": "` + toolName + `"`,
				`"name" : "` + toolName + `"`,
			}
			for _, pat := range patterns {
				idx := strings.Index(candidate, pat)
				if idx < 0 {
					continue
				}
				// Walk backward to find the opening {.
				jsonStart := strings.LastIndex(candidate[:idx], "{")
				if jsonStart < 0 {
					continue
				}
				// Walk forward to find matching closing }.
				jsonEnd := findMatchingBrace(candidate, jsonStart)
				if jsonEnd < 0 {
					continue
				}
				jsonStr := candidate[jsonStart : jsonEnd+1]
				var v struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				}
				if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
					continue
				}
				if strings.TrimSpace(v.Name) == "" {
					continue
				}
				if _, ok := allowed[v.Name]; !ok {
					continue
				}
				prose := strings.TrimSpace(candidate[:jsonStart]) + " " + strings.TrimSpace(candidate[jsonEnd+1:])
				prose = strings.TrimSpace(prose)
				args := normalizeOllamaToolArgs(v.Arguments)
				return ToolUse{
					ID:    "ollama-embedded-0",
					Name:  v.Name,
					Input: args,
				}, prose, true
			}
		}
	}

	// Strategy 2: Search for "tool_name {" or "tool_name({" pattern in the text.
	for toolName := range allowed {
		for _, candidate := range []string{stripped, body} {
			idx := strings.Index(candidate, toolName+" {")
			if idx < 0 {
				idx = strings.Index(candidate, toolName+"({")
			}
			if idx < 0 {
				continue
			}

			// Find the JSON start.
			jsonStart := strings.Index(candidate[idx:], "{")
			if jsonStart < 0 {
				continue
			}
			jsonStart += idx

			jsonEnd := findMatchingBrace(candidate, jsonStart)
			if jsonEnd < 0 {
				continue
			}

			jsonStr := candidate[jsonStart : jsonEnd+1]
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
				continue
			}

			prose := strings.TrimSpace(candidate[:idx]) + " " + strings.TrimSpace(candidate[jsonEnd+1:])
			prose = strings.TrimSpace(prose)

			return ToolUse{
				ID:    "ollama-embedded-0",
				Name:  toolName,
				Input: jsonStr,
			}, prose, true
		}
	}

	return ToolUse{}, body, false
}

// findMatchingBrace finds the index of the closing '}' that matches the opening
// '{' at position start. Returns -1 if no match is found.
func findMatchingBrace(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
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
	low := strings.ToLower(err.Error())

	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Err != nil {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return fmt.Errorf("cannot reach Ollama at %s (connection refused). Is Ollama running? Try `ollama serve`, confirm `ollama ps`, or set OLLAMA_HOST if the daemon listens elsewhere — %w", host, err)
		}
	}
	if strings.Contains(low, "connection refused") {
		return fmt.Errorf("cannot reach Ollama at %s (connection refused). Is Ollama running? Try `ollama serve`, confirm `ollama ps`, or set OLLAMA_HOST if the daemon listens elsewhere — %w", host, err)
	}

	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("Ollama at %s did not respond in time. Is the daemon running and reachable? Check VPN/firewall and OLLAMA_HOST — %w", host, err)
	}

	if strings.Contains(low, "no such host") || strings.Contains(low, "could not resolve") {
		return fmt.Errorf("Ollama host %s could not be resolved (DNS). Check the host name in OLLAMA_HOST or settings.json — %w", host, err)
	}

	if strings.Contains(low, "tls") || strings.Contains(low, "x509") || strings.Contains(low, "certificate") {
		return fmt.Errorf("TLS error talking to Ollama at %s (https URL or proxy?). For a local daemon use http://127.0.0.1:11434 unless you have HTTPS termination — %w", host, err)
	}

	// Typical client dial / transport errors
	if strings.Contains(low, "dial tcp") || strings.Contains(low, "dial ") || strings.Contains(low, "connectex") || strings.Contains(low, "wsarefuse") {
		return fmt.Errorf("cannot connect to Ollama at %s. Is Ollama running (`ollama serve`)? Confirm OLLAMA_HOST matches where the daemon listens; run `goclaw doctor` — %w", host, err)
	}

	return err
}
