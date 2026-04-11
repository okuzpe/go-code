package session

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/stretchr/testify/require"
)

func TestPlainTranscript_userAssistant(t *testing.T) {
	s := New()
	s.Add("user", "hi")
	s.AddAssistant("hello", nil)
	out := s.PlainTranscript()
	require.Contains(t, out, "[user]")
	require.Contains(t, out, "hi")
	require.Contains(t, out, "[assistant]")
	require.Contains(t, out, "hello")
}

func TestPlainTranscript_toolRoundTrip(t *testing.T) {
	s := New()
	s.AddAssistant("", []llm.ToolCallRecord{{ID: "1", Name: "bash", Input: `{"command":"echo"}`}})
	s.AddToolResults([]llm.ToolResultRecord{{ToolUseID: "1", ToolName: "bash", Content: "ok\n"}})
	out := s.PlainTranscript()
	require.Contains(t, out, "tool_use")
	require.Contains(t, out, "bash")
	require.Contains(t, out, "tool_results")
}
