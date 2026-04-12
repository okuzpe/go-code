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

func TestRunToolInvocationAllow(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})

	pol := permissions.NewPolicy()
	pol.Set("read_file", permissions.ModeAllow)

	var afterName string
	o := New(
		config.Default(),
		nil,
		session.New(),
		reg,
		pol,
		hooks.New(),
		agents.Explore,
		WithAfterTool(func(toolName string, toolInput string, resultBytes int, isError bool) {
			afterName = toolName
		}),
	)

	content, isError, err := o.RunToolInvocation(context.Background(), "read_file", "{}", nil)
	require.NoError(t, err)
	require.False(t, isError)
	require.Equal(t, "", content)
	require.Equal(t, "read_file", afterName)
}

func TestRunToolInvocationDeny(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "bash"})

	pol := permissions.NewPolicy()
	pol.Set("bash", permissions.ModeDeny)

	o := New(
		config.Default(),
		nil,
		session.New(),
		reg,
		pol,
		hooks.New(),
		agents.GeneralPurpose,
	)

	content, isError, err := o.RunToolInvocation(context.Background(), "bash", `{"command":"echo hi"}`, nil)
	require.NoError(t, err)
	require.True(t, isError)
	require.Contains(t, content, "permission denied")
}

func TestRunToolInvocationEmptyName(t *testing.T) {
	o := New(
		config.Default(),
		nil,
		session.New(),
		tools.New(),
		permissions.NewPolicy(),
		hooks.New(),
		agents.Explore,
	)
	_, _, err := o.RunToolInvocation(context.Background(), "", "{}", nil)
	require.Error(t, err)
}
