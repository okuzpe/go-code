package app

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
	"sync"
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
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/okuzpe/goclaw/internal/ui/chat"
	"github.com/spf13/cobra"
)

// terminalSink streams LLM text and tool lifecycle events to the terminal in readline mode.
// Text deltas are printed directly to stdout; tool events go to stderr so they don't
// mix with the streamed response.
type terminalSink struct {
	needsNL bool
}

func (s *terminalSink) OnTextDelta(text string) {
	fmt.Print(text)
	if len(text) > 0 {
		s.needsNL = text[len(text)-1] != '\n'
	}
}

func (s *terminalSink) OnToolUse(name, preview string) {
	if s.needsNL {
		fmt.Println()
		s.needsNL = false
	}
	if len(preview) > 120 {
		preview = preview[:120] + "…"
	}
	fmt.Fprintf(os.Stderr, "\n⚙ %s(%s)\n", name, preview)
}

func (s *terminalSink) OnToolResult(_ string, bytes int, isError bool) {
	if isError {
		fmt.Fprintf(os.Stderr, "  ✗ error (%d bytes)\n", bytes)
	} else {
		fmt.Fprintf(os.Stderr, "  ↳ %d bytes\n", bytes)
	}
}

func (s *terminalSink) OnDone(_ string) {
	if s.needsNL {
		fmt.Println()
		s.needsNL = false
	}
}

