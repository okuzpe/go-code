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
	// "fix" keywords take priority over code keywords — fix the panic is an action task.
	require.Equal(t, "fix", classifyTaskRoleRules("fix the panic in main()", agents.GeneralPurpose))
	// Pure code block with no fix/review intent → code role.
	require.Equal(t, "code", classifyTaskRoleRules("```go\nfunc main(){}\n```", agents.GeneralPurpose))
	// Reasoning with no fix intent → reasoning role.
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
		profile: agents.Builder,
		ut:      &userTurnState{},
	}

	// "implement" triggers code role (not fix).
	o.prepareTurnModel(context.Background(), "implement a function to parse JSON")
	require.Equal(t, "coder:14b", o.ut.turnModel)

	// general-purpose fallback is "default"; short vague prompts use the fast path → default task model.
	o.ut.turnModel = ""
	o.prepareTurnModel(context.Background(), "synthetic vague message that matches no strong keyword")
	require.Equal(t, "fallback:7b", o.ut.turnModel)

	o.ut.turnModel = ""
	o.prepareTurnModel(context.Background(), "hola")
	require.Equal(t, "fallback:7b", o.ut.turnModel)
}

func TestPrepareTurnModel_AmbiguousDefaultUsesCodeModelWhenMapped(t *testing.T) {
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
		profile: agents.Builder,
		ut:      &userTurnState{},
	}

	// Empty message stays at rules "default" after low-confidence collapse; upgrade to code tier when configured.
	o.prepareTurnModel(context.Background(), "")
	require.Equal(t, "code", o.ut.taskRole)
	require.Equal(t, "coder:14b", o.ut.turnModel)

	o.ut.turnModel = ""
	o.ut.taskRole = ""
	o.profile = agents.Explore
	o.prepareTurnModel(context.Background(), "")
	require.NotEqual(t, "code", o.ut.taskRole, "read-only explore must not get ambiguous_default->code")
}

func TestBuildRequestTurnModelOverridesGlobal(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "global:latest"

	o := &Orchestrator{
		cfg:     cfg,
		session: session.New(),
		tools:   tools.New(),
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.GeneralPurpose,
		ut:      &userTurnState{turnModel: "worker:tag"},
	}
	req := o.buildRequest()
	require.Equal(t, "worker:tag", req.Model)
}

func TestPrepareTurnModel_BuildLiteKeepsSingleModel(t *testing.T) {
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
		ut:      &userTurnState{},
	}

	o.prepareTurnModel(context.Background(), "implement a function to parse JSON")
	require.Equal(t, "code", o.ut.taskRole)
	require.Equal(t, "", o.ut.turnModel)
}
