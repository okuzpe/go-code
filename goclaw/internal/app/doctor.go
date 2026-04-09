package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/mcp"
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
	webBackend, webBackendOK := config.NormalizeWebSearchBackend(cfg.WebSearchBackend)
	if !webBackendOK && strings.TrimSpace(cfg.WebSearchBackend) != "" {
		lines = append(lines, fmt.Sprintf("web_search: unknown backend %q (using ddg)", strings.TrimSpace(cfg.WebSearchBackend)))
	} else {
		lines = append(lines, fmt.Sprintf("web_search backend: %s", webBackend))
	}
	if webBackend == "brave" {
		lines = append(lines, checkLine("brave search api key configured", strings.TrimSpace(cfg.BraveSearchAPIKey) != ""))
	}
	if webBackend == "serpapi" {
		lines = append(lines, checkLine("serpapi key configured", strings.TrimSpace(cfg.SerpAPIKey) != ""))
	}
	if webBackend != "ddg" {
		lines = append(lines, fmt.Sprintf("  - fallback to duckduckgo: %v", cfg.WebSearchFallbackDDG))
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
		mode := strings.ToLower(strings.TrimSpace(cfg.TokenCountMode))
		if mode == "" {
			mode = "auto"
		}
		lines = append(lines, fmt.Sprintf("token_count_mode: %s (auto uses count_tokens API near compaction threshold)", mode))
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
	hintLines = append(hintLines, mcpConnectionHintLines(cfg, rt)...)
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
		if srv.Disabled || strings.TrimSpace(srv.ID) == "" {
			continue
		}
		if strings.TrimSpace(srv.Command) == "" && strings.TrimSpace(srv.URL) == "" {
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

func mcpConnectionHintLines(cfg config.Config, rt *ChatRuntime) []string {
	if rt.DisableTools {
		return nil
	}
	var out []string
	eligible := 0
	connected := len(rt.McpSessions)
	for _, srv := range cfg.MCPServers {
		if srv.Disabled || strings.TrimSpace(srv.ID) == "" {
			continue
		}
		if strings.TrimSpace(srv.Command) == "" && strings.TrimSpace(srv.URL) == "" {
			continue
		}
		eligible++
		u := strings.TrimSpace(srv.URL)
		if u == "" {
			continue
		}
		parsed, err := url.Parse(u)
		if err != nil {
			continue
		}
		if verr := mcp.ValidateHTTPURL(parsed, false); verr != nil && !cfg.MCPServersAllowRemote {
			out = append(out, fmt.Sprintf("  MCP %q: %v — or set \"mcp_allow_remote_urls\": true if you trust this host.", srv.ID, verr))
		}
		host := strings.ToLower(parsed.Hostname())
		if strings.HasPrefix(strings.ToLower(u), "https://") && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
			out = append(out, "  HTTPS MCP on loopback: ensure the certificate is trusted, or use http:// for local dev.")
		}
	}
	if eligible > 0 && connected < eligible {
		out = append(out, "  MCP: check startup logs for initialize/tools/list errors. HTTP: confirm the server is up, path is correct, and add Authorization in mcp_servers[].headers if you need auth (401).")
	}
	return dedupeDoctorHints(out)
}

func dedupeDoctorHints(lines []string) []string {
	seen := make(map[string]struct{}, len(lines))
	var out []string
	for _, line := range lines {
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
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
	if !toolsDisabled && cfg.Provider != "anthropic" && ollamaOK {
		hints = append(hints,
			"  Local Ollama models may still refuse to summarize news or pages even when web_search/web_fetch succeed.",
			"  - Try a general instruct/chat model, or switch provider to anthropic for more reliable web summarization.",
		)
	}
	return hints
}

func mcpServerSection(rt *ChatRuntime) []string {
	out := []string{"mcp servers:"}
	servers := rt.Cfg.MCPServers
	if len(servers) == 0 {
		out = append(out, "  (none in settings.json — add \"mcp_servers\" with \"command\" and/or \"url\")")
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
	url := strings.TrimSpace(srv.URL)
	prefix := "  "
	if srv.Disabled {
		return prefix + "○ disabled  id=" + emptyPlaceholder(id) + "  " + shortCommandSummary(srv)
	}
	if id == "" || (cmd == "" && url == "") {
		return prefix + "✗ invalid  (need non-empty id and command or url)  id=" + emptyPlaceholder(id)
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
	if u := strings.TrimSpace(srv.URL); u != "" {
		if len(u) > 72 {
			u = u[:69] + "..."
		}
		return "url=" + u
	}
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
