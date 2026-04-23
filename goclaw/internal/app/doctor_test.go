package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/mcp"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/stretchr/testify/require"
)

func TestDoctorReportNilRuntime(t *testing.T) {
	got := DoctorReportFromRuntime(context.Background(), nil)
	require.Equal(t, "doctor: no runtime", got)
}

type stubMCPConn struct{}

func (stubMCPConn) Initialize(context.Context) error { return nil }

func (stubMCPConn) ListTools(context.Context) ([]mcp.ToolInfo, error) { return nil, nil }

func (stubMCPConn) CallTool(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}

func (stubMCPConn) Close() error { return nil }

func TestFormatMCPServerLine(t *testing.T) {
	connected := map[string]struct{}{"ok": {}}

	t.Run("disabled", func(t *testing.T) {
		ln := formatMCPServerLine(config.MCPServerConfig{ID: "x", Command: "cmd", Disabled: true}, false, connected)
		require.Contains(t, ln, "disabled")
		require.Contains(t, ln, "id=x")
	})
	t.Run("invalid", func(t *testing.T) {
		ln := formatMCPServerLine(config.MCPServerConfig{ID: "", Command: "c"}, false, connected)
		require.Contains(t, ln, "invalid")
	})
	t.Run("invalid_missing_command_and_url", func(t *testing.T) {
		ln := formatMCPServerLine(config.MCPServerConfig{ID: "x", Command: ""}, false, connected)
		require.Contains(t, ln, "invalid")
	})
	t.Run("http_url", func(t *testing.T) {
		ln := formatMCPServerLine(config.MCPServerConfig{ID: "ok", URL: "http://127.0.0.1:9/mcp"}, false, connected)
		require.Contains(t, ln, "connected")
		require.Contains(t, ln, "url=http://127.0.0.1:9/mcp")
	})
	t.Run("connected", func(t *testing.T) {
		ln := formatMCPServerLine(config.MCPServerConfig{ID: "ok", Command: "npx", Args: []string{"-y", "pkg"}}, false, connected)
		require.Contains(t, ln, "connected")
		require.Contains(t, ln, "cmd=npx")
	})
	t.Run("not_connected", func(t *testing.T) {
		ln := formatMCPServerLine(config.MCPServerConfig{ID: "missing", Command: "npx"}, false, connected)
		require.Contains(t, ln, "not connected")
	})
	t.Run("tools_disabled", func(t *testing.T) {
		ln := formatMCPServerLine(config.MCPServerConfig{ID: "ok", Command: "npx"}, true, connected)
		require.Contains(t, ln, "not started")
		require.Contains(t, ln, "tools disabled")
	})
}

func TestDoctorReportToolPermissionsEffective(t *testing.T) {
	reg := tools.New()
	reg.Register(tools.NewReadFile(t.TempDir()))

	pol := permissions.NewPolicy()
	require.NoError(t, pol.ApplyConfigModes(map[string]string{"read_file": "allow"}))

	rt := &ChatRuntime{
		Cfg:          config.Config{Provider: "ollama", OllamaHost: "http://127.0.0.1:9"},
		Workdir:      t.TempDir(),
		Sess:         session.New(),
		Policy:       pol,
		Reg:          reg,
		DisableTools: true,
	}
	report := DoctorReportFromRuntime(context.Background(), rt)
	require.Contains(t, report, "read_file: allow")
	// Built-ins still appear in registry when tools disabled? Reg is populated in PrepareChatRuntime only with tools - here we manually set Reg with one tool. OK.
}

func TestDecisionLabel(t *testing.T) {
	require.Equal(t, "allow", decisionLabel(permissions.DecisionAllow))
	require.Equal(t, "deny", decisionLabel(permissions.DecisionDeny))
	require.Equal(t, "ask", decisionLabel(permissions.DecisionAsk))
}

func TestMcpSummaryEligibleVsConnected(t *testing.T) {
	rt := &ChatRuntime{
		Cfg: config.Config{
			MCPServers: []config.MCPServerConfig{
				{ID: "a", Command: "true"},
				{ID: "b", Command: "true"},
			},
		},
		McpSessions:     []mcp.Conn{stubMCPConn{}},
		McpConnectedIDs: []string{"a"},
		DisableTools:    false,
	}
	lines := mcpSummaryLines(rt)
	text := strings.Join(lines, "\n")
	require.Contains(t, text, "mcp servers configured (eligible): 2")
	require.Contains(t, text, "mcp sessions connected: 1")
	require.Contains(t, text, "note: one or more MCP")
}

func TestMcpConnectionHintLinesNonLoopbackURL(t *testing.T) {
	rt := &ChatRuntime{
		Cfg: config.Config{
			MCPServers: []config.MCPServerConfig{
				{ID: "remote", URL: "https://example.com/mcp"},
			},
		},
		DisableTools: false,
	}
	h := mcpConnectionHintLines(rt.Cfg, rt)
	require.NotEmpty(t, h)
	require.Contains(t, strings.Join(h, "\n"), "mcp_allow_remote_urls")
}

func TestDoctorReportPluginSkillMemoryLines(t *testing.T) {
	wd := t.TempDir()
	ud := t.TempDir()
	memDir := filepath.Join(ud, "memory")
	require.NoError(t, os.MkdirAll(memDir, 0o700))
	mem := memory.New(memDir)
	_, err := mem.Save(memory.Entry{Name: "n", Type: memory.TypeUser, Body: "x"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte("# index\n"), 0o600))

	rt := &ChatRuntime{
		Cfg: config.Config{
			Provider:         "ollama",
			OllamaHost:       "http://127.0.0.1:9",
			UserConfigDir:    ud,
			PluginDirs:       []string{filepath.Join(wd, "plugins")},
			ProjectConfigDir: ".goclaw",
		},
		Workdir:      wd,
		Sess:         session.New(),
		MemStore:     mem,
		DisableTools: true,
	}
	report := DoctorReportFromRuntime(context.Background(), rt)
	require.Contains(t, report, "plugins / skills / memory:")
	require.Contains(t, report, "skill search roots:")
	require.Contains(t, report, "project tree skills snippet")
	require.Contains(t, report, "memory entries")
	require.Contains(t, report, "MEMORY.md present")
}

func TestMcpSummaryEligibleURLServers(t *testing.T) {
	rt := &ChatRuntime{
		Cfg: config.Config{
			MCPServers: []config.MCPServerConfig{
				{ID: "h", URL: "http://127.0.0.1:9/mcp"},
			},
		},
		DisableTools: false,
	}
	lines := mcpSummaryLines(rt)
	text := strings.Join(lines, "\n")
	require.Contains(t, text, "mcp servers configured (eligible): 1")
}

func TestProfileHintLinesSpawnAgentProfileUsesCoordinatorExample(t *testing.T) {
	lines := profileHintLines(agents.All()["coordinator"])
	text := strings.Join(lines, "\n")
	require.Contains(t, text, "profile coordinator")
	require.NotContains(t, text, "profile general-purpose or stack-coder")
}
