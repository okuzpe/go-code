package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageToAnthropicWire_plainUser(t *testing.T) {
	m := Message{Role: "user", Content: "hello"}
	w, err := messageToAnthropicWire(m)
	require.NoError(t, err)
	require.Equal(t, "user", w.Role)
	require.Equal(t, "hello", w.Content)
}

func TestMessageToAnthropicWire_plainAssistant(t *testing.T) {
	m := Message{Role: "assistant", Content: "response"}
	w, err := messageToAnthropicWire(m)
	require.NoError(t, err)
	require.Equal(t, "assistant", w.Role)
	require.Equal(t, "response", w.Content)
}

func TestMessageToAnthropicWire_assistantWithToolCall(t *testing.T) {
	m := Message{
		Role:    "assistant",
		Content: "I'll call a tool",
		ToolCalls: []ToolCallRecord{
			{ID: "call_1", Name: "read_file", Input: `{"path":"foo.go"}`},
		},
	}
	w, err := messageToAnthropicWire(m)
	require.NoError(t, err)
	require.Equal(t, "assistant", w.Role)
	blocks, ok := w.Content.([]map[string]any)
	require.True(t, ok, "expected blocks slice")
	require.Len(t, blocks, 2)
	require.Equal(t, "text", blocks[0]["type"])
	require.Equal(t, "tool_use", blocks[1]["type"])
	require.Equal(t, "call_1", blocks[1]["id"])
	require.Equal(t, "read_file", blocks[1]["name"])
}

func TestMessageToAnthropicWire_assistantToolCallNoText(t *testing.T) {
	m := Message{
		Role: "assistant",
		ToolCalls: []ToolCallRecord{
			{ID: "call_2", Name: "bash", Input: `{"command":"ls"}`},
		},
	}
	w, err := messageToAnthropicWire(m)
	require.NoError(t, err)
	blocks, ok := w.Content.([]map[string]any)
	require.True(t, ok)
	require.Len(t, blocks, 1, "no text block when Content is empty")
	require.Equal(t, "tool_use", blocks[0]["type"])
}

func TestMessageToAnthropicWire_assistantToolCallInvalidJSON(t *testing.T) {
	m := Message{
		Role: "assistant",
		ToolCalls: []ToolCallRecord{
			{ID: "call_3", Name: "bash", Input: `{bad`},
		},
	}
	_, err := messageToAnthropicWire(m)
	require.Error(t, err)
}

func TestMessageToAnthropicWire_assistantToolCallEmptyInput(t *testing.T) {
	// Empty input should be treated as {}
	m := Message{
		Role: "assistant",
		ToolCalls: []ToolCallRecord{
			{ID: "call_4", Name: "bash", Input: ""},
		},
	}
	_, err := messageToAnthropicWire(m)
	require.NoError(t, err)
}

func TestMessageToAnthropicWire_userWithToolResults(t *testing.T) {
	m := Message{
		Role: "user",
		ToolResults: []ToolResultRecord{
			{ToolUseID: "call_1", ToolName: "read_file", Content: "file content"},
		},
	}
	w, err := messageToAnthropicWire(m)
	require.NoError(t, err)
	blocks, ok := w.Content.([]map[string]any)
	require.True(t, ok)
	require.Len(t, blocks, 1)
	require.Equal(t, "tool_result", blocks[0]["type"])
	require.Equal(t, "call_1", blocks[0]["tool_use_id"])
}

func TestMessageToAnthropicWire_userWithToolResultIsError(t *testing.T) {
	m := Message{
		Role: "user",
		ToolResults: []ToolResultRecord{
			{ToolUseID: "call_e", ToolName: "bash", Content: "error msg", IsError: true},
		},
	}
	w, err := messageToAnthropicWire(m)
	require.NoError(t, err)
	blocks := w.Content.([]map[string]any)
	require.True(t, blocks[0]["is_error"].(bool))
}

func TestMessageToAnthropicWire_userWithResultsAndExtraText(t *testing.T) {
	m := Message{
		Role:    "user",
		Content: "extra text",
		ToolResults: []ToolResultRecord{
			{ToolUseID: "call_5", ToolName: "bash", Content: "done"},
		},
	}
	w, err := messageToAnthropicWire(m)
	require.NoError(t, err)
	blocks := w.Content.([]map[string]any)
	require.Len(t, blocks, 2)
	require.Equal(t, "text", blocks[0]["type"])
	require.Equal(t, "tool_result", blocks[1]["type"])
}

func TestMessagesToAnthropicWire_multiple(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	out, err := messagesToAnthropicWire(msgs)
	require.NoError(t, err)
	require.Len(t, out, 2)
}

func TestMessagesToAnthropicWire_propagatesError(t *testing.T) {
	msgs := []Message{
		{Role: "assistant", ToolCalls: []ToolCallRecord{{ID: "x", Name: "bash", Input: "bad{"}}},
	}
	_, err := messagesToAnthropicWire(msgs)
	require.Error(t, err)
}
