package chat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderAgentStatus_ThinkingDoesNotDuplicateDefaultLabel(t *testing.T) {
	t.Parallel()

	got := stripANSI(RenderAgentStatus(
		DefaultTheme(),
		AgentStateThinking,
		"*",
		"",
		5,
		"",
		"",
		0,
		0,
		"",
	))

	require.Contains(t, got, "Thinking")
	require.Contains(t, got, "(5s)")
	require.Equal(t, 1, strings.Count(got, "Thinking"))
}

func TestRenderAgentStatus_ThinkingKeepsSpecificPhaseLabel(t *testing.T) {
	t.Parallel()

	got := stripANSI(RenderAgentStatus(
		DefaultTheme(),
		AgentStateThinking,
		"*",
		"[2/3] Planning next step",
		5,
		"",
		"",
		0,
		0,
		"",
	))

	require.Contains(t, got, "[2/3]")
	require.Contains(t, got, "Planning next step")
	require.Contains(t, got, "(5s)")
	require.NotContains(t, got, "thinking")
}
