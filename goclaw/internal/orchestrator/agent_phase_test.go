package orchestrator

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/stretchr/testify/require"
)

func TestThinkingPhaseLine_fastIsNeutral(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Thinking", ThinkingPhaseLine(0, "fast", PhaseContext{}))
	require.Equal(t, "Exploring", ThinkingPhaseLine(0, "explore", PhaseContext{}))
	require.Equal(t, "Analyzing repository", ThinkingPhaseLine(0, "code", PhaseContext{}))
	require.Equal(t, "Planning", ThinkingPhaseLine(0, "default", PhaseContext{}))
}

func TestClassifyTaskRoleRules_greetingUsesFast(t *testing.T) {
	t.Parallel()
	require.Equal(t, "fast", classifyTaskRoleRules("hola", agents.GeneralPurpose))
	require.Equal(t, "fast", classifyTaskRoleRules("hi", agents.GeneralPurpose))
	require.Equal(t, "fast", classifyTaskRoleRules("hola\n¿qué tal?", agents.GeneralPurpose))
	require.Equal(t, "code", classifyTaskRoleRules("hi\n\nplease implement auth", agents.GeneralPurpose))
	require.Equal(t, "code", classifyTaskRoleRules("good code style question", agents.GeneralPurpose))
}
