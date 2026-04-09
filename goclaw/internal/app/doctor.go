package app

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/spf13/cobra"
)

// RunDoctor prints a short preflight report and exits.
func RunDoctor(cmd *cobra.Command, _ []string) error {
	rt, err := PrepareChatRuntime(cmd)
	if err != nil {
		return err
	}
	fmt.Println(DoctorReportFromRuntime(context.Background(), rt))
	return nil
}

func DoctorReportFromRuntime(ctx context.Context, rt *ChatRuntime) string {
	if rt == nil {
		return "doctor: no runtime"
	}
	cfg := rt.Cfg
	lines := []string{
		"goclaw doctor",
		"",
		fmt.Sprintf("workspace: %s", rt.Workdir),
		fmt.Sprintf("session:   %s", rt.Sess.ID),
		fmt.Sprintf("provider:  %s", cfg.Provider),
		fmt.Sprintf("model:     %s", cfg.Model()),
		fmt.Sprintf("profile:   %s", rt.Profile.Name),
		fmt.Sprintf("tools:     %s", enabledDisabled(!rt.DisableTools)),
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("user config dir: %s", cfg.UserConfigDir))
	lines = append(lines, fmt.Sprintf("sessions dir:    %s", filepath.Join(cfg.UserConfigDir, "sessions")))
	lines = append(lines, fmt.Sprintf("memory dir:      %s", filepath.Join(cfg.UserConfigDir, "memory")))

	lines = append(lines, "")
	lines = append(lines, "checks:")
	var ollamaOK bool
	if cfg.Provider == "anthropic" {
		lines = append(lines, checkLine("anthropic api key", strings.TrimSpace(cfg.APIKey) != ""))
	} else {
		ollamaHost := effectiveOllamaHost(cfg.OllamaHost)
		ollamaOK = probeOllama(ctx, ollamaHost)
		lines = append(lines, checkLine("ollama host reachable", ollamaOK))
		lines = append(lines, fmt.Sprintf("  - ollama host: %s", ollamaHost))
	}

	if rt.Store != nil {
		lines = append(lines, checkLine("session store initialized", true))
	} else {
		lines = append(lines, checkLine("session store initialized", false))
	}

	lines = append(lines, mcpSummaryLines(rt)...)

	hintLines := hintLines(cfg, ollamaOK, rt.DisableTools)
	if len(hintLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, "hints:")
		lines = append(lines, hintLines...)
	}

	lines = append(lines, "")
	lines = append(lines, mcpServerSection(rt)...)

	lines = append(lines, "")
	lines = append(lines, toolPermissionSection(rt)...)

	return strings.Join(lines, "\n")
}

func mcpSummaryLines(rt *ChatRuntime) []string {
	eligible := 0
	for _, srv := range rt.Cfg.MCPServers {
		if srv.Disabled || strings.TrimSpace(srv.ID) == "" || strings.TrimSpace(srv.Command) == "" {
			continue
		}
		eligible++
	}
	connected := len(rt.McpSessions)
	out := []string{
		checkLine("mcp configured servers reachable", !rt.DisableTools || eligible == 0 || connected == eligible),
		fmt.Sprintf("  - mcp servers configured (eligible): %d", eligible),
		fmt.Sprintf("  - mcp sessions connected: %d", connected),
	}
	if !rt.DisableTools && eligible > 0 && connected < eligible {
		out = append(out, "  - note: one or more MCP servers failed to start; see log output above")
	}
	return out
}

func hintLines(cfg config.Config, ollamaOK, toolsDisabled bool) []string {
	var hints []string
	if cfg.Provider != "anthropic" && !ollamaOK {
		host := effectiveOllamaHost(cfg.OllamaHost)
		hints = append(hints,
			fmt.Sprintf("  Ollama did not respond at %s within the probe timeout.", host),
			"  - Start the daemon (example: ollama serve) or install Ollama if it is missing.",
			"  - If it listens elsewhere, set OLLAMA_HOST in the environment or \"ollama_host\" in settings.json.",
			"  - Confirm the model is pulled: ollama pull "+strings.TrimSpace(cfg.OllamaModel),
		)
	}
	if toolsDisabled {
		hints = append(hints, "  Tools are disabled (--no-tools or GOCLAW_DISABLE_TOOLS=1); MCP servers were not started.")
	}
	return hints
}

