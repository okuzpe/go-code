package app

import (
	"context"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/mcp"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/stretchr/testify/require"
)

func TestDoctorReportNilRuntime(t *testing.T) {
	got := DoctorReportFromRuntime(context.Background(), nil)
	require.Equal(t, "doctor: no runtime", got)
}

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
		McpSessions:     []*mcp.Session{new(mcp.Session)},
		McpConnectedIDs: []string{"a"},
		DisableTools:    false,
	}
	lines := mcpSummaryLines(rt)
	text := strings.Join(lines, "\n")
	require.Contains(t, text, "mcp servers configured (eligible): 2")
	require.Contains(t, text, "mcp sessions connected: 1")
	require.Contains(t, text, "note: one or more MCP")
}
