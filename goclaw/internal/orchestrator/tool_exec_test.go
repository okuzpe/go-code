package orchestrator

import (
	"context"
	"runtime"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/stretchr/testify/require"
)

func TestExecuteToolReadOnlyBlocksMCPTool(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "mcp__demo__echo"})

	pol := permissions.NewPolicy()
	for _, n := range []string{"mcp__demo__echo"} {
		pol.Set(n, permissions.ModeAllow)
	}

	o := New(
		config.Default(),
		nil,
		session.New(),
		reg,
		pol,
		hooks.New(),
		agents.Profile{Name: "ro", ReadOnly: true},
	)

	out := o.executeTool(context.Background(), &llm.ToolUse{Name: "mcp__demo__echo", Input: "{}"}, nil)
	require.True(t, out.IsError)
	require.Contains(t, out.Content, "mcp tools are disabled")
	require.Nil(t, out.Err)
}

func TestExecuteToolPreToolUseExternalHookExit1Blocks(t *testing.T) {
	hookReg := hooks.New()
	if runtime.GOOS == "windows" {
		hookReg.OnCommand(hooks.PreToolUse, "cmd.exe", "/c", "exit /b 1")
	} else {
		hookReg.OnCommand(hooks.PreToolUse, "/bin/sh", "-c", "exit 1")
	}

	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	pol := permissions.NewPolicy()
	pol.Set("read_file", permissions.ModeAllow)

	o := New(
		config.Default(),
		nil,
		session.New(),
		reg,
		pol,
		hookReg,
		agents.Explore,
	)

	out := o.executeTool(context.Background(), &llm.ToolUse{Name: "read_file", Input: "{}"}, nil)
	require.True(t, out.IsError)
	require.Contains(t, out.Content, "pre_tool_use hook")
	require.Nil(t, out.Err)
}
