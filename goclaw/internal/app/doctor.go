package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/ide"
	"github.com/okuzpe/goclaw/internal/mcp"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/plugin"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/skills"
)

const doctorMCPDisplayMaxRunes = 72

func DoctorReportFromRuntime(_ context.Context, rt *ChatRuntime) string {
	if rt == nil {
		return "doctor: no runtime"
	}
	cfg := rt.Cfg
	taskModelsLine := "task_models: (none)"
	if n := len(cfg.TaskModels); n > 0 {
		taskModelsLine = fmt.Sprintf("task_models: %d role(s) mapped", n)
	}
	lines := []string{
		"goclaw doctor",
		"",
		fmt.Sprintf("tool path root (default project): %s", rt.Workdir),
	}
	if ld := strings.TrimSpace(rt.LaunchDir); ld != "" {
		if wd := strings.TrimSpace(rt.Workdir); filepath.Clean(ld) != filepath.Clean(wd) {
			lines = append(lines, fmt.Sprintf("launch cwd: %s (project .goclaw loads from here)", ld))
		}
	}
	lines = append(lines,
		fmt.Sprintf("session:   %s", rt.Sess.ID),
		fmt.Sprintf("provider:  %s", cfg.Provider),
	)
	lines = append(lines,
		fmt.Sprintf("model:     %s", cfg.Model()),
		fmt.Sprintf("task_model_router: %s", config.NormalizeTaskModelRouter(cfg.TaskModelRouter)),
		taskModelsLine,
		fmt.Sprintf("auto_profile_intent: %s", config.NormalizeAutoProfileIntent(cfg.AutoProfileIntent)),
		fmt.Sprintf("auto_direct_coding_profile: %s", config.NormalizeAutoDirectCodingProfile(cfg.AutoDirectCodingProfile)),
		fmt.Sprintf("action_repair_escalation: %v", cfg.ActionRepairEscalation),
		fmt.Sprintf("profile:   %s", rt.Profile.Name),
		fmt.Sprintf("file tools: %s", doctorFileToolsLine(rt.Profile)),
		fmt.Sprintf("tools:     %s", enabledDisabled(!rt.DisableTools)),
	)

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
	lines = append(lines, ideBridgeDoctorLines(rt)...)

	lines = append(lines, "")
	lines = append(lines, telegramDoctorLines(rt)...)

	lines = append(lines, "")
	lines = append(lines, "checks:")
	var ollamaOK bool
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "anthropic":
		lines = append(lines, "  ✗ provider \"anthropic\" is no longer supported — set \"provider\" to \"ollama\"")
	case "openai_compatible":
		lines = append(lines, "  ✗ provider \"openai_compatible\" is not supported — goclaw uses local Ollama only; set \"provider\" to \"ollama\" and use ollama_model")
	default:
		ollamaHost := effectiveOllamaHost(cfg.OllamaHost)
		probe := rt.OllamaProbe
		ollamaOK = probe.Reachable
		lines = append(lines, checkLine("ollama host reachable", ollamaOK))
		lines = append(lines, fmt.Sprintf("  - ollama host: %s", ollamaHost))
		lines = append(lines, fmt.Sprintf("  - ollama_num_ctx: %d", cfg.OllamaNumCtx))
		if ollamaOK {
			modelName := strings.TrimSpace(cfg.Model())
			if modelName != "" {
				lines = append(lines, checkLine("configured model in local ollama library", probe.ModelInLibrary))
				if !probe.ModelInLibrary {
					lines = append(lines, "    fix: ollama pull "+modelName)
				}
			}
		}
		if OllamaFunctionToolsDropped(rt) {
			lines = append(lines, "  ✗ ollama rejected tool calling — running text-only this session")
		} else {
			lines = append(lines, "  ✓ ollama tool calling active")
		}
	}

	lines = append(lines, checkLine("session store initialized", rt.Store != nil))

	lines = append(lines, mcpSummaryLines(rt)...)

	hintLines := hintLines(cfg, ollamaOK, rt.OllamaProbe.ModelInLibrary, rt.DisableTools, OllamaFunctionToolsDropped(rt))
	hintLines = append(hintLines, profileHintLines(rt.Profile)...)
	hintLines = append(hintLines, writeToolApprovalHintLines(rt)...)
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

	lines = append(lines, "")
	lines = append(lines, pluginSkillMemorySection(rt)...)

	return strings.Join(lines, "\n")
}

