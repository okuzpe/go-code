package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/skills"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/spf13/cobra"
)

const (
	privateDirPerm  = 0o700
	skillsMaxRunes  = 24000
	mcpDialTimeout  = 30 * time.Second
	hookHTTPTimeout = 15 * time.Second
)

// ChatRuntime holds shared subsystems built once for an interactive chat session.
type ChatRuntime struct {
	Cfg              config.Config
	// Workdir is the default project directory (orchestrator path hints, default glob/grep tree,
	// project context snippet). File tools still accept absolute paths anywhere on disk.
	Workdir string
	// LaunchDir is the process working directory when goclaw started; project .goclaw settings,
	// hooks, project agents, and resolution of relative paths in file tools use this path
	// (may differ from Workdir when tool_workspace_root points at a subdirectory).
	LaunchDir        string
	Client           llm.Client
	Sess             *session.Session
	Store            *session.Store
	Reg              *tools.Registry
	MemStore         *memory.Store
	Policy           *permissions.Policy
	HookReg          *hooks.Registry
	Profile          agents.Profile
	Profs            map[string]agents.Profile
	UserAgentsDir    string
	ProjectAgentsDir string
	DisableTools     bool
	Mock             bool
	McpSessions      []mcp.Conn
	// McpConnectedIDs lists MCP server ids that started and registered tools successfully (same order as McpSessions).
	McpConnectedIDs []string
	OrchOpts        []orchestrator.Option
}

// OllamaFunctionToolsDropped reports whether the Ollama HTTP client fell back to text-only
// requests (model rejected wire tools). When true, agent tools are not invoked on the API.
func OllamaFunctionToolsDropped(rt *ChatRuntime) bool {
	if rt == nil || rt.DisableTools {
		return false
	}
	oc, ok := rt.Client.(*llm.OllamaClient)
	if !ok {
		return false
	}
	return oc.FunctionToolsDropped()
}

// appendIDEBridgeMCPServerIfMissing appends the IDE-discovered MCP server when it is not already configured.
func appendIDEBridgeMCPServerIfMissing(cfg *config.Config, endpointURL string, hdrs map[string]string) {
	if cfg == nil {
		return
	}
	u := strings.TrimSpace(endpointURL)
	if u == "" {
		return
	}
	for _, existing := range cfg.MCPServers {
		if strings.TrimSpace(existing.URL) == u || strings.TrimSpace(existing.ID) == "ide" {
			return
		}
	}
	cfg.MCPServers = append(cfg.MCPServers, config.MCPServerConfig{
		ID:      "ide",
		URL:     u,
		Headers: hdrs,
	})
}

// buildMCPServerDial builds a dial closure for one MCP server (HTTP preferred when URL is set).
func buildMCPServerDial(srv config.MCPServerConfig, workdir string, allowRemote bool) (func(context.Context) (mcp.Conn, error), error) {
	hasURL := strings.TrimSpace(srv.URL) != ""
	hasCmd := strings.TrimSpace(srv.Command) != ""
	switch {
	case hasURL:
		parsed, err := url.Parse(strings.TrimSpace(srv.URL))
		if err != nil {
			return nil, err
		}
		if err := mcp.ValidateHTTPURL(parsed, allowRemote); err != nil {
			return nil, err
		}
		urlStr := parsed.String()
		hdrs := cloneHeaderMap(srv.Headers)
		if tf := strings.TrimSpace(srv.BearerTokenFile); tf != "" {
			if tok, err := readBearerTokenFile(workdir, tf); err != nil {
				slog.Warn("mcp bearer_token_file unreadable", "id", srv.ID, "err", err)
			} else if tok != "" && !headerHasAuthorization(hdrs) {
				if hdrs == nil {
					hdrs = make(map[string]string)
				}
				hdrs["Authorization"] = "Bearer " + tok
			}
		}
		return func(_ context.Context) (mcp.Conn, error) {
			return mcp.NewHTTPSession(urlStr, hdrs)
		}, nil
	case hasCmd:
		cmd := srv.Command
		args := append([]string(nil), srv.Args...)
		env := srv.EnvSlice()
		cwd := srv.CWD
		return func(dctx context.Context) (mcp.Conn, error) {
			return mcp.StartStdioSession(dctx, cmd, args, env, cwd)
		}, nil
	default:
		return nil, fmt.Errorf("mcp: neither url nor command configured")
	}
}

