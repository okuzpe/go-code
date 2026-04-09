package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// --- stripCodeFences ---

func TestStripCodeFences_noFence(t *testing.T) {
	require.Equal(t, `{"key":"val"}`, stripCodeFences(`{"key":"val"}`))
}

func TestStripCodeFences_jsonFence(t *testing.T) {
	input := "```json\n{\"key\":\"val\"}\n```"
	require.Equal(t, `{"key":"val"}`, stripCodeFences(input))
}

func TestStripCodeFences_plainFence(t *testing.T) {
	input := "```\nhello\n```"
	require.Equal(t, "hello", stripCodeFences(input))
}

func TestStripCodeFences_noClosingFence(t *testing.T) {
	// Should still return what's after the opening line.
	input := "```json\n{\"key\":\"val\"}"
	result := stripCodeFences(input)
	require.Contains(t, result, "key")
}

func TestStripCodeFences_fenceNoNewline(t *testing.T) {
	// "```" with no newline: returns as-is (no inner content).
	require.Equal(t, "```", stripCodeFences("```"))
}

// --- findMatchingBrace ---

func TestFindMatchingBrace_simple(t *testing.T) {
	s := `{"a":1}`
	idx := findMatchingBrace(s, 0)
	require.Equal(t, len(s)-1, idx)
}

func TestFindMatchingBrace_nested(t *testing.T) {
	s := `{"a":{"b":2}}`
	idx := findMatchingBrace(s, 0)
	require.Equal(t, len(s)-1, idx)
}

func TestFindMatchingBrace_noMatch(t *testing.T) {
	s := `{"a":1`
	idx := findMatchingBrace(s, 0)
	require.Equal(t, -1, idx)
}

func TestFindMatchingBrace_notStartingAtBrace(t *testing.T) {
	s := `prefix{"a":1}`
	// Start at index 6 where '{' is.
	idx := findMatchingBrace(s, 6)
	require.Equal(t, len(s)-1, idx)
}

// --- extractEmbeddedToolCall ---

func TestExtractEmbeddedToolCall_noMatch(t *testing.T) {
	_, _, ok := extractEmbeddedToolCall("just plain text", nil)
	require.False(t, ok)
}

func TestExtractEmbeddedToolCall_namePattern(t *testing.T) {
	specs := []ToolSpec{{Name: "bash", Description: "run", InputSchema: map[string]any{}}}
	body := `{"name":"bash","arguments":{"command":"ls"}}`
	tu, _, ok := extractEmbeddedToolCall(body, specs)
	require.True(t, ok)
	require.Equal(t, "bash", tu.Name)
}

func TestExtractEmbeddedToolCall_inCodeFence(t *testing.T) {
	specs := []ToolSpec{{Name: "read_file", Description: "read", InputSchema: map[string]any{}}}
	body := "```json\n{\"name\":\"read_file\",\"arguments\":{\"path\":\"x\"}}\n```"
	tu, _, ok := extractEmbeddedToolCall(body, specs)
	require.True(t, ok)
	require.Equal(t, "read_file", tu.Name)
}

func TestExtractEmbeddedToolCall_unknownTool(t *testing.T) {
	specs := []ToolSpec{{Name: "bash", Description: "run", InputSchema: map[string]any{}}}
	body := `{"name":"unknown_tool","arguments":{}}`
	_, _, ok := extractEmbeddedToolCall(body, specs)
	require.False(t, ok)
}

func TestExtractEmbeddedToolCall_emptyBody(t *testing.T) {
	_, _, ok := extractEmbeddedToolCall("", nil)
	require.False(t, ok)
}
