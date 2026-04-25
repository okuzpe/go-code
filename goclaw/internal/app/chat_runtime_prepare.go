package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/ide"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/mcp"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/plugin"
	"github.com/okuzpe/goclaw/internal/projectcontext"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/skills"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/spf13/cobra"
)

type runtimeProjectInputs struct {
	Full         string
	Thin         string
	Lite         string
	SkillSnippet string
}

func runtimeExplicitAgentProfileFromCLI(cmd *cobra.Command) bool {
	explicit := strings.TrimSpace(os.Getenv("GOCLAW_AGENT_PROFILE")) != ""
	if cmd == nil {
		return explicit
	}
	if p, err := cmd.Flags().GetString("profile"); err == nil && strings.TrimSpace(p) != "" {
		explicit = true
	}
	if m, err := cmd.Flags().GetString("mode"); err == nil && strings.TrimSpace(m) != "" {
		explicit = true
	}
	return explicit
}

func loadRuntimeProfiles(cfg config.Config, launchDir string) (string, string, map[string]agents.Profile, agents.Profile, error) {
	userAgentsDir := filepath.Join(cfg.UserConfigDir, "agents")
	projectAgentsDir := filepath.Join(launchDir, cfg.ProjectConfigDir, "agents")
	profs, err := agents.AllWithCustom(userAgentsDir, projectAgentsDir)
	if err != nil {
		slog.Warn("custom agent load error", "err", err)
		profs = agents.All()
	}
	profileKey := agents.CanonicalProfileName(cfg.AgentProfile)
	profile, ok := profs[profileKey]
	if !ok {
		profile, ok = profs[agents.CanonicalProfileName(profileKey)]
	}
	if !ok {
		return "", "", nil, agents.Profile{}, fmt.Errorf("unknown agent profile %q; valid profiles: %s (use --mode build|plan, --profile, or \"agent_profile\" in settings.json)",
			cfg.AgentProfile, agents.JoinSortedProfileKeys(profs))
	}
	return userAgentsDir, projectAgentsDir, profs, runtimeAutonomousProfile(profile), nil
}

func runtimeAutonomousProfile(profile agents.Profile) agents.Profile {
	if isTTY(os.Stdout) {
		return profile
	}
	profile.SystemPrompt += "\n\n=== AUTONOMOUS MODE ===\n" +
		"Running without an interactive terminal. Be more conservative:\n" +
		"- Prefer read operations over writes when the intent is ambiguous\n" +
		"- Skip optional cleanup or cosmetic tasks unless explicitly requested\n" +
		"- After completing the task, report what was done; do not prompt for next steps\n" +
		"- If a required confirmation cannot be given, skip the risky action and explain why"
	return profile
}

func newRuntimeClient(cfg *config.Config) (llm.Client, error) {
	p := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch p {
	case "", "ollama":
		cfg.Provider = "ollama"
		oc := llm.NewOllamaWithHTTPTimeout(cfg.OllamaHost, cfg.OllamaHTTPClientTimeout())
		oc.RequireWireTools = cfg.OllamaRequireWireTools
		return oc, nil
	case "anthropic":
		return nil, fmt.Errorf("provider \"anthropic\" is no longer supported; set \"provider\" to \"ollama\" (default) and use a local model via ollama_model / OLLAMA_MODEL")
	case "openai_compatible":
		return nil, fmt.Errorf("provider \"openai_compatible\" is not supported; goclaw uses local Ollama only - set \"provider\" to \"ollama\", remove openai_* settings, and configure ollama_model / OLLAMA_MODEL")
	default:
		return nil, fmt.Errorf("unknown provider %q: only \"ollama\" is supported", cfg.Provider)
	}
}

func loadRuntimeSession(store *session.Store, sessDir, sessionFlag string) (*session.Session, error) {
	switch id := strings.TrimSpace(sessionFlag); id {
	case "":
		sess := session.New()
		slog.Debug("new session", "id", sess.ID)
		return sess, nil
	default:
		loaded, err := store.Load(id)
		if err != nil {
			return nil, fmt.Errorf("load session %q: %w", id, err)
		}
		if loaded == nil {
			return nil, fmt.Errorf("session %q not found under %s", id, sessDir)
		}
		slog.Debug("resumed session", "id", loaded.ID, "messages", loaded.Len())
		return loaded, nil
	}
}

