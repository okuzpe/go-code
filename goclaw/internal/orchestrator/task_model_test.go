package orchestrator

import (
	"context"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/stretchr/testify/require"
)

func TestClassifyTaskRoleRules_CodeAndReasoning(t *testing.T) {
	t.Parallel()
	require.Equal(t, "code", classifyTaskRoleRules("fix the panic in main()", agents.GeneralPurpose))
	require.Equal(t, "code", classifyTaskRoleRules("```go\nfunc main(){}\n```", agents.GeneralPurpose))
	require.Equal(t, "reasoning", classifyTaskRoleRules("why might this trade-off favor ACID over eventual consistency?", agents.GeneralPurpose))
}

func TestClassifyTaskRoleRules_ProfileFallback(t *testing.T) {
	t.Parallel()
	require.Equal(t, "reasoning", classifyTaskRoleRules("ok", agents.Plan))
	require.Equal(t, "explore", classifyTaskRoleRules("ok", agents.Explore))
}

func TestParseRouterRoleJSON(t *testing.T) {
	t.Parallel()
	r, err := parseRouterRoleJSON(`Here: {"role":"code"}`)
	require.NoError(t, err)
	require.Equal(t, "code", r)

	r, err = parseRouterRoleJSON("```json\n{\"role\":\"fast\"}\n```")
	require.NoError(t, err)
	require.Equal(t, "fast", r)

	_, err = parseRouterRoleJSON("no json")
	require.Error(t, err)
}

func TestPrepareTurnModel_UsesTaskMap(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "global:latest"
	cfg.TaskModelRouter = "rules"
	cfg.TaskModels = map[string]string{
		"code":    "coder:14b",
		"default": "fallback:7b",
	}

	o := &Orchestrator{
		cfg:     cfg,
		session: session.New(),
		tools:   tools.New(),
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.GeneralPurpose,
	}

	o.prepareTurnModel(context.Background(), "refactor this function for clarity")
	require.Equal(t, "coder:14b", o.turnModel)

	o.turnModel = ""
	o.prepareTurnModel(context.Background(), "synthetic vague message that matches no strong keyword")
	require.Equal(t, "fallback:7b", o.turnModel)
}

func TestBuildRequestTurnModelOverridesGlobal(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "global:latest"

	o := &Orchestrator{
		cfg:       cfg,
		session:   session.New(),
		tools:     tools.New(),
		perms:     permissions.NewPolicy(),
		hooks:     hooks.New(),
		profile:   agents.GeneralPurpose,
		turnModel: "worker:tag",
	}
	req := o.buildRequest()
	require.Equal(t, "worker:tag", req.Model)
}
