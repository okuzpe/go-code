package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/okuzpe/goclaw/internal/cli"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/mcp"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestPrepareChatRuntime_MCPServerStartFailureContinues(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	badBin := filepath.Join(t.TempDir(), "___nonexistent_mcp_binary___")
	userDir := filepath.Join(home, ".goclaw")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	mcpCfg := []map[string]any{
		{"id": "bad-a", "command": badBin},
		{"id": "bad-b", "command": badBin},
	}
	raw, err := json.Marshal(map[string]any{
		"agent_profile": "explore",
		"mcp_servers":   mcpCfg,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "settings.json"), raw, 0o600))

	cmd := cli.NewRootCmd("dev",
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command) error { return nil },
		func(*cobra.Command, []string) error { return nil },
		nil,
	)
	require.NoError(t, cmd.ParseFlags([]string{}))

	oldwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(proj))
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	rt, err := PrepareChatRuntime(cmd)
	require.NoError(t, err)
	require.Empty(t, rt.McpSessions, "failed MCP servers should not abort runtime or add sessions")
	require.NotEmpty(t, rt.ScratchDir)
	info, statErr := os.Stat(rt.ScratchDir)
	require.NoError(t, statErr)
	require.True(t, info.IsDir())
}

func TestPrepareChatRuntime_ProjectHooksOnlyWhenTrusted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell marker for project hooks test")
	}
	home := t.TempDir()
	proj := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	userDir := filepath.Join(home, ".goclaw")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"agent_profile":"explore"}`), 0o600))

	goclawDir := filepath.Join(proj, ".goclaw")
	require.NoError(t, os.MkdirAll(goclawDir, 0o700))
	marker := filepath.Join(proj, "hook_marker.txt")
	hookJSON, err := json.Marshal([]any{
		map[string]any{"event": "session_start", "command": "sh", "args": []string{"-c", "echo ok > \"" + marker + "\""}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(goclawDir, "hooks.json"), hookJSON, 0o600))

	cmd := cli.NewRootCmd("dev",
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command) error { return nil },
		func(*cobra.Command, []string) error { return nil },
		nil,
	)
	require.NoError(t, cmd.ParseFlags([]string{}))

	oldwd, werr := os.Getwd()
	require.NoError(t, werr)
	require.NoError(t, os.Chdir(proj))
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	_, err = PrepareChatRuntime(cmd)
	require.NoError(t, err)
	_, statErr := os.Stat(marker)
	require.Error(t, statErr, "hooks.json should not load when workspace is untrusted")

	trustedSettings := []byte(`{"agent_profile":"explore","trusted_workspace":true}`)
	require.NoError(t, os.WriteFile(filepath.Join(goclawDir, "settings.json"), trustedSettings, 0o600))
	_ = os.Remove(marker)

	cmd2 := cli.NewRootCmd("dev",
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command) error { return nil },
		func(*cobra.Command, []string) error { return nil },
		nil,
	)
	require.NoError(t, cmd2.ParseFlags([]string{}))
	_, err = PrepareChatRuntime(cmd2)
	require.NoError(t, err)
	_, err = os.Stat(marker)
	require.NoError(t, err, "trusted workspace should run project session_start hook")
}

func TestChatRuntimeCloseIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	scratch := filepath.Join(tmp, "scratch")
	require.NoError(t, os.MkdirAll(scratch, 0o700))

	var sessionEndN int
	hookReg := hooks.New()
	hookReg.On(hooks.SessionEnd, func(_ context.Context, _ hooks.Event) error {
		sessionEndN++
		return nil
	})

	rt := &ChatRuntime{
		HookReg:    hookReg,
		ScratchDir: scratch,
		McpSessions: []mcp.Conn{
			stubMCPConn{},
		},
	}

	rt.Close()
	rt.Close()

	require.Equal(t, 1, sessionEndN, "session_end should only fire once")
	_, err := os.Stat(scratch)
	require.Error(t, err, "scratch dir should be removed on first close")
	require.Empty(t, rt.ScratchDir)
	require.Nil(t, rt.McpSessions)
}