func createRuntimeMemoryStores(cfg config.Config, launchDir string, profile agents.Profile) (*memory.Store, *memory.Store, error) {
	memDir := filepath.Join(cfg.UserConfigDir, "memory")
	if err := os.MkdirAll(memDir, privateDirPerm); err != nil {
		return nil, nil, fmt.Errorf("memory dir: %w", err)
	}
	memStore := memory.New(memDir)
	if profile.MemoryScope != "" {
		agentMemDir := memory.PerAgentMemoryDir(profile.MemoryScope, profile.Name, cfg.UserConfigDir, launchDir, cfg.ProjectConfigDir)
		if err := os.MkdirAll(agentMemDir, privateDirPerm); err != nil {
			slog.Warn("per-agent memory dir create failed; using global store", "dir", agentMemDir, "err", err)
		} else {
			memStore = memory.New(agentMemDir)
			slog.Debug("per-agent memory store attached", "profile", profile.Name, "scope", profile.MemoryScope, "dir", agentMemDir)
		}
	}

	var projectMemStore *memory.Store
	projectMemDir := filepath.Join(launchDir, cfg.ProjectConfigDir, "memory")
	if info, err := os.Stat(projectMemDir); err == nil && info.IsDir() {
		projectMemStore = memory.New(projectMemDir)
		slog.Debug("project memory store attached", "dir", projectMemDir)
	}
	return memStore, projectMemStore, nil
}

func newRuntimeHookRegistry(cfg config.Config, launchDir string) *hooks.Registry {
	hookReg := hooks.New()
	for _, h := range cfg.ExternalHooks {
		et, err := hooks.ParseEventType(h.Event)
		if err != nil {
			slog.Warn("skip external hook", "event", h.Event, "err", err)
			continue
		}
		if strings.TrimSpace(h.URL) != "" {
			hookReg.OnHTTP(et, strings.TrimSpace(h.URL), hookHTTPTimeout)
		} else if strings.TrimSpace(h.Command) != "" {
			hookReg.OnCommand(et, h.Command, h.Args...)
		}
	}
	for _, name := range plugin.RegisterHooksFromDirs(hookReg, cfg.PluginDirs, launchDir, cfg.PluginAllow, cfg.PluginDeny) {
		slog.Info("plugin hooks registered", "name", name)
	}
	if cfg.TrustedWorkspace {
		hookPath := filepath.Join(launchDir, ".goclaw", "hooks.json")
		if err := hooks.LoadHooksFile(hookReg, hookPath); err != nil {
			slog.Warn("load project hooks", "path", hookPath, "err", err)
		}
	}
	_ = hookReg.Fire(context.Background(), hooks.Event{Type: hooks.SessionStart})
	return hookReg
}

func buildRuntimeProjectInputs(toolRoot, launchDir string, cfg config.Config) runtimeProjectInputs {
	skillRoots := []string{
		filepath.Join(launchDir, cfg.ProjectConfigDir, "skills"),
		filepath.Join(launchDir, ".claude", "skills"),
		filepath.Join(cfg.UserConfigDir, "skills"),
		filepath.Join(cfg.UserConfigDir, ".claude", "skills"),
	}
	if toolRoot != launchDir {
		skillRoots = append([]string{
			filepath.Join(toolRoot, cfg.ProjectConfigDir, "skills"),
			filepath.Join(toolRoot, ".claude", "skills"),
		}, skillRoots...)
	}
	skillRoots = appendMonorepoClaudeSkillRoot(launchDir, skillRoots)
	skillSnippet, _ := skills.Collect(skillRoots, cfg.EffectiveSkillsMaxRunes())
	return runtimeProjectInputs{
		Full:         projectcontext.Build(toolRoot, cfg, true),
		Thin:         projectcontext.Build(toolRoot, cfg, false),
		Lite:         projectcontext.BuildLite(toolRoot),
		SkillSnippet: skillSnippet,
	}
}