// effectiveToolWorkspaceRoot returns the raw default-project-dir override: CLI --workspace, then
// GOCLAW_TOOL_WORKSPACE, then settings.json tool_workspace_root (may be empty).
func effectiveToolWorkspaceRoot(cfg config.Config, cmd *cobra.Command) string {
	if cmd != nil {
		if w, err := cmd.Flags().GetString("workspace"); err == nil && strings.TrimSpace(w) != "" {
			return strings.TrimSpace(w)
		}
	}
	if e := strings.TrimSpace(os.Getenv("GOCLAW_TOOL_WORKSPACE")); e != "" {
		return e
	}
	return strings.TrimSpace(cfg.ToolWorkspaceRoot)
}

// PrepareChatRuntime builds config, session, tools, hooks, and MCP for one interactive run.
func PrepareChatRuntime(cmd *cobra.Command) (*ChatRuntime, error) {
	sessionFlag, err := cmd.Flags().GetString("session")
	if err != nil {
		return nil, err
	}
	noToolsFlag, err := cmd.Flags().GetBool("no-tools")
	if err != nil {
		return nil, err
	}
	mockFlag, err := cmd.Flags().GetBool("mock")
	if err != nil {
		return nil, err
	}

	cfg, launchDir, err := loadMergedConfigForRun(cmd)
	if err != nil {
		return nil, err
	}
	if cmd != nil {
		if v, err := cmd.Flags().GetString("task-model-router"); err == nil && strings.TrimSpace(v) != "" {
			cfg.TaskModelRouter = config.NormalizeTaskModelRouter(v)
		}
	}

	toolRoot, err := config.ResolveToolWorkspace(launchDir, effectiveToolWorkspaceRoot(cfg, cmd))
	if err != nil {
		return nil, err
	}
	if toolRoot != launchDir {
		slog.Info("tool path root differs from launch cwd", "tool_path_root", toolRoot, "launch_cwd", launchDir)
	}

	sessDir := filepath.Join(cfg.UserConfigDir, "sessions")
	store, err := session.NewStore(sessDir)
	if err != nil {
		return nil, fmt.Errorf("session store: %w", err)
	}

	userAgentsDir := filepath.Join(cfg.UserConfigDir, "agents")
	projectAgentsDir := filepath.Join(launchDir, cfg.ProjectConfigDir, "agents")
	profs, err := agents.AllWithCustom(userAgentsDir, projectAgentsDir)
	if err != nil {
		slog.Warn("custom agent load error", "err", err)
		profs = agents.All()
	}
	profileKey := strings.TrimSpace(cfg.AgentProfile)
	profile, ok := profs[profileKey]
	if !ok {
		profile, ok = profs[strings.ToLower(profileKey)]
	}
	if !ok {
		return nil, fmt.Errorf("unknown agent profile %q; valid profiles: %s (use --profile or \"agent_profile\" in settings.json)",
			cfg.AgentProfile, agents.JoinSortedProfileKeys(profs))
	}

	slog.Info("starting goclaw", "provider", cfg.Provider, "model", cfg.Model(), "profile", profile.Name)

	var client llm.Client
	switch cfg.Provider {
	case "anthropic":
		apiKey := strings.TrimSpace(cfg.APIKey)
		if apiKey == "" {
			return nil, fmt.Errorf("provider=anthropic requires an API key: set environment variable ANTHROPIC_API_KEY, or add \"api_key\" to ~/.goclaw/settings.json or your project .goclaw/settings.json (run \"goclaw doctor\" to verify)")
		}
		client = llm.NewAnthropic(apiKey, cfg.BaseURL)
	case "openai_compatible":
		baseURL := strings.TrimSpace(cfg.OpenAICompatBaseURL)
		apiKey := strings.TrimSpace(cfg.OpenAICompatAPIKey)
		if baseURL == "" {
			return nil, fmt.Errorf("provider=openai_compatible requires a base URL: set OPENAI_BASE_URL or \"openai_base_url\" in settings (example: https://openrouter.ai/api/v1)")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("provider=openai_compatible requires an API key: set OPENAI_API_KEY or \"openai_api_key\" in settings")
		}
		if strings.TrimSpace(cfg.Model()) == "" {
			return nil, fmt.Errorf("provider=openai_compatible requires a model id: set OPENAI_MODEL or \"openai_model\" in settings")
		}
		client = llm.NewOpenAICompat(apiKey, baseURL)
	default:
		client = llm.NewOllama(cfg.OllamaHost)
	}

	var sess *session.Session
	switch id := strings.TrimSpace(sessionFlag); id {
	case "":
		sess = session.New()
		slog.Debug("new session", "id", sess.ID)
	default:
		loaded, err := store.Load(id)
		if err != nil {
			return nil, fmt.Errorf("load session %q: %w", id, err)
		}
		if loaded == nil {
			return nil, fmt.Errorf("session %q not found under %s", id, sessDir)
		}
		sess = loaded
		slog.Debug("resumed session", "id", sess.ID, "messages", sess.Len())
	}

	memDir := filepath.Join(cfg.UserConfigDir, "memory")
	if err := os.MkdirAll(memDir, privateDirPerm); err != nil {
		return nil, fmt.Errorf("memory dir: %w", err)
	}
	memStore := memory.New(memDir)

	// Per-agent memory: if the active profile declares a MemoryScope, replace the global
	// user store with a scoped store so tool auto-capture and prompt injection stay isolated.
	if profile.MemoryScope != "" {
		agentMemDir := memory.PerAgentMemoryDir(profile.MemoryScope, profile.Name, cfg.UserConfigDir, launchDir, cfg.ProjectConfigDir)
		if err := os.MkdirAll(agentMemDir, privateDirPerm); err != nil {
			slog.Warn("per-agent memory dir create failed; using global store", "dir", agentMemDir, "err", err)
		} else {
			memStore = memory.New(agentMemDir)
			slog.Debug("per-agent memory store attached", "profile", profile.Name, "scope", profile.MemoryScope, "dir", agentMemDir)
		}
	}

	// Per-project memory (D14): .goclaw/memory/ — only attached when the directory already
	// exists so we don't create it on every run. Users opt in by creating the directory.
	var projectMemStore *memory.Store
	projectMemDir := filepath.Join(launchDir, cfg.ProjectConfigDir, "memory")
	if info, err := os.Stat(projectMemDir); err == nil && info.IsDir() {
		projectMemStore = memory.New(projectMemDir)
		slog.Debug("project memory store attached", "dir", projectMemDir)
	}

	policy := permissions.NewPolicy()
	if err := policy.ApplyConfigModes(cfg.PermissionModes); err != nil {
		return nil, err
	}

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

	// Build project context and skills early so they can be shared with workers.
	projectCtx := buildProjectContext(toolRoot)
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
	skillSnippet, _ := skills.Collect(skillRoots, skillsMaxRunes)

	reg := tools.New()
	disableTools := noToolsFlag || strings.TrimSpace(os.Getenv("GOCLAW_DISABLE_TOOLS")) == "1"
	if cfg.IDEBridgeMCP && !disableTools {
		ideDir := filepath.Join(cfg.UserConfigDir, "ide")
		if u, hdrs, err := ide.DiscoverMCPEndpoint(ideDir); err == nil {
			appendIDEBridgeMCPServerIfMissing(&cfg, u, hdrs)
		}
	}
	var mcpSessions []mcp.Conn
	var mcpConnectedIDs []string
	var todoStore *todos.Store
	if !disableTools {
		todoStore = todos.NewStore()
		registerBuiltInTools(reg, toolRoot, launchDir, cfg, todoStore)

		// spawn_agent: worker registry excludes spawn_agent itself to prevent infinite nesting.
		workerReg := tools.New()
		registerBuiltInTools(workerReg, toolRoot, launchDir, cfg, todos.NewStore())
		reg.Register(coordinator.New(cfg, client, workerReg, policy, hookReg).
			WithProfiles(profs).
			WithWorkdir(toolRoot).
			WithLaunchDir(launchDir).
			WithProjectContext(projectCtx).
			WithMemoryStore(memStore).
			WithSkillsSnippet(skillSnippet))
		reg.Register(coordinator.NewStopTask())

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
	}

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
		if projectCtx != "" {
			orchOpts = append(orchOpts, orchestrator.WithProjectContext(projectCtx))
		}
	}
	if skillSnippet != "" {
		orchOpts = append(orchOpts, orchestrator.WithSkillsSnippet(skillSnippet))
	}
	if !disableTools {
		orchOpts = append(orchOpts, orchestrator.WithTodoStore(todoStore))
		sid := sess.ID
		orchOpts = append(orchOpts, orchestrator.WithAfterTool(func(toolName string, toolInput string, resultBytes int, isError bool) {
			ideNotifier.AfterTool(toolName, resultBytes, isError)
			memory.MaybeAutoCaptureFromTool(cfg, memStore, sid, toolName, toolInput, isError)
		}))
	}
	if cfg.Provider == "anthropic" && strings.ToLower(strings.TrimSpace(cfg.TokenCountMode)) != "heuristic" {
		if ac, ok := client.(*llm.AnthropicClient); ok {
			orchOpts = append(orchOpts, orchestrator.WithInputTokenCounter(ac))
		}
	}

	return &ChatRuntime{
		Cfg:              cfg,
		Workdir:          toolRoot,
		LaunchDir:        launchDir,
		Client:           client,
		Sess:             sess,
		Store:            store,
		Reg:              reg,
		MemStore:         memStore,
		Policy:           policy,
		HookReg:          hookReg,
		Profile:          profile,
		Profs:            profs,
		UserAgentsDir:    userAgentsDir,
		ProjectAgentsDir: projectAgentsDir,
		DisableTools:     disableTools,
		Mock:             mockFlag,
		McpSessions:      mcpSessions,
		McpConnectedIDs:  mcpConnectedIDs,
		OrchOpts:         orchOpts,
	}, nil
}