// RunListSessions prints saved session ids under the configured user config dir.
func RunListSessions() error {
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

// RunChat starts the interactive REPL (TUI or readline). version is the CLI build version string.
func RunChat(cmd *cobra.Command, version string, _ []string) error {
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
	readlineFlag, err := cmd.Flags().GetBool("readline")
	if err != nil {
		return err
	}
	tuiFlag, err := cmd.Flags().GetBool("tui")
	if err != nil {
		return err
	}
	mockFlag, err := cmd.Flags().GetBool("mock")
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
	if err := policy.ApplyConfigModes(cfg.PermissionModes); err != nil {
		return err
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
		// AfterTool: notify IDE only; terminal display is handled by terminalSink (readline)
		// or batchedProgramSink (TUI).
		orchOpts = append(orchOpts, orchestrator.WithAfterTool(func(toolName string, resultBytes int, isError bool) {
			ideNotifier.AfterTool(toolName, resultBytes, isError)
		}))
	}

	forceReadline := readlineFlag || strings.TrimSpace(os.Getenv("GOCLAW_USE_READLINE")) == "1"
	wantTUI := tuiFlag || strings.TrimSpace(os.Getenv("GOCLAW_USE_TUI")) == "1"
	useTUI := isTTY(os.Stdout) && !forceReadline && wantTUI

	printStartupBanner(version, cfg.Provider, cfg.Model(), profile.Name, sess.ID, workdir, disableTools, useTUI)

	if !useTUI && isTTY(os.Stdout) {
		fmt.Print(slashcmd.PopularSlashHint(workdir))
		fmt.Println()
	}

	slashEnv := slashcmd.SlashEnv{Workdir: workdir, Profs: profs}

	// SIGTERM → global shutdown context.
	// SIGINT is handled per-request below so Ctrl+C cancels an in-flight request
	// without exiting the REPL.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	if useTUI {
		// TUI path: BubbleTea owns signal handling for Ctrl+C.
		// Restore SIGINT so the TUI can handle it via tea.KeyCtrlC.
		signal.Reset(os.Interrupt)

		approval := chat.NewApprovalBroker()
		orchOpts = append(orchOpts, orchestrator.WithToolApprover(approval.ToolApprover()))
		orch := orchestrator.New(cfg, client, sess, reg, policy, hookReg, profile, orchOpts...)

		slash := func(input string) (handled bool, out string, quit bool, modelSubmit string, err error) {
			h, o, q, ms, e := slashcmd.HandleSlash(ctx, slashEnv, input, memStore, orch, &sess, store)
			if q && errors.Is(e, slashcmd.ErrReplQuit) {
				return h, o, q, ms, nil
			}
			return h, o, q, ms, e
		}
		var submit chat.Submitter
		if mockFlag {
			submit = func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error) {
				return streamMockAssistant(ctx, userText, sink, sess)
			}
		} else {
			submit = func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error) {
				return orch.RunStreaming(ctx, userText, sink)
			}
		}
		err := chat.RunApp(ctx, chat.Options{
			Title: fmt.Sprintf("goclaw  provider=%s  model=%s  profile=%s  session=%s",
				cfg.Provider, cfg.Model(), profile.Name, shortSessionID(sess.ID)),
		}, approval, submit, slash)
		slog.Info("saving session", "id", sess.ID, "messages", sess.Len())
		if saveErr := store.Save(sess); saveErr != nil {
			slog.Error("failed to save session", "err", saveErr)
		}
		return err
	}

	// ── Readline REPL ──────────────────────────────────────────────────────────

	// intCh receives SIGINT for per-request cancellation.
	intCh := make(chan os.Signal, 1)
	signal.Notify(intCh, os.Interrupt)
	defer signal.Stop(intCh)

	// reqMu guards reqCancelFn; non-nil while a request is in flight.
	var (
		reqMu      sync.Mutex
		reqCancelFn context.CancelFunc
	)
	setReqCancel := func(fn context.CancelFunc) {
		reqMu.Lock()
		reqCancelFn = fn
		reqMu.Unlock()
	}
	cancelReq := func() bool {
		reqMu.Lock()
		fn := reqCancelFn
		reqMu.Unlock()
		if fn != nil {
			fn()
			return true
		}
		return false
	}

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

	// Close readline on SIGTERM (global shutdown).
	go func() { <-ctx.Done(); _ = rl.Close() }()

	// Handle SIGINT: cancel in-flight request if any; otherwise close readline (exit).
	go func() {
		for {
			select {
			case <-intCh:
				if !cancelReq() {
					// No request in flight — treat as exit.
					_ = rl.Close()
					return
				}
				fmt.Fprintln(os.Stderr, "\n^C")
			case <-ctx.Done():
				return
			}
		}
	}()

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

		handled, slashOut, quit, modelSubmit, sErr := slashcmd.HandleSlash(ctx, slashEnv, input, memStore, orch, &sess, store)
		if quit && errors.Is(sErr, slashcmd.ErrReplQuit) {
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
			if slashOut != "" {
				fmt.Println(slashOut)
			}
			if strings.TrimSpace(modelSubmit) != "" {
				sink := &terminalSink{}
				reqCtx, reqCancel := context.WithCancel(ctx)
				setReqCancel(reqCancel)
				var rerr error
				if mockFlag {
					_, rerr = streamMockAssistant(reqCtx, modelSubmit, sink, sess)
				} else {
					_, rerr = orch.RunStreaming(reqCtx, modelSubmit, sink)
				}
				setReqCancel(nil)
				reqCancel()
				if rerr != nil && !(errors.Is(rerr, context.Canceled) && ctx.Err() == nil) {
					slog.Error("orchestrator error", "err", rerr)
					fmt.Fprintf(os.Stderr, "error: %v\n", rerr)
				}
			}
			continue
		}

		sink := &terminalSink{}
		reqCtx, reqCancel := context.WithCancel(ctx)
		setReqCancel(reqCancel)
		var runErr error
		if mockFlag {
			_, runErr = streamMockAssistant(reqCtx, input, sink, sess)
		} else {
			_, runErr = orch.RunStreaming(reqCtx, input, sink)
		}
		setReqCancel(nil)
		reqCancel()
		if runErr != nil && !(errors.Is(runErr, context.Canceled) && ctx.Err() == nil) {
			slog.Error("orchestrator error", "err", runErr)
			fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
		}
	}

	slog.Info("saving session", "id", sess.ID, "messages", sess.Len())
	if err := store.Save(sess); err != nil {
		slog.Error("failed to save session", "err", err)
	}

	return nil
}

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
