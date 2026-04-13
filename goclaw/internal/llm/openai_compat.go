package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// maxOpenAISSELine caps bufio.Scanner token size for long SSE lines (tool arguments).
const maxOpenAISSELine = 1024 * 1024

// OpenAICompatClient implements Client against an OpenAI Chat Completions–compatible HTTP API.
// It is not wired into the goclaw CLI (Ollama-only product build); it remains for unit tests and mock servers.
// (OpenRouter, Groq, LM Studio local server, vLLM, Azure OpenAI, etc.).
type OpenAICompatClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewOpenAICompat creates a client. baseURL must include the API version prefix, e.g.
// https://api.openai.com/v1 or https://openrouter.ai/api/v1
func NewOpenAICompat(apiKey, baseURL string) *OpenAICompatClient {
	return &OpenAICompatClient{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		http:    &http.Client{Timeout: 10 * time.Minute},
	}
}

// Stream sends a request to /chat/completions with stream=true and parses SSE chunks.
func (c *OpenAICompatClient) Stream(ctx context.Context, req Request) (<-chan Event, <-chan error) {
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

type openAIChatRequest struct {
	Model      string              `json:"model"`
	Messages   []openAIChatMessage `json:"messages"`
	Stream     bool                `json:"stream"`
	MaxTokens  int                 `json:"max_tokens,omitempty"`
	Tools      []openAIToolDef     `json:"tools,omitempty"`
	ToolChoice any                 `json:"tool_choice,omitempty"`
}

type openAIChatMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content,omitempty"`
	ToolCalls  []openAIToolCallWire `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type openAIToolCallWire struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIToolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters,omitempty"`
	} `json:"function"`
}

func (c *OpenAICompatClient) stream(ctx context.Context, req Request, out chan<- Event) error {
	body := openAIChatRequest{
		Model:    req.Model,
		Messages: buildOpenAIChatMessages(req),
		Stream:   true,
	}
	if req.MaxTokens > 0 {
		body.MaxTokens = req.MaxTokens
	} else {
		body.MaxTokens = 8192
	}
	for _, t := range req.Tools {
		def := openAIToolDef{Type: "function"}
		def.Function.Name = t.Name
		def.Function.Description = t.Description
		def.Function.Parameters = t.InputSchema
		body.Tools = append(body.Tools, def)
	}
	if len(body.Tools) > 0 {
		body.ToolChoice = "auto"
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal openai request: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	resp, err := doHTTPWithRetry(ctx, c.http, func() (*http.Request, error) {
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		r.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			r.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		r.Header.Set("Accept", "text/event-stream")
		return r, nil
	})
	if err != nil {
		return fmt.Errorf("openai_compat http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		msg := strings.TrimSpace(apiErr.Error.Message)
		if msg == "" {
			msg = "request failed"
		}
		return fmt.Errorf("openai_compat %d: %s", resp.StatusCode, msg)
	}

	return parseOpenAIEventStream(resp.Body, out)
}

func buildOpenAIChatMessages(req Request) []openAIChatMessage {
	var out []openAIChatMessage
	if strings.TrimSpace(req.System) != "" {
		out = append(out, openAIChatMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			if len(m.ToolResults) == 0 {
				out = append(out, openAIChatMessage{Role: "user", Content: m.Content})
				continue
			}
			if strings.TrimSpace(m.Content) != "" {
				out = append(out, openAIChatMessage{Role: "user", Content: m.Content})
			}
			for _, tr := range m.ToolResults {
				out = append(out, openAIChatMessage{
					Role:       "tool",
					ToolCallID: tr.ToolUseID,
					Content:    toolResultOpenAIContent(tr),
				})
			}
		case "assistant":
			if len(m.ToolCalls) == 0 {
				out = append(out, openAIChatMessage{Role: "assistant", Content: m.Content})
				continue
			}
			tcs := make([]openAIToolCallWire, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				args := strings.TrimSpace(tc.Input)
				if args == "" {
					args = "{}"
				}
				w := openAIToolCallWire{
					ID:   tc.ID,
					Type: "function",
				}
				w.Function.Name = tc.Name
				w.Function.Arguments = args
				tcs = append(tcs, w)
			}
			out = append(out, openAIChatMessage{
				Role:      "assistant",
				Content:   m.Content,
				ToolCalls: tcs,
			})
		default:
			out = append(out, openAIChatMessage{Role: m.Role, Content: m.Content})
		}
	}
	return out
}

func toolResultOpenAIContent(tr ToolResultRecord) string {
	if tr.IsError {
		return "error: " + tr.Content
	}
	return tr.Content
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type openAIToolAccum struct {
	id      string
	name    string
	args    strings.Builder
	hasID   bool
	hasName bool
}

func parseOpenAIEventStream(body io.Reader, out chan<- Event) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxOpenAISSELine)
	byIndex := make(map[int]*openAIToolAccum)

	flushTools := func() {
		if len(byIndex) == 0 {
			return
		}
		indices := make([]int, 0, len(byIndex))
		for i := range byIndex {
			indices = append(indices, i)
		}
		sort.Ints(indices)
		for _, idx := range indices {
			acc := byIndex[idx]
			if acc == nil || !acc.hasName {
				continue
			}
			id := acc.id
			if id == "" {
				id = fmt.Sprintf("openai-tool-%d", idx)
			}
			args := acc.args.String()
			if strings.TrimSpace(args) == "" {
				args = "{}"
			}
			out <- ToolUse{ID: id, Name: strings.TrimSpace(acc.name), Input: args}
		}
		byIndex = make(map[int]*openAIToolAccum)
	}

	emitUsage := func(u *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	}) {
		if u == nil {
			out <- Usage{}
			return
		}
		out <- Usage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			flushTools()
			out <- Done{}
			return nil
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			ch := chunk.Choices[0]

			if ch.Delta.Content != "" {
				if t := stripLeakedChatTemplateTokens(ch.Delta.Content); t != "" {
					out <- TextDelta{Text: t}
				}
			}

			for _, tc := range ch.Delta.ToolCalls {
				acc, ok := byIndex[tc.Index]
				if !ok {
					acc = &openAIToolAccum{}
					byIndex[tc.Index] = acc
				}
				if tc.ID != "" {
					acc.id = tc.ID
					acc.hasID = true
				}
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
					acc.hasName = true
				}
				if tc.Function.Arguments != "" {
					acc.args.WriteString(tc.Function.Arguments)
				}
			}

			switch ch.FinishReason {
			case "tool_calls":
				flushTools()
				emitUsage(chunk.Usage)
				out <- Done{}
				return nil
			case "stop", "length":
				flushTools()
				emitUsage(chunk.Usage)
				out <- Done{}
				return nil
			}
		} else if chunk.Usage != nil {
			emitUsage(chunk.Usage)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading openai stream: %w", err)
	}
	flushTools()
	out <- Done{}
	return nil
}

var _ Client = (*OpenAICompatClient)(nil)
