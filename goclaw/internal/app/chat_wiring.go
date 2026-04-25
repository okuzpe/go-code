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
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/mcp"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
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
	Cfg config.Config
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
	// ExplicitAgentProfileFromCLI is true when the user set --profile on the CLI or GOCLAW_AGENT_PROFILE in the environment.
	// Auto-elevate from coordinator to a direct coding profile is skipped when this is set.
	ExplicitAgentProfileFromCLI bool
	McpSessions                 []mcp.Conn
	// McpConnectedIDs lists MCP server ids that started and registered tools successfully (same order as McpSessions).
	McpConnectedIDs []string
	OrchOpts        []orchestrator.Option
	// ScratchDir is the absolute session scratch path when allocated; empty when omitted (e.g. doctor).
	ScratchDir string
	// OllamaProbe is set when provider is ollama: result of GET /api/tags at PrepareChatRuntime (reachability + model in library).
	OllamaProbe OllamaStartupProbe

	closed bool
}

// Close releases runtime-owned resources in the same order as the chat/prompt entrypoints:
// session_end hook while the scratch dir still exists, then scratch cleanup, then MCP shutdown.
func (rt *ChatRuntime) Close() {
	if rt == nil || rt.closed {
		return
	}
	rt.closed = true
	if rt.HookReg != nil {
		_ = rt.HookReg.Fire(context.Background(), hooks.Event{Type: hooks.SessionEnd})
	}
	cleanupSessionScratch(rt.ScratchDir)
	rt.ScratchDir = ""
	for _, s := range rt.McpSessions {
		_ = s.Close()
	}
	rt.McpSessions = nil
}

// SaveSession persists the in-memory transcript when a session store is attached.
func (rt *ChatRuntime) SaveSession() error {
	if rt == nil || rt.Store == nil || rt.Sess == nil {
		return nil
	}
	return rt.Store.Save(rt.Sess)
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
	return prepareChatRuntime(cmd, true)
}

// prepareChatRuntime is the implementation of PrepareChatRuntime. When allocateScratch is false,
// no per-session scratch directory is created (used by doctor and other non-agent flows).
func prepareChatRuntime(cmd *cobra.Command, allocateScratch bool) (*ChatRuntime, error) {
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
	explicitAgentProfileFromCLI := runtimeExplicitAgentProfileFromCLI(cmd)
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

	userAgentsDir, projectAgentsDir, profs, profile, err := loadRuntimeProfiles(cfg, launchDir)
	if err != nil {
		return nil, err
	}
	client, err := newRuntimeClient(&cfg)
	if err != nil {
		return nil, err
	}
	slog.Info("starting goclaw", "provider", cfg.Provider, "model", cfg.Model(), "profile", profile.Name)

	sess, err := loadRuntimeSession(store, sessDir, sessionFlag)
	if err != nil {
		return nil, err
	}
	memStore, projectMemStore, err := createRuntimeMemoryStores(cfg, launchDir, profile)
	if err != nil {
		return nil, err
	}

	policy := permissions.NewPolicy()
	if err := policy.ApplyConfigModes(cfg.PermissionModes); err != nil {
		return nil, err
	}

	hookReg := newRuntimeHookRegistry(cfg, launchDir)
	projectInputs := buildRuntimeProjectInputs(toolRoot, launchDir, cfg)
	disableTools := noToolsFlag || strings.TrimSpace(os.Getenv("GOCLAW_DISABLE_TOOLS")) == "1"
	reg, mcpSessions, mcpConnectedIDs, todoStore, err := registerRuntimeToolsAndMCP(&cfg, toolRoot, launchDir, client, profs, policy, hookReg, memStore, projectInputs, disableTools)
	if err != nil {
		return nil, err
	}
	scratchDir, err := allocateRuntimeScratchDir(cfg, sess.ID, allocateScratch)
	if err != nil {
		return nil, err
	}
	orchOpts := buildRuntimeOrchestratorOptions(cfg, toolRoot, launchDir, scratchDir, disableTools, sess.ID, memStore, projectMemStore, todoStore, projectInputs)

	ollamaProbe := ProbeOllamaStartup(cfg)
	return &ChatRuntime{
		Cfg:                         cfg,
		Workdir:                     toolRoot,
		LaunchDir:                   launchDir,
		Client:                      client,
		Sess:                        sess,
		Store:                       store,
		Reg:                         reg,
		MemStore:                    memStore,
		Policy:                      policy,
		HookReg:                     hookReg,
		Profile:                     profile,
		Profs:                       profs,
		UserAgentsDir:               userAgentsDir,
		ProjectAgentsDir:            projectAgentsDir,
		DisableTools:                disableTools,
		Mock:                        mockFlag,
		ExplicitAgentProfileFromCLI: explicitAgentProfileFromCLI,
		McpSessions:                 mcpSessions,
		McpConnectedIDs:             mcpConnectedIDs,
		OrchOpts:                    orchOpts,
		ScratchDir:                  scratchDir,
		OllamaProbe:                 ollamaProbe,
	}, nil
}

func cleanupSessionScratch(dir string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("remove session scratch dir", "dir", dir, "err", err)
	}
}

// registerBuiltInTools registers the core built-in tools into r (plus optional script when allow_script is true).
// It does NOT register spawn_agent - callers that need it do so separately.
// This is the single source of truth for built-in tool registration.
func registerBuiltInTools(r *tools.Registry, toolRoot string, launchDir string, cfg config.Config, todoStore *todos.Store) error {
	pathScope := tools.PathScope{
		Root:         toolRoot,
		RelativeBase: launchDir,
	}
	r.Register(tools.NewReadFileScope(pathScope))
	r.Register(tools.NewGlobScope(pathScope))
	r.Register(tools.NewGrepScope(pathScope))
	r.Register(tools.NewBashWithTimeout(cfg.BashTimeoutSeconds()))
	r.Register(tools.NewRunCommandWithTimeout(cfg.BashTimeoutSeconds()))
	if cfg.AllowScript {
		r.Register(tools.NewScriptWithTimeout(cfg.BashTimeoutSeconds()))
	}
	r.Register(tools.NewWriteFileScope(pathScope))
	r.Register(tools.NewWriteFilesScope(pathScope))
	r.Register(tools.NewCreateProjectScope(pathScope))
	r.Register(tools.NewRunTestsScope(pathScope, cfg.BashTimeoutSeconds()))
	r.Register(tools.NewGitToolScope(pathScope))
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
	toolSearch, err := tools.NewToolSearch(r)
	if err != nil {
		return fmt.Errorf("tool_search: %w", err)
	}
	r.Register(toolSearch)
	todoTool, err := tools.NewTodoWrite(todoStore)
	if err != nil {
		return fmt.Errorf("todo_write: %w", err)
	}
	r.Register(todoTool)
	return nil
}
