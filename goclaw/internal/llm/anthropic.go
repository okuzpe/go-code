package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const anthropicVersion = "2023-06-01"

// AnthropicClient implements Client against the Anthropic Messages API.
type AnthropicClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewAnthropic creates a client. baseURL defaults to https://api.anthropic.com.
func NewAnthropic(apiKey, baseURL string) *AnthropicClient {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicClient{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// Stream sends a request to /v1/messages and streams back events.
// The caller must drain both channels; errc receives at most one value.
func (c *AnthropicClient) Stream(ctx context.Context, req Request) (<-chan Event, <-chan error) {
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

// --- internal ----------------------------------------------------------------

// anthropicRequest is the JSON body sent to /v1/messages.
type anthropicRequest struct {
	Model     string                 `json:"model"`
	MaxTokens int                    `json:"max_tokens"`
	System    string                 `json:"system,omitempty"`
	Messages  []anthropicWireMessage `json:"messages"`
	Tools     []anthropicTool        `json:"tools,omitempty"`
	Stream    bool                   `json:"stream"`
}

type anthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

// SSE event shapes from Anthropic streaming protocol.
type sseEvent struct {
	Type  string    `json:"type"`
	Index int       `json:"index"`
	Delta *sseDelta `json:"delta,omitempty"`
	Usage *sseUsage `json:"usage,omitempty"`
	// content_block_start fields
	ContentBlock *sseBlock `json:"content_block,omitempty"`
}

type sseDelta struct {
	Type        string `json:"type"`         // "text_delta" | "input_json_delta"
	Text        string `json:"text"`         // text_delta
	PartialJSON string `json:"partial_json"` // input_json_delta
	StopReason  string `json:"stop_reason,omitempty"`
}

type sseUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type sseBlock struct {
	Type string `json:"type"` // "text" | "tool_use"
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *AnthropicClient) stream(ctx context.Context, req Request, out chan<- Event) error {
	// Build request body.
	body := anthropicRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		System:    req.System,
		Stream:    true,
	}
	if body.MaxTokens == 0 {
		body.MaxTokens = 8192
	}
	wire, err := messagesToAnthropicWire(req.Messages)
	if err != nil {
		return fmt.Errorf("anthropic messages: %w", err)
	}
	body.Messages = wire
	for _, t := range req.Tools {
		w := anthropicTool{}
		w.Name = t.Name
		w.Description = t.Description
		w.InputSchema = t.InputSchema
		body.Tools = append(body.Tools, w)
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := doHTTPWithRetry(ctx, c.http, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.baseURL+"/v1/messages", bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", anthropicVersion)
		req.Header.Set("Accept", "text/event-stream")
		return req, nil
	})
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&apiErr) //nolint:errcheck
		return fmt.Errorf("anthropic %d: %s", resp.StatusCode, apiErr.Error.Message)
	}

	// Parse SSE stream.
	// Anthropic SSE format:
	//   event: <type>
	//   data: <json>
	//   (blank line)
	scanner := bufio.NewScanner(resp.Body)

	// track tool_use block being accumulated across content_block_delta events
	var (
		currentToolID   string
		currentToolName string
		currentToolJSON strings.Builder
	)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var ev sseEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue // skip malformed lines
		}

		switch ev.Type {
		case "content_block_start":
			if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {
				currentToolID = ev.ContentBlock.ID
				currentToolName = ev.ContentBlock.Name
				currentToolJSON.Reset()
			}

		case "content_block_delta":
			if ev.Delta == nil {
				continue
			}
			switch ev.Delta.Type {
			case "text_delta":
				out <- TextDelta{Text: ev.Delta.Text}
			case "input_json_delta":
				currentToolJSON.WriteString(ev.Delta.PartialJSON)
			}

		case "content_block_stop":
			// If we were accumulating a tool_use, emit it now.
			if currentToolID != "" {
				out <- ToolUse{
					ID:    currentToolID,
					Name:  currentToolName,
					Input: currentToolJSON.String(),
				}
				currentToolID = ""
				currentToolName = ""
				currentToolJSON.Reset()
			}

		case "message_delta":
			if ev.Usage != nil {
				out <- Usage{
					InputTokens:  ev.Usage.InputTokens,
					OutputTokens: ev.Usage.OutputTokens,
				}
			}

		case "message_stop":
			out <- Done{}
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading stream: %w", err)
	}
	return nil
}
