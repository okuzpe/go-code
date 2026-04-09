package orchestrator

import (
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/stretchr/testify/require"
)

func TestTruncateForJSONOutput(t *testing.T) {
	t.Parallel()
	short := "hello"
	require.Equal(t, short, truncateForJSONOutput(short))
	long := strings.Repeat("x", maxJSONToolResultRunes+10)
	out := truncateForJSONOutput(long)
	require.Contains(t, out, "truncated")
	require.LessOrEqual(t, len([]rune(out)), maxJSONToolResultRunes+30)
}

func TestAppendJSONToolTrace(t *testing.T) {
	t.Parallel()
	var trace []JSONToolCall
	pending := []llm.ToolUse{{ID: "1", Name: "read_file", Input: `{"path":"a"}`}}
	results := []llm.ToolResultRecord{{ToolUseID: "1", ToolName: "read_file", Content: "ok", IsError: false}}
	appendJSONToolTrace(&trace, pending, results)
	require.Len(t, trace, 1)
	require.Equal(t, "1", trace[0].ID)
	require.Equal(t, "read_file", trace[0].Name)
	require.Equal(t, `{"path":"a"}`, trace[0].Input)
	require.Equal(t, "ok", trace[0].Result)
}