// registerBuiltInTools registers the 10 built-in tools into r (plus optional script when allow_script is true).
// It does NOT register spawn_agent — callers that need it do so separately.
// This is the single source of truth for built-in tool registration.
func registerBuiltInTools(r *tools.Registry, toolRoot string, launchDir string, cfg config.Config, todoStore *todos.Store) {
	pathScope := tools.PathScope{
		Root:         toolRoot,
		RelativeBase: launchDir,
	}
	r.Register(tools.NewReadFileScope(pathScope))
	r.Register(tools.NewGlobScope(pathScope))
	r.Register(tools.NewGrepScope(pathScope))
	r.Register(tools.NewBashWithTimeout(cfg.BashTimeoutSeconds()))
	if cfg.AllowScript {
		r.Register(tools.NewScriptWithTimeout(cfg.BashTimeoutSeconds()))
	}
	r.Register(tools.NewWriteFileScope(pathScope))
	r.Register(tools.NewEditFileScope(pathScope))
	r.Register(tools.NewPatchScope(pathScope))
	r.Register(tools.NewWebFetch())
	webBackend, webBackendOK := config.NormalizeWebSearchBackend(cfg.WebSearchBackend)
	if !webBackendOK && strings.TrimSpace(cfg.WebSearchBackend) != "" {
		slog.Warn("unknown web_search_backend, using ddg", "value", cfg.WebSearchBackend)
	}
	r.Register(tools.NewWebSearch(tools.WebSearchOptions{
		Backend:     webBackend,
		BraveAPIKey: cfg.BraveSearchAPIKey,
		SerpAPIKey:  cfg.SerpAPIKey,
		FallbackDDG: cfg.WebSearchFallbackDDG,
	}))
	todoTool, err := tools.NewTodoWrite(todoStore)
	if err != nil {
		panic(err)
	}
	r.Register(todoTool)
}