func mcpServerSection(rt *ChatRuntime) []string {
	out := []string{"mcp servers:"}
	servers := rt.Cfg.MCPServers
	if len(servers) == 0 {
		out = append(out, "  (none in settings.json — add \"mcp_servers\" array to configure stdio servers)")
		return out
	}
	connected := make(map[string]struct{}, len(rt.McpConnectedIDs))
	for _, id := range rt.McpConnectedIDs {
		connected[id] = struct{}{}
	}
	for _, srv := range servers {
		out = append(out, formatMCPServerLine(srv, rt.DisableTools, connected))
	}
	return out
}

func formatMCPServerLine(srv config.MCPServerConfig, toolsDisabled bool, connected map[string]struct{}) string {
	id := strings.TrimSpace(srv.ID)
	cmd := strings.TrimSpace(srv.Command)
	prefix := "  "
	if srv.Disabled {
		return prefix + "○ disabled  id=" + emptyPlaceholder(id) + "  " + shortCommandSummary(srv)
	}
	if id == "" || cmd == "" {
		return prefix + "✗ invalid  (need non-empty id and command)  id=" + emptyPlaceholder(id)
	}
	if toolsDisabled {
		return prefix + "○ not started  id=" + id + "  (tools disabled)  " + shortCommandSummary(srv)
	}
	if _, ok := connected[id]; ok {
		return prefix + "✓ connected  id=" + id + "  " + shortCommandSummary(srv)
	}
	return prefix + "✗ not connected  id=" + id + "  " + shortCommandSummary(srv)
}

func emptyPlaceholder(s string) string {
	if s == "" {
		return "(empty)"
	}
	return s
}

func shortCommandSummary(srv config.MCPServerConfig) string {
	cmd := strings.TrimSpace(srv.Command)
	var b strings.Builder
	b.WriteString("cmd=")
	b.WriteString(cmd)
	if len(srv.Args) > 0 {
		b.WriteString(" ")
		args := strings.Join(srv.Args, " ")
		if len(args) > 72 {
			args = args[:69] + "..."
		}
		b.WriteString(args)
	}
	if srv.CWD != "" {
		b.WriteString("  cwd=" + srv.CWD)
	}
	return b.String()
}

func toolPermissionSection(rt *ChatRuntime) []string {
	out := []string{"tool permissions (effective):"}
	if rt.Policy == nil || rt.Reg == nil {
		out = append(out, "  (unavailable)")
		return out
	}
	specs := rt.Reg.Specs()
	if len(specs) == 0 {
		out = append(out, "  (no tools registered)")
		return out
	}
	for _, sp := range specs {
		label := decisionLabel(rt.Policy.Evaluate(sp.Name))
		out = append(out, fmt.Sprintf("  %s: %s", sp.Name, label))
	}
	if len(rt.Cfg.PermissionModes) == 0 {
		out = append(out, "  (no tool_permissions in settings — all of the above use the default: ask)")
	} else {
		out = append(out, "  (entries in settings.json override defaults only for named tools)")
	}
	return out
}

func decisionLabel(d permissions.Decision) string {
	switch d {
	case permissions.DecisionAllow:
		return "allow"
	case permissions.DecisionDeny:
		return "deny"
	default:
		return "ask"
	}
}

func enabledDisabled(ok bool) string {
	if ok {
		return "enabled"
	}
	return "disabled"
}

func checkLine(name string, ok bool) string {
	if ok {
		return "  ✓ " + name
	}
	return "  ✗ " + name
}

func effectiveOllamaHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return "http://localhost:11434"
	}
	return host
}

func probeOllama(ctx context.Context, host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "http://localhost:11434"
	}
	u := strings.TrimRight(host, "/") + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 900 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

// Minimal compile-time guard: DoctorReportFromRuntime expects fields set by PrepareChatRuntime.
var _ = config.Config{}
var _ = session.Store{}
