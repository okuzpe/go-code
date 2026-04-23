package app

import (
	"context"
	"errors"
	"testing"

	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/testutil/mockopenai"
	"github.com/stretchr/testify/require"
)

func TestFullscreenSubmitterAugmentsActionStalledError(t *testing.T) {
	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "please fix this", Response: "I don't have access to your terminal session."},
		{Match: "[goclaw] The user asked for code or repository changes.", Response: "I don't have access to your terminal session."},
		{Match: "[goclaw] Action nudges were exhausted without native tool calls.", Response: "I don't have access to your terminal session."},
	})
	defer srv.Close()

	rt := newActionStallRuntime(appTestOpenAIClient(srv))
	orch := orchestrator.New(rt.Cfg, rt.Client, rt.Sess, rt.Reg, rt.Policy, rt.HookReg, rt.Profile)
	submit := fullscreenNewChatSubmitter(rt, orch, coordinator.NewFocusRouter())

	_, err := submit(context.Background(), "please fix this", "", nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, orchestrator.ErrActionStalled))
	require.Contains(t, err.Error(), "failed to take real tool-driven action")
}