func registerRuntimeToolsAndMCP(cfg *config.Config, toolRoot, launchDir string, client llm.Client, profs map[string]agents.Profile, policy *permissions.Policy, hookReg *hooks.Registry, memStore *memory.Store, projectInputs runtimeProjectInputs, disableTools bool) (*tools.Registry, []mcp.Conn, []string, *todos.Store, error) {
	reg := tools.New()
	if cfg.IDEBridgeMCP && !disableTools {
		ideDir := filepath.Join(cfg.UserConfigDir, "ide")
		if u, hdrs, err := ide.DiscoverMCPEndpoint(ideDir); err == nil {
			appendIDEBridgeMCPServerIfMissing(cfg, u, hdrs)
		} else {
			slog.Warn("ide bridge mcp: no MCP endpoint from lockfiles", "dir", ideDir, "err", err)
		}
	}
	if disableTools {
		return reg, nil, nil, nil, nil
	}

	todoStore := todos.NewStore()
	if err := registerBuiltInTools(reg, toolRoot, launchDir, *cfg, todoStore); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("register built-in tools: %w", err)
	}
	workerReg := tools.New()
	if err := registerBuiltInTools(workerReg, toolRoot, launchDir, *cfg, todos.NewStore()); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("register worker tools: %w", err)
	}
	reg.Register(coordinator.New(*cfg, client, workerReg, policy, hookReg).
		WithProfiles(profs).
		WithWorkdir(toolRoot).
		WithLaunchDir(launchDir).
		WithProjectContext(projectInputs.Full).
		WithProjectContextThin(projectInputs.Thin).
		WithProjectContextLite(projectInputs.Lite).
		WithMemoryStore(memStore).
		WithSkillsSnippet(projectInputs.SkillSnippet))
	reg.Register(coordinator.NewStopTask())

	var mcpSessions []mcp.Conn
	var mcpConnectedIDs []string
	for _, srv := range cfg.MCPServers {
		if srv.Disabled || srv.ID == "" {
			continue
		}
		hasURL := strings.TrimSpace(srv.URL) != ""
		hasCmd := strings.TrimSpace(srv.Command) != ""
		if !hasURL && !hasCmd {
			continue
		}
		dial, prepErr := buildMCPServerDial(srv, launchDir, cfg.MCPServersAllowRemote)
		if prepErr != nil {
			slog.Warn("mcp dial setup failed", "id", srv.ID, "err", prepErr)
			continue
		}
		sctx, cancel := context.WithTimeout(context.Background(), mcpDialTimeout)
		mcpSess, startErr := mcp.NewResilientConn(sctx, dial)
		if startErr != nil {
			slog.Warn("mcp connect failed", "id", srv.ID, "err", startErr)
			cancel()
			continue
		}
		if err := mcp.RegisterSessionTools(sctx, reg, mcpSess, srv.ID); err != nil {
			slog.Warn("mcp register tools failed", "id", srv.ID, "err", err)
			_ = mcpSess.Close()
			cancel()
			continue
		}
		cancel()
		mcpSessions = append(mcpSessions, mcpSess)
		mcpConnectedIDs = append(mcpConnectedIDs, srv.ID)
		slog.Info("mcp server connected", "id", srv.ID)
	}
	return reg, mcpSessions, mcpConnectedIDs, todoStore, nil
}

func allocateRuntimeScratchDir(cfg config.Config, sessID string, allocate bool) (string, error) {
	if !allocate {
		return "", nil
	}
	scratchDir := filepath.Join(cfg.UserConfigDir, "scratch", sessID)
	if err := os.MkdirAll(scratchDir, privateDirPerm); err != nil {
		return "", fmt.Errorf("session scratch dir: %w", err)
	}
	absScratch, err := filepath.Abs(scratchDir)
	if err != nil {
		return "", fmt.Errorf("session scratch dir abs: %w", err)
	}
	return absScratch, nil
}

func buildRuntimeOrchestratorOptions(cfg config.Config, toolRoot, launchDir, scratchDir string, disableTools bool, sessID string, memStore, projectMemStore *memory.Store, todoStore *todos.Store, projectInputs runtimeProjectInputs) []orchestrator.Option {
	ideNotifier := ide.FromEnv()
	orchOpts := []orchestrator.Option{orchestrator.WithMemoryStore(memStore)}
	if projectMemStore != nil {
		orchOpts = append(orchOpts, orchestrator.WithProjectMemoryStore(projectMemStore))
	}
	if toolRoot != "" {
		orchOpts = append(orchOpts, orchestrator.WithWorkdir(toolRoot))
		if strings.TrimSpace(launchDir) != "" {
			orchOpts = append(orchOpts, orchestrator.WithLaunchDir(launchDir))
		}
		if projectInputs.Full != "" {
			orchOpts = append(orchOpts, orchestrator.WithProjectContext(projectInputs.Full))
		}
		if projectInputs.Thin != "" {
			orchOpts = append(orchOpts, orchestrator.WithProjectContextThin(projectInputs.Thin))
		}
		if projectInputs.Lite != "" {
			orchOpts = append(orchOpts, orchestrator.WithProjectContextLite(projectInputs.Lite))
		}
	}
	if projectInputs.SkillSnippet != "" {
		orchOpts = append(orchOpts, orchestrator.WithSkillsSnippet(projectInputs.SkillSnippet))
	}
	if scratchDir != "" {
		orchOpts = append(orchOpts, orchestrator.WithScratchDir(scratchDir))
	}
	if !disableTools {
		orchOpts = append(orchOpts, orchestrator.WithTodoStore(todoStore))
		orchOpts = append(orchOpts, orchestrator.WithAfterTool(func(toolName string, toolInput string, resultBytes int, isError bool) {
			ideNotifier.AfterTool(toolName, resultBytes, isError)
			memory.MaybeAutoCaptureFromTool(cfg, memStore, sessID, toolName, toolInput, isError)
		}))
	}
	return orchOpts
}