func pluginSkillMemorySection(rt *ChatRuntime) []string {
	out := []string{"plugins / skills / memory:"}
	out = append(out, fmt.Sprintf("  plugin_dirs configured: %d", len(rt.Cfg.PluginDirs)))
	out = append(out, fmt.Sprintf("  plugin manifests (allowed): %d", countPluginManifests(rt)))
	roots := skillRootsForDoctor(rt)
	out = append(out, fmt.Sprintf("  skill search roots: %d", len(roots)))
	// Only aggregate SKILL.md from the tool path root here — user home skill trees can be large;
	// snippet size matches what startup injects from the repo, not the full merged prompt.
	wsSkillRoots := workspaceSkillRootsOnly(rt)
	snippet, _ := skills.Collect(wsSkillRoots, skillsMaxRunes)
	out = append(out, fmt.Sprintf("  project tree skills snippet (approx): %d bytes", len(strings.TrimSpace(snippet))))
	memCount := 0
	memIndex := false
	if rt.MemStore != nil {
		if list, err := rt.MemStore.List(); err == nil {
			memCount = len(list)
		}
		memDir := filepath.Join(rt.Cfg.UserConfigDir, "memory")
		if st, err := os.Stat(filepath.Join(memDir, "MEMORY.md")); err == nil && !st.IsDir() {
			memIndex = true
		}
	}
	out = append(out, fmt.Sprintf("  memory entries (.md): %d", memCount))
	if memIndex {
		out = append(out, checkLine("memory index MEMORY.md present under ~/.goclaw/memory/", true))
	} else {
		out = append(out, "  ✗ memory index MEMORY.md missing (optional; create ~/.goclaw/memory/MEMORY.md or add memory entries)")
	}
	return out
}

func skillRootsForDoctor(rt *ChatRuntime) []string {
	var roots []string
	roots = append(roots, workspaceSkillRootsOnly(rt)...)
	ud := strings.TrimSpace(rt.Cfg.UserConfigDir)
	if ud != "" {
		roots = append(roots,
			filepath.Join(ud, "skills"),
			filepath.Join(ud, ".claude", "skills"),
		)
	}
	return roots
}

func workspaceSkillRootsOnly(rt *ChatRuntime) []string {
	tool := strings.TrimSpace(rt.Workdir)
	if tool == "" {
		return nil
	}
	cfg := rt.Cfg
	launch := strings.TrimSpace(rt.LaunchDir)
	add := func(wd string) []string {
		out := []string{
			filepath.Join(wd, cfg.ProjectConfigDir, "skills"),
			filepath.Join(wd, ".claude", "skills"),
		}
		return appendMonorepoClaudeSkillRoot(wd, out)
	}
	out := add(tool)
	if launch != "" && filepath.Clean(launch) != filepath.Clean(tool) {
		out = append(out, add(launch)...)
	}
	return out
}

func countPluginManifests(rt *ChatRuntime) int {
	wd := strings.TrimSpace(rt.Workdir)
	n := 0
	for _, d := range rt.Cfg.PluginDirs {
		abs := pluginResolveDir(wd, d)
		if abs == "" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(abs, "goclaw-plugin.json"))
		if err != nil {
			continue
		}
		var man plugin.Manifest
		if json.Unmarshal(raw, &man) != nil || strings.TrimSpace(man.Name) == "" {
			continue
		}
		if plugin.Allowed(man.Name, rt.Cfg.PluginAllow, rt.Cfg.PluginDeny) {
			n++
		}
	}
	return n
}

func pluginResolveDir(workdir, d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return ""
	}
	if filepath.IsAbs(d) {
		return filepath.Clean(d)
	}
	if strings.TrimSpace(workdir) == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(workdir, d))
}

func isEligibleMCPServer(srv config.MCPServerConfig) bool {
	if srv.Disabled || strings.TrimSpace(srv.ID) == "" {
		return false
	}
	return strings.TrimSpace(srv.Command) != "" || strings.TrimSpace(srv.URL) != ""
}

func countEligibleMCPServers(servers []config.MCPServerConfig) int {
	n := 0
	for _, srv := range servers {
		if isEligibleMCPServer(srv) {
			n++
		}
	}
	return n
}

