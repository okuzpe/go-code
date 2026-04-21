package planfile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStepExecutionUserMessages(t *testing.T) {
	t.Parallel()
	steps := []string{"First", "Second"}
	msgs := StepExecutionUserMessages("/tmp/ws/.goclaw/plan.md", "# Plan\nbody", steps, HandoffOptions{})
	require.Len(t, msgs, 2)
	require.Contains(t, msgs[0], "[goclaw plan execution 1/2]")
	require.Contains(t, msgs[0], "## Step")
	require.Contains(t, msgs[0], "First")
	require.Contains(t, msgs[1], "[goclaw plan execution 2/2]")
	require.Contains(t, msgs[1], "Second")
}

func TestStepExecutionUserMessagesCoordinatorHint(t *testing.T) {
	t.Parallel()
	msgs := StepExecutionUserMessages("p.md", "body", []string{"Do X"}, HandoffOptions{UseCoordinator: true})
	require.Len(t, msgs, 1)
	require.True(t, strings.Contains(msgs[0], "spawn_agent"))
}