// readProjectFileLines reads a workspace-relative file and returns up to maxLines of trimmed content.
func readProjectFileLines(workdir, name string, maxLines int) ([]string, bool) {
	data, err := os.ReadFile(filepath.Join(workdir, name))
	if err != nil {
		return nil, false
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines, true
}

// buildProjectContext reads key project files from workdir and returns a compact
// summary for injection into the system prompt. Returns "" when nothing useful is found.
// Reads at most two small files; fast enough to call at startup.
func buildProjectContext(workdir string) string {
	var parts []string

	// Detect project type from manifest files.
	type candidate struct {
		file    string
		maxLine int
		label   string
	}
	manifests := []candidate{
		{"go.mod", 8, "go.mod"},
		{"package.json", 6, "package.json"},
		{"Cargo.toml", 8, "Cargo.toml"},
		{"pyproject.toml", 8, "pyproject.toml"},
	}
	for _, c := range manifests {
		if lines, ok := readProjectFileLines(workdir, c.file, c.maxLine); ok {
			parts = append(parts, c.label+":\n  "+strings.Join(lines, "\n  "))
			break // one manifest is enough
		}
	}

	// Append first 20 lines of README if present.
	for _, name := range []string{"README.md", "README.txt", "README"} {
		if lines, ok := readProjectFileLines(workdir, name, 20); ok {
			parts = append(parts, name+":\n  "+strings.Join(lines, "\n  "))
			break
		}
	}

	// Append CLAUDE.md project rules — this is the primary source of agent instructions.
	if lines, ok := readProjectFileLines(workdir, "CLAUDE.md", 60); ok {
		parts = append(parts, "CLAUDE.md (project rules):\n  "+strings.Join(lines, "\n  "))
	}

	return strings.Join(parts, "\n\n")
}
