package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chzyer/readline"
	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
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
	"github.com/okuzpe/goclaw/internal/ui/chat"
	"github.com/spf13/cobra"
)

// runListSessions prints saved session ids under the configured user config dir.
func runListSessions() error {
	workdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	cfg := config.Default()
	cfg, err = config.Load(cfg, workdir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	sessDir := filepath.Join(cfg.UserConfigDir, "sessions")
	store, err := session.NewStore(sessDir)
	if err != nil {
		return fmt.Errorf("session store: %w", err)
	}
	ids, err := store.ListIDs()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	if len(ids) == 0 {
		fmt.Println("(no saved sessions)")
		return nil
	}
	for _, id := range ids {
		fmt.Println(id)
	}
	return nil
}

func runChat(cmd *cobra.Command, _ []string) error {
	profileFlag, err := cmd.Flags().GetString("profile")
	if err != nil {
		return err
	}
	sessionFlag, err := cmd.Flags().GetString("session")
	if err != nil {
		return err
	}
	noToolsFlag, err := cmd.Flags().GetBool("no-tools")
	if err != nil {
		return err
	}

	workdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := config.Default()
	cfg, err = config.Load(cfg, workdir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if p := strings.TrimSpace(profileFlag); p != "" {
		cfg.AgentProfile = p
	}

	sessDir := filepath.Join(cfg.UserConfigDir, "sessions")
	store, err := session.NewStore(sessDir)
	if err != nil {
		return fmt.Errorf("session store: %w", err)
	}

	profs := agents.All()
	profile, ok := profs[cfg.AgentProfile]
	if !ok {
		return fmt.Errorf("unknown agent profile %q (--profile general-purpose|explore|plan|verification|guide|statusline)", cfg.AgentProfile)
	}

	slog.Info("starting goclaw", "provider", cfg.Provider, "model", cfg.Model(), "profile", profile.Name)

	var client llm.Client
	switch cfg.Provider {
	case "anthropic":
		if cfg.APIKey == "" {
			return fmt.Errorf("provider=anthropic requires ANTHROPIC_API_KEY")
		}
		client = llm.NewAnthropic(cfg.APIKey, cfg.BaseURL)
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
			return fmt.Errorf("load session %q: %w", id, err)
		}
		if loaded == nil {
			return fmt.Errorf("session %q not found under %s", id, sessDir)
		}
		sess = loaded
		slog.Debug("resumed session", "id", sess.ID, "messages", sess.Len())
	}

	memDir := filepath.Join(cfg.UserConfigDir, "memory")
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		return fmt.Errorf("memory dir: %w", err)
	}
	memStore := memory.New(memDir)

	reg := tools.New()
	disableTools := noToolsFlag || strings.TrimSpace(os.Getenv("GOCLAW_DISABLE_TOOLS")) == "1"
	var mcpSessions []*mcp.Session
	var todoStore *todos.Store
	defer func() {
		for _, s := range mcpSessions {
			_ = s.Close()
		}
	}()
	if !disableTools {
		todoStore = todos.NewStore()
		reg.Register(tools.NewReadFile(workdir))
		reg.Register(tools.NewGlob(workdir))
		reg.Register(tools.NewGrep(workdir))
		reg.Register(tools.NewBashWithTimeout(cfg.BashTimeoutSeconds()))
		reg.Register(tools.NewWriteFile(workdir))
		reg.Register(tools.NewEditFile(workdir))
		reg.Register(tools.NewWebFetch())
		reg.Register(tools.NewWebSearch())
		reg.Register(tools.NewTodoWrite(todoStore))

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
			slog.Info("mcp server connected", "id", srv.ID)
		}
	}

	policy := permissions.NewPolicy()
	for tool, modeStr := range cfg.PermissionModes {
		m, err := permissions.ParseMode(modeStr)
		if err != nil {
			return fmt.Errorf("settings tool_permissions %q: %w", tool, err)
		}
		policy.Set(tool, m)
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
	defer func() {
		_ = hookReg.Fire(context.Background(), hooks.Event{Type: hooks.SessionEnd})
	}()

	ideNotifier := ide.FromEnv()
	orchOpts := []orchestrator.Option{orchestrator.WithMemoryStore(memStore)}
	if !disableTools {
		orchOpts = append(orchOpts, orchestrator.WithTodoStore(todoStore))
		orchOpts = append(orchOpts, orchestrator.WithAfterTool(func(toolName string, resultBytes int, isError bool) {
			if isError {
				fmt.Fprintf(os.Stderr, "[tool result] %s — error payload (%d bytes)\n", toolName, resultBytes)
			} else {
				fmt.Fprintf(os.Stderr, "[tool result] %s — %d bytes returned to the model\n", toolName, resultBytes)
			}
			ideNotifier.AfterTool(toolName, resultBytes, isError)
		}))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	printStartupBanner(cfg.Provider, cfg.Model(), profile.Name, sess.ID, disableTools)

	slashEnv := SlashEnv{Workdir: workdir, Profs: profs}

	// TUI mode for interactive terminals.
	if isTTY(os.Stdout) {
		approval := chat.NewApprovalBroker()
		orchOpts = append(orchOpts, orchestrator.WithToolApprover(approval.ToolApprover()))
		orch := orchestrator.New(cfg, client, sess, reg, policy, hookReg, profile, orchOpts...)

		slash := func(input string) (handled bool, out string, quit bool, err error) {
			handled, out, err = handleSlash(ctx, slashEnv, input, memStore, orch, &sess, store)
			if errors.Is(err, ErrReplQuit) {
				return true, out, true, nil
			}
			return handled, out, false, err
		}
		submit := func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error) {
			return orch.RunStreaming(ctx, userText, sink)
		}
		return chat.RunApp(ctx, chat.Options{
			Title: fmt.Sprintf("goclaw  provider=%s  model=%s  profile=%s  session=%s",
				cfg.Provider, cfg.Model(), profile.Name, sess.ID),
		}, approval, submit, slash)
	}

	// Non-TTY fallback: keep the classic readline flow.
	historyFile := filepath.Join(cfg.UserConfigDir, "history")
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          replPrompt(sess),
		HistoryFile:     historyFile,
		HistoryLimit:    500,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("readline: %w", err)
	}
	defer rl.Close()

	getPrompt := func() string { return replPrompt(sess) }
	orchOpts = append(orchOpts, orchestrator.WithToolApprover(terminalToolApprover(rl, getPrompt)))
	orch := orchestrator.New(cfg, client, sess, reg, policy, hookReg, profile, orchOpts...)

	go func() { <-ctx.Done(); _ = rl.Close() }()

	for {
		rl.SetPrompt(replPrompt(sess))
		input, err := rl.Readline()
		if err != nil {
			if errors.Is(err, readline.ErrInterrupt) || errors.Is(err, io.EOF) {
				fmt.Println()
				break
			}
			slog.Error("readline error", "err", err)
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		handled, slashOut, sErr := handleSlash(ctx, slashEnv, input, memStore, orch, &sess, store)
		if errors.Is(sErr, ErrReplQuit) {
			if slashOut != "" {
				fmt.Println(slashOut)
			}
			break
		}
		if sErr != nil {
			slog.Error("command error", "err", sErr)
			fmt.Fprintf(os.Stderr, "error: %v\n", sErr)
			continue
		}
		if handled {
			fmt.Println(slashOut)
			continue
		}

		response, err := orch.Run(ctx, input)
		if err != nil {
			slog.Error("orchestrator error", "err", err)
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		fmt.Println(response)
	}

	slog.Info("saving session", "id", sess.ID, "messages", sess.Len())
	if err := store.Save(sess); err != nil {
		slog.Error("failed to save session", "err", err)
	}

	return nil
}

// replPrompt shows a short session id prefix so multi-session REPL use is obvious.
func replPrompt(s *session.Session) string {
	if s == nil {
		return "> "
	}
	id := s.ID
	if len(id) > 8 {
		id = id[:8]
	}
	return id + "> "
}

// terminalToolApprover prompts the user via readline before each tool execution.
func terminalToolApprover(rl *readline.Instance, getPrompt func() string) orchestrator.ToolApprover {
	return func(ctx context.Context, toolName, toolInput string) (bool, error) {
		_ = ctx
		preview := toolInput
		if len(preview) > 400 {
			preview = preview[:400] + "…"
		}
		printToolApprovalPrompt(os.Stderr, toolName, preview)
		rl.SetPrompt("Allow execution? [y/N]: ")
		line, err := rl.Readline()
		rl.SetPrompt(getPrompt())
		if err != nil {
			return false, err
		}
		s := strings.ToLower(strings.TrimSpace(line))
		return s == "y" || s == "yes", nil
	}
}