func mcpSummaryLines(rt *ChatRuntime) []string {
	eligible := countEligibleMCPServers(rt.Cfg.MCPServers)
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
	eligible := countEligibleMCPServers(cfg.MCPServers)
	connected := len(rt.McpSessions)
	for _, srv := range cfg.MCPServers {
		if !isEligibleMCPServer(srv) {
			continue
		}
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

func doctorFileToolsLine(p agents.Profile) string {
	if p.AllowsWorkspaceFileWrites() {
		return "write_file / edit_file / patch exposed to model"
	}
	return "hidden (read-only or narrow tool_allowlist — model cannot call write tools)"
}

func profileHintLines(p agents.Profile) []string {
	if p.AllowsWorkspaceFileWrites() {
		return nil
	}
	out := []string{
		"  This profile does not expose write_file, edit_file, or patch to the model.",
	}
	if p.AllowsSpawnAgentDelegation() {
		out = append(out,
			"  Hub-style profile: delegate coding with spawn_agent (e.g. profile general-purpose or stack-coder).",
			"  Put file paths and acceptance criteria in the task field — workers do not see this chat.",
			"  For direct edits in this same session: /profile general-purpose — or set \"agent_profile\": \"general-purpose\" in .goclaw/settings.json",
		)
	} else {
		out = append(out,
			"  For file edits and coding in this session: /profile general-purpose",
			"  — or set \"agent_profile\": \"general-purpose\" in .goclaw/settings.json",
		)
	}
	return out
}

func writeToolApprovalHintLines(rt *ChatRuntime) []string {
	if rt == nil || rt.DisableTools || !rt.Profile.AllowsWorkspaceFileWrites() || rt.Policy == nil {
		return nil
	}
	wf := rt.Policy.Evaluate("write_file")
	ef := rt.Policy.Evaluate("edit_file")
	if wf == permissions.DecisionDeny || ef == permissions.DecisionDeny {
		return []string{
			"  tool_permissions sets write_file or edit_file to deny — the agent cannot persist those edits until you change settings.",
		}
	}
	if wf == permissions.DecisionAsk || ef == permissions.DecisionAsk {
		return []string{
			"  write_file / edit_file are on ask mode: workspace-relative writes (score 60) are auto-approved via yolo_threshold — only script and absolute-path writes require explicit approval.",
		}
	}
	return nil
}

func hintLines(cfg config.Config, ollamaOK, ollamaModelInLibrary, toolsDisabled, ollamaToolsDropped bool) []string {
	var hints []string
	if config.NormalizeTaskModelRouter(cfg.TaskModelRouter) == "llm" {
		hints = append(hints,
			"  task_model_router is \"llm\": adds ~2-3s per turn for LLM classification.",
			"  - Use \"rules\" for zero-latency heuristic routing, or \"off\" to disable.",
		)
	}
	if config.NormalizeTaskModelRouter(cfg.TaskModelRouter) != "off" && len(cfg.TaskModels) == 0 {
		hints = append(hints,
			"  task_model_router is not \"off\" but task_models is empty — per-turn model routing is inactive.",
			"  - Add a \"task_models\" map in settings.json or set task_model_router to \"off\".",
		)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.AgentProfile), "coordinator") && config.NormalizeAutoDirectCodingProfile(cfg.AutoDirectCodingProfile) == "off" {
		hints = append(hints,
			"  agent_profile is coordinator (hub): the parent session cannot run read_file/bash directly.",
			"  - For single-window coding: set \"auto_direct_coding_profile\" to \"general-purpose\" or \"builder\", optionally with \"auto_profile_intent\": \"rules\", or use /profile in-session.",
		)
	}
	if cfg.LLMCompaction {
		hints = append(hints,
			"  llm_compaction is enabled: each compaction fires an extra LLM call (slower on large sessions).",
			"  - Disable with \"llm_compaction\": false if compaction latency is noticeable.",
		)
	}
	if !ollamaOK {
		host := effectiveOllamaHost(cfg.OllamaHost)
		hints = append(hints,
			fmt.Sprintf("  Ollama did not respond at %s within the probe timeout.", host),
			"  - Start the daemon (example: ollama serve) or install Ollama if it is missing.",
			"  - If it listens elsewhere, set OLLAMA_HOST in the environment or \"ollama_host\" in settings.json.",
			"  - Confirm the model is pulled: ollama pull "+strings.TrimSpace(cfg.OllamaModel),
		)
	}
	if ollamaOK && !ollamaModelInLibrary {
		if m := strings.TrimSpace(cfg.Model()); m != "" {
			host := effectiveOllamaHost(cfg.OllamaHost)
			hints = append(hints,
				fmt.Sprintf("  Configured model %q is not listed by Ollama at %s (not pulled or name typo).", m, host),
				"  - Run: ollama pull "+m,
			)
		}
	}
	if toolsDisabled {
		hints = append(hints, "  Tools are disabled (--no-tools or GOCLAW_DISABLE_TOOLS=1); MCP servers were not started.")
	}
	if !toolsDisabled && ollamaOK {
		hints = append(hints,
			"  Local Ollama models may still refuse to summarize news or pages even when web_search/web_fetch succeed.",
			"  - Try a general instruct/chat model or a larger tag (e.g. pull a tool-capable variant).",
		)
	}
	if ollamaToolsDropped {
		model := strings.TrimSpace(cfg.OllamaModel)
		hints = append(hints,
			"  Ollama rejected tool calling for model "+model+" — goclaw is running text-only (no read_file/bash/etc).",
			"  Likely causes:",
			"  1. Model too old — re-pull it:  ollama pull "+model,
			"  2. Ollama version too old — update Ollama to v0.3+ (https://ollama.com)",
			fmt.Sprintf("  3. Context too small for tool schemas — set \"ollama_num_ctx\": %d (or higher) in ~/.goclaw/settings.json", config.DefaultOllamaNumCtx),
			fmt.Sprintf("  Current ollama_num_ctx: %d", cfg.OllamaNumCtx),
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
		return "url=" + truncate(u, doctorMCPDisplayMaxRunes)
	}
	cmd := strings.TrimSpace(srv.Command)
	var b strings.Builder
	b.WriteString("cmd=")
	b.WriteString(cmd)
	if len(srv.Args) > 0 {
		b.WriteString(" ")
		args := strings.Join(srv.Args, " ")
		b.WriteString(truncate(args, doctorMCPDisplayMaxRunes))
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

// ideBridgeDoctorLines summarizes IDE lockfile discovery and GOCLAW_IDE_NOTIFY_URL for doctor.
func ideBridgeDoctorLines(rt *ChatRuntime) []string {
	if rt == nil {
		return nil
	}
	cfg := rt.Cfg
	out := []string{"ide bridge:"}
	out = append(out, fmt.Sprintf("  ide_bridge_mcp: %v", cfg.IDEBridgeMCP))

	notifyRaw := strings.TrimSpace(os.Getenv("GOCLAW_IDE_NOTIFY_URL"))
	if notifyRaw == "" {
		out = append(out, "  GOCLAW_IDE_NOTIFY_URL: (unset)")
	} else {
		u, err := url.Parse(notifyRaw)
		loopbackOK := err == nil && (u.Scheme == "http" || u.Scheme == "https")
		if loopbackOK {
			host := strings.ToLower(u.Hostname())
			loopbackOK = host == "127.0.0.1" || host == "localhost" || host == "::1"
		}
		if loopbackOK {
			out = append(out, checkLine("GOCLAW_IDE_NOTIFY_URL loopback", true))
			out = append(out, "    "+truncate(notifyRaw, doctorMCPDisplayMaxRunes))
		} else {
			out = append(out, checkLine("GOCLAW_IDE_NOTIFY_URL loopback", false))
			out = append(out, "    (invalid URL, scheme, or non-loopback host — see startup logs)")
		}
	}

	ideDir := filepath.Join(cfg.UserConfigDir, "ide")
	if rt.DisableTools {
		out = append(out, "  ide lockfile MCP: skipped (tools disabled)")
		return out
	}
	if !cfg.IDEBridgeMCP {
		out = append(out, "  ide lockfile MCP: off (set \"ide_bridge_mcp\": true — see docs/goclaw/ide-editor-setup.md)")
		return out
	}
	endpoint, _, err := ide.DiscoverMCPEndpoint(ideDir)
	if err != nil {
		out = append(out, checkLine("ide lockfile MCP discovery", false))
		out = append(out, "    dir: "+ideDir)
		out = append(out, "    err: "+err.Error())
		out = append(out, "    fix: docs/goclaw/ide-editor-setup.md")
		return out
	}
	out = append(out, checkLine("ide lockfile MCP discovery", true))
	out = append(out, "    endpoint: "+truncate(endpoint, doctorMCPDisplayMaxRunes))
	return out
}

// telegramDoctorLines summarizes optional Telegram bridge settings (no token contents).
func telegramDoctorLines(rt *ChatRuntime) []string {
	if rt == nil {
		return nil
	}
	cfg := rt.Cfg
	out := []string{"telegram bridge (optional):"}
	tok, err := cfg.ResolveTelegramBotToken(rt.LaunchDir)
	hasToken := err == nil && strings.TrimSpace(tok) != ""
	if err != nil {
		out = append(out, checkLine("telegram bot token readable", false))
		out = append(out, "    err: "+err.Error())
	} else if hasToken {
		out = append(out, checkLine("telegram bot token configured", true))
		out = append(out, fmt.Sprintf("    token length: %d characters", len(tok)))
	} else {
		out = append(out, checkLine("telegram bot token configured", false))
		out = append(out, "    (unset — see docs/goclaw/telegram-bridge.md)")
	}
	n := len(cfg.TelegramAllowedUserIDs)
	out = append(out, checkLine("telegram_allowed_user_ids non-empty", n > 0))
	if n > 0 {
		out = append(out, fmt.Sprintf("    allowlist count: %d", n))
	} else if hasToken {
		out = append(out, "    warning: token set but allowlist empty — goclaw telegram bridge will exit")
		out = append(out, "    hint: goclaw telegram user add <numeric-id-or-@username>")
	}
	if sid := strings.TrimSpace(cfg.TelegramSessionID); sid != "" {
		out = append(out, fmt.Sprintf("  telegram_session_id: %s (when --session is omitted)", sid))
	}
	return out
}

// Minimal compile-time guard: DoctorReportFromRuntime expects fields set by PrepareChatRuntime.
var _ = config.Config{}
var _ = session.Store{}
