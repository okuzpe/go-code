package llm

import (
	"encoding/json"
	"fmt"
)

// anthropicWireMessage is one element of the Messages array for /v1/messages.
// Content is either a plain string or an array of content blocks (text, tool_use, tool_result).
type anthropicWireMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func messagesToAnthropicWire(msgs []Message) ([]anthropicWireMessage, error) {
	out := make([]anthropicWireMessage, 0, len(msgs))
	for _, m := range msgs {
		w, err := messageToAnthropicWire(m)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

func messageToAnthropicWire(m Message) (anthropicWireMessage, error) {
	switch m.Role {
	case "assistant":
		if len(m.ToolCalls) == 0 {
			return anthropicWireMessage{Role: "assistant", Content: m.Content}, nil
		}
		blocks, err := anthropicAssistantBlocks(m.Content, m.ToolCalls)
		if err != nil {
			return anthropicWireMessage{}, err
		}
		return anthropicWireMessage{Role: "assistant", Content: blocks}, nil
	case "user":
		if len(m.ToolResults) == 0 {
			return anthropicWireMessage{Role: "user", Content: m.Content}, nil
		}
		blocks, err := anthropicUserToolResultBlocks(m.ToolResults, m.Content)
		if err != nil {
			return anthropicWireMessage{}, err
		}
		return anthropicWireMessage{Role: "user", Content: blocks}, nil
	default:
		return anthropicWireMessage{Role: m.Role, Content: m.Content}, nil
	}
}

func anthropicAssistantBlocks(text string, calls []ToolCallRecord) ([]map[string]any, error) {
	capBlocks := len(calls)
	if text != "" {
		capBlocks++
	}
	blocks := make([]map[string]any, 0, capBlocks)
	if text != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": text})
	}
	for _, tc := range calls {
		var inputObj any
		in := tc.Input
		if in == "" {
			in = "{}"
		}
		if err := json.Unmarshal([]byte(in), &inputObj); err != nil {
			return nil, fmt.Errorf("tool %q input is not valid JSON: %w", tc.Name, err)
		}
		blocks = append(blocks, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Name,
			"input": inputObj,
		})
	}
	return blocks, nil
}

func anthropicUserToolResultBlocks(results []ToolResultRecord, extraText string) ([]map[string]any, error) {
	capBlocks := len(results)
	if extraText != "" {
		capBlocks++
	}
	blocks := make([]map[string]any, 0, capBlocks)
	if extraText != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": extraText})
	}
	for _, tr := range results {
		bl := map[string]any{
			"type":        "tool_result",
			"tool_use_id": tr.ToolUseID,
			"content":     tr.Content,
		}
		if tr.IsError {
			bl["is_error"] = true
		}
		blocks = append(blocks, bl)
	}
	return blocks, nil
}
