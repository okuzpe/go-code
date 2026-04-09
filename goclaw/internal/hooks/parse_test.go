package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseEventType_allKnown(t *testing.T) {
	cases := []struct {
		in   string
		want EventType
	}{
		{"pre_tool_use", PreToolUse},
		{"post_tool_use", PostToolUse},
		{"post_tool_use_failure", PostToolUseFailure},
		{"session_start", SessionStart},
		{"session_end", SessionEnd},
		// Case insensitive
		{"PRE_TOOL_USE", PreToolUse},
		{"SESSION_END", SessionEnd},
		// Extra whitespace
		{"  post_tool_use  ", PostToolUse},
	}
	for _, c := range cases {
		got, err := ParseEventType(c.in)
		require.NoError(t, err, "input=%q", c.in)
		require.Equal(t, c.want, got, "input=%q", c.in)
	}
}

func TestParseEventType_unknownReturnsError(t *testing.T) {
	_, err := ParseEventType("on_commit")
	require.Error(t, err)
	require.Contains(t, err.Error(), "on_commit")
}
