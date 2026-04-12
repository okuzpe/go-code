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
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// OllamaClient implements Client against a local Ollama instance.
// Default host: http://localhost:11434
type OllamaClient struct {
	host string
	http *http.Client

	// toolsUnsupportedOnce logs once when we fall back to chat-only after Ollama rejects tools.
	// Use Debug (not Warn): writing stderr during Bubble Tea alt-screen corrupts the TUI on some terminals (e.g. Windows).
	toolsUnsupportedOnce sync.Once

	// functionToolsDropped is set when Ollama returns "does not support tools" and we retry without wire tools.
	functionToolsDropped atomic.Bool
}

// FunctionToolsDropped reports whether this client ever fell back to HTTP /api/chat without
// the tools JSON field because the model rejected tool calling. The UI can surface this once
// per session (e.g. footer) so users know read_file / web_search are not available on the wire.
func (c *OllamaClient) FunctionToolsDropped() bool {
	return c.functionToolsDropped.Load()
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
				slog.Warn("ollama: model rejected tool calling — falling back to text-only (no read_file/bash/etc.). " +
					"Use a tools-capable model (e.g. qwen2.5-coder:7b) or update Ollama. " +
					"Set ollama_num_ctx in settings.json (default 8192) if context is too small for tool schemas. " +
					"Ollama error: " + msg)
			})
			c.functionToolsDropped.Store(true)
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

// ollamaTextOnlyToolingNote is appended when Ollama retries without wire tools but the session
// still had tool specs (model rejected tools). Keeps the model honest about the wire while still
// acting as a coding pair programmer (markdown patches, steps, fenced code the user can paste).
const ollamaTextOnlyToolingNote = "[OLLAMA TEXT-ONLY FALLBACK: this HTTP request has no function tools on the wire — do not claim or simulate tool_use, read_file, bash, web_search, or any API/tool execution. You cannot read the user's repo or run commands in this request. Still help as a coding assistant: give concrete edits in fenced code blocks, ordered checklists, unified-style diffs, and file-by-file instructions the user can apply manually. If you need file contents, ask them to paste snippets or use @ paths in clients that expand them before send. For live web data or repo truth, say briefly (in the user's language) that this mode cannot fetch it here. Do not dwell on tooling mechanics.]"

func buildOllamaChatRequest(req Request, wireTools bool) ollamaRequest {
	body := ollamaRequest{
		Model:  req.Model,
		Stream: true,
	}
	if req.MaxTokens > 0 {
		body.Options.NumPredict = req.MaxTokens
	}
	if req.NumCtx > 0 {
		body.Options.NumCtx = req.NumCtx
	}
	if req.System != "" {
		sys := req.System
		if !wireTools && len(req.Tools) > 0 {
			sys = strings.TrimSpace(sys) + "\n\n" + ollamaTextOnlyToolingNote
		}
		body.Messages = append(body.Messages, ollamaMessage{
			Role:    "system",
			Content: sys,
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

// proseToolNameAliases map hallucinated tool labels from local models to registry names.
// Only targets that exist in the current request's ToolSpec list are emitted.
var proseToolNameAliases = map[string]string{
	"glob_file_search":    "glob",
	"analyze_file_search": "glob",
	"file_search":         "glob",
	"codebase_search":     "glob",
	"list_dir":            "glob",
	"search_files":        "glob",
}

// toolCallDirectiveRE matches a whole line like "TOOL CALL: read_file" (case-insensitive).
var toolCallDirectiveRE = regexp.MustCompile(`(?i)^\s*tool call:\s*(\S+)\s*$`)

func allowedToolNamesLower(specs []ToolSpec) map[string]string {
	m := make(map[string]string, len(specs))
	for _, sp := range specs {
		k := strings.ToLower(strings.TrimSpace(sp.Name))
		if k != "" {
			m[k] = sp.Name
		}
	}
	return m
}

// resolveProseToolName maps a model-supplied token to a concrete tool name in specs.
func resolveProseToolName(raw string, byLower map[string]string) (string, bool) {
	t := strings.TrimSpace(raw)
	if t == "" {
		return "", false
	}
	low := strings.ToLower(t)
	if canon, ok := proseToolNameAliases[low]; ok {
		low = strings.ToLower(canon)
	}
	if exact, ok := byLower[low]; ok {
		return exact, true
	}
	return "", false
}

// defaultInputForProseToolDirective returns JSON input when the model emitted only a name (no JSON).
// Only safe defaults are supported; unknown tools return false.
func defaultInputForProseToolDirective(toolName string) (string, bool) {
	switch toolName {
	case "glob":
		return `{"pattern":"*"}`, true
	default:
		return "", false
	}
}

// tryProseToolDirective detects lines like "TOOL CALL: foo" and converts them to a native ToolUse
// when the resolved tool is in specs and has a safe default input (see defaultInputForProseToolDirective).
func tryProseToolDirective(full string, specs []ToolSpec) (prose string, tu ToolUse, ok bool) {
	full = strings.TrimSpace(full)
	if full == "" || len(specs) == 0 {
		return "", ToolUse{}, false
	}
	byLower := allowedToolNamesLower(specs)
	lines := strings.Split(full, "\n")
	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		sub := toolCallDirectiveRE.FindStringSubmatch(line)
		if len(sub) != 2 {
			continue
		}
		resolved, okName := resolveProseToolName(sub[1], byLower)
		if !okName {
			return "", ToolUse{}, false
		}
		input, okIn := defaultInputForProseToolDirective(resolved)
		if !okIn {
			return "", ToolUse{}, false
		}
		var parts []string
		if b := strings.TrimSpace(strings.Join(lines[:i], "\n")); b != "" {
			parts = append(parts, b)
		}
		if a := strings.TrimSpace(strings.Join(lines[i+1:], "\n")); a != "" {
			parts = append(parts, a)
		}
		proseOut := strings.TrimSpace(strings.Join(parts, "\n"))
		return proseOut, ToolUse{
			ID:    "ollama-prose-0",
			Name:  resolved,
			Input: input,
		}, true
	}
	return "", ToolUse{}, false
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
				} else if prose, tu, ok := tryProseToolDirective(fullContent, specs); ok {
					if strings.TrimSpace(prose) != "" {
						out <- TextDelta{Text: prose}
					}
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
		return fmt.Errorf("ollama at %s did not respond in time. Is the daemon running and reachable? Check VPN/firewall and OLLAMA_HOST — %w", host, err)
	}

	if strings.Contains(low, "no such host") || strings.Contains(low, "could not resolve") {
		return fmt.Errorf("ollama host %s could not be resolved (DNS). Check the host name in OLLAMA_HOST or settings.json — %w", host, err)
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
