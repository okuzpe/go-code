package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// anthropicCountTokensBody is the JSON body for POST /v1/messages/count_tokens.
type anthropicCountTokensBody struct {
	Model    string                 `json:"model"`
	System   string                 `json:"system,omitempty"`
	Messages []anthropicWireMessage `json:"messages"`
	Tools    []anthropicTool        `json:"tools,omitempty"`
}

// CountInputTokens calls the Anthropic count_tokens API for the same shape as a chat request
// (model, system, messages, tools). It does not include max_tokens or streaming.
func (c *AnthropicClient) CountInputTokens(ctx context.Context, req Request) (int, error) {
	wire, err := messagesToAnthropicWire(req.Messages)
	if err != nil {
		return 0, fmt.Errorf("anthropic count_tokens messages: %w", err)
	}
	body := anthropicCountTokensBody{
		Model:    req.Model,
		System:   req.System,
		Messages: wire,
	}
	for _, t := range req.Tools {
		body.Tools = append(body.Tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal count_tokens: %w", err)
	}

	resp, err := doHTTPWithRetry(ctx, c.http, func() (*http.Request, error) {
		r, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.baseURL+"/v1/messages/count_tokens", bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("x-api-key", c.apiKey)
		r.Header.Set("anthropic-version", anthropicVersion)
		r.Header.Set("Accept", "application/json")
		return r, nil
	})
	if err != nil {
		return 0, fmt.Errorf("count_tokens http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		return 0, fmt.Errorf("anthropic count_tokens %d: %s", resp.StatusCode, apiErr.Error.Message)
	}

	var out struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("decode count_tokens: %w", err)
	}
	return out.InputTokens, nil
}
