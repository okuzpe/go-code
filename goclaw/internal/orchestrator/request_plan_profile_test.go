package orchestrator

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/okuzpe/goclaw/testutil/mockopenai"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestPlanProfileIncludesOverrideMarker(t *testing.T) {
	srv := mockopenai.New(nil)
	defer srv.Close()

	o := New(
		testOrchestratorConfig(),
		testOpenAIClient(srv),
		session.New(),
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.Plan,
	)
	req := o.buildRequest()
	require.Contains(t, req.System, planProfileModeMarker)
	require.Contains(t, req.System, "Critical files (3–5)")
	require.Contains(t, req.System, "Clarify only when blocked")
	require.Contains(t, req.System, "Review gate:")
	require.Contains(t, req.System, "implement now, adjust the plan, or save/review first")
}

func TestBuildRequestGeneralPurposeOmitsPlanOverrideMarker(t *testing.T) {
	srv := mockopenai.New(nil)
	defer srv.Close()

	o := newOrch(t, testOpenAIClient(srv))
	req := o.buildRequest()
	require.NotContains(t, req.System, planProfileModeMarker)
}
