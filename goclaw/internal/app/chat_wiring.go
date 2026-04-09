package app

import (
	"context"
	"fmt"
	"log/slog"
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
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/spf13/cobra"
)

// ChatRuntime holds shared subsystems built once for an interactive chat session.
type ChatRuntime struct {
	Cfg          config.Config
	Workdir      string
	Client       llm.Client
	Sess         *session.Session
	Store        *session.Store
	Reg          *tools.Registry
	MemStore     *memory.Store
	Policy       *permissions.Policy
	HookReg      *hooks.Registry
	Profile      agents.Profile
	Profs            map[string]agents.Profile
	UserAgentsDir    string
	ProjectAgentsDir string
	DisableTools     bool
	Mock         bool
	McpSessions  []*mcp.Session
	// McpConnectedIDs lists MCP server ids that started and registered tools successfully (same order as McpSessions).
	McpConnectedIDs []string
	OrchOpts        []orchestrator.Option
}

// PrepareChatRuntime builds config, session, tools, hooks, and MCP for one interactive run.
func PrepareChatRuntime(cmd *cobra.Command) (*ChatRuntime, error) {
	profileFlag, err := cmd.Flags().GetString("profile")
	if err != nil {
		return nil, err
	}
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

	workdir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	cfg := config.Default()
	cfg, err = config.Load(cfg, workdir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if p := strings.TrimSpace(profileFlag); p != "" {
		cfg.AgentProfile = p
	}

	sessDir := filepath.Join(cfg.UserConfigDir, "sessions")
	store, err := session.NewStore(sessDir)
	if err != nil {
		return nil, fmt.Errorf("session store: %w", err)
	}

	userAgentsDir := filepath.Join(cfg.UserConfigDir, "agents")
	projectAgentsDir := filepath.Join(workdir, cfg.ProjectConfigDir, "agents")
	profs, err := agents.AllWithCustom(userAgentsDir, projectAgentsDir)
	if err != nil {
		slog.Warn("custom agent load error", "err", err)
		profs = agents.All()
	}
	profile, ok := profs[cfg.AgentProfile]
	if !ok {
		return nil, fmt.Errorf("unknown agent profile %q; valid profiles: %s (use --profile or \"agent_profile\" in settings.json)",
			cfg.AgentProfile, agents.ProfileListHint())
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
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		return nil, fmt.Errorf("memory dir: %w", err)
	}
	memStore := memory.New(memDir)

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
			hookReg.OnHTTP(et, strings.TrimSpace(h.URL), 15*time.Second)
		} else if strings.TrimSpace(h.Command) != "" {
			hookReg.OnCommand(et, h.Command, h.Args...)
		}
	}
	if cfg.TrustedWorkspace {
		hookPath := filepath.Join(workdir, ".goclaw", "hooks.json")
		if err := hooks.LoadHooksFile(hookReg, hookPath); err != nil {
			slog.Warn("load project hooks", "path", hookPath, "err", err)
		}
	}
	_ = hookReg.Fire(context.Background(), hooks.Event{Type: hooks.SessionStart})

	reg := tools.New()
	disableTools := noToolsFlag || strings.TrimSpace(os.Getenv("GOCLAW_DISABLE_TOOLS")) == "1"
	var mcpSessions []*mcp.Session
	var mcpConnectedIDs []string
	var todoStore *todos.Store
	if !disableTools {
		todoStore = todos.NewStore()
		registerBuiltInTools(reg, workdir, cfg, todoStore)

		// spawn_agent: worker registry excludes spawn_agent itself to prevent infinite nesting.
		workerReg := tools.New()
		registerBuiltInTools(workerReg, workdir, cfg, todos.NewStore())
		reg.Register(coordinator.New(cfg, client, workerReg, policy, hookReg).WithProfiles(profs))

		for _, srv := range cfg.MCPServers {
			if srv.Disabled || srv.ID == "" || srv.Command == "" {
				continue
			}
			sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			mcpSess, err := mcp.StartStdioSession(sctx, srv.Command, srv.Args, srv.EnvSlice(), srv.CWD)
			if err != nil {
				slog.Warn("mcp server start failed", "id", srv.ID, "err", err)
				cancel()
				continue
			}
			if err := mcpSess.Initialize(sctx); err != nil {
				slog.Warn("mcp initialize failed", "id", srv.ID, "err", err)
				_ = mcpSess.Close()
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
	if !disableTools {
		orchOpts = append(orchOpts, orchestrator.WithTodoStore(todoStore))
		orchOpts = append(orchOpts, orchestrator.WithAfterTool(func(toolName string, resultBytes int, isError bool) {
			ideNotifier.AfterTool(toolName, resultBytes, isError)
		}))
	}

	return &ChatRuntime{
		Cfg:          cfg,
		Workdir:      workdir,
		Client:       client,
		Sess:         sess,
		Store:        store,
		Reg:          reg,
		MemStore:     memStore,
		Policy:       policy,
		HookReg:      hookReg,
		Profile:          profile,
		Profs:            profs,
		UserAgentsDir:    userAgentsDir,
		ProjectAgentsDir: projectAgentsDir,
		DisableTools:     disableTools,
		Mock:         mockFlag,
		McpSessions:     mcpSessions,
		McpConnectedIDs: mcpConnectedIDs,
		OrchOpts:        orchOpts,
	}, nil
}

// registerBuiltInTools registers the 9 built-in tools into r.
// It does NOT register spawn_agent — callers that need it do so separately.
// This is the single source of truth for built-in tool registration.
func registerBuiltInTools(r *tools.Registry, workdir string, cfg config.Config, todoStore *todos.Store) {
	r.Register(tools.NewReadFile(workdir))
	r.Register(tools.NewGlob(workdir))
	r.Register(tools.NewGrep(workdir))
	r.Register(tools.NewBashWithTimeout(cfg.BashTimeoutSeconds()))
	if cfg.AllowScript {
		r.Register(tools.NewScriptWithTimeout(cfg.BashTimeoutSeconds()))
	}
	r.Register(tools.NewWriteFile(workdir))
	r.Register(tools.NewEditFile(workdir))
	r.Register(tools.NewWebFetch())
	r.Register(tools.NewWebSearch())
	r.Register(tools.NewTodoWrite(todoStore))
}
