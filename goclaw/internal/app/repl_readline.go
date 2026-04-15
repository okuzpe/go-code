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
	"slices"
	"strings"
	"sync"

	"github.com/chzyer/readline"
	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/text"
)

const (
	replPromptIDMaxRunes             = 8
	replToolApprovalFallbackMaxRunes = 400
	replHistoryLimit                 = 500
)

func replHistoryFile(userConfigDir string) string {
	return filepath.Join(userConfigDir, "history")
}

func replPrompt(s *session.Session, focus *coordinator.FocusRouter) string {
	if s == nil {
		return "> "
	}
	id := text.TruncateRunesHard(s.ID, replPromptIDMaxRunes)
	if focus != nil {
		if w := strings.TrimSpace(focus.Current()); w != "" {
			wid := text.TruncateRunesHard(w, replPromptIDMaxRunes)
			return fmt.Sprintf("%s@w%s> ", id, wid)
		}
	}
	return id + "> "
}

func terminalToolApprover(rl *readline.Instance, getPrompt func() string, uiAppearance string) orchestrator.ToolApprover {
	return func(ctx context.Context, toolName, toolInput string) (bool, error) {
		_ = ctx
		preview := orchestrator.FormatToolUsePreview(toolName, toolInput)
		if preview == "" {
			preview = truncate(toolInput, replToolApprovalFallbackMaxRunes)
		}
		printToolApprovalPrompt(os.Stderr, toolName, preview, uiAppearance)
		rl.SetPrompt("Allow execution? [y/N]: ")
		line, err := rl.Readline()
		rl.SetPrompt(getPrompt())
		if err != nil {
			slog.Error("tool approval input error", "tool", toolName, "err", err)
			return false, err
		}
		s := strings.ToLower(strings.TrimSpace(line))
		return s == "y" || s == "yes", nil
	}
}

// runOrchestratorTurn runs one streaming turn with per-request cancellation.
// baseCtx is the REPL lifetime context (e.g. SIGTERM); a user Ctrl+C only cancels reqCtx.
func runOrchestratorTurn(
	baseCtx context.Context,
	mock bool,
	provider string,
	model string,
	orch *orchestrator.Orchestrator,
	sess *session.Session,
	userText string,
	sink orchestrator.StreamSink,
	setReqCancel func(context.CancelFunc),
	workdir string,
) {
	reqCtx, reqCancel := context.WithCancel(baseCtx)
	setReqCancel(reqCancel)
	startLen := 0
	if ls, ok := sink.(*loggingTerminalSink); ok {
		startLen = ls.ToolLogLen()
	}
	var final string
	var runErr error
	if mock {
		final, runErr = StreamMockAssistant(reqCtx, userText, sink, sess)
	} else {
		final, runErr = orch.RunStreaming(reqCtx, userText, sink)
	}
	runErr = AugmentOrchestratorErr(provider, model, runErr)
	setReqCancel(nil)
	reqCancel()
	if runErr != nil && !(errors.Is(runErr, context.Canceled) && baseCtx.Err() == nil) {
		slog.Error("orchestrator error", "err", runErr)
		fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
	}
	if runErr == nil {
		if ls, ok := sink.(*loggingTerminalSink); ok {
			ls.PrintReadlineTurnFooters(startLen, final, workdir)
		}
	}
}

type readlineREPL struct {
	baseCtx  context.Context
	rt       *ChatRuntime
	slashEnv slashcmd.SlashEnv
	orch     *orchestrator.Orchestrator
	focus    *coordinator.FocusRouter
	sink     *loggingTerminalSink
	// replSession is the active *session.Session for slash + orchestrator turns; slash handlers
	// may replace it via SlashContext.Sess. Tab completion reads the same pointer.
	replSession *session.Session
}

func (r *readlineREPL) run(rl *readline.Instance, intCh <-chan os.Signal) {
	var (
		reqMu       sync.Mutex
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

	// replSession is the working session pointer. Slash commands (e.g. /new) may replace it
	// via SlashContext.Sess; we sync it back to r.rt.Sess after each HandleSlash call (line below).
	// The signal-handler goroutine only calls reqCancelFn, which aborts the in-flight orchestrator
	// turn — it never touches replSession. Once RunStreaming returns (before we touch replSession
	// again), the goroutine is guaranteed not to be writing to it, so this pattern is safe without a mutex.
	r.replSession = r.rt.Sess

	go func() {
		for {
			select {
			case <-intCh:
				if !cancelReq() {
					_ = rl.Close()
					return
				}
				fmt.Fprintln(os.Stderr, "\n^C")
			case <-r.baseCtx.Done():
				return
			}
		}
	}()

	for {
		rl.SetPrompt(replPrompt(r.replSession, r.focus))
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

		sc := slashcmd.SlashContext{SlashEnv: r.slashEnv, Mem: r.rt.MemStore, Orch: r.orch, Sess: &r.replSession, Store: r.rt.Store}
		handled, slashOut, quit, modelSubmit, sErr := slashcmd.HandleSlash(r.baseCtx, sc, input, nil)
		r.rt.Sess = r.replSession
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
				runOrchestratorTurn(r.baseCtx, r.rt.Mock, r.rt.Cfg.Provider, r.rt.Cfg.Model(), r.orch, r.replSession, modelSubmit, r.sink, setReqCancel, r.rt.Workdir)
			}
			continue
		}

		if wid := strings.TrimSpace(r.focus.Current()); wid != "" {
			startLen := r.sink.ToolLogLen()
			werr := runWorkerTurn(r.baseCtx, r.rt.Cfg.Provider, r.rt.Cfg.Model(), wid, input, r.sink, setReqCancel)
			if werr == nil {
				snap, _ := coordinator.SnapshotInteractiveWorker(wid)
				r.sink.PrintReadlineTurnFooters(startLen, snap, r.rt.Workdir)
				continue
			}
			if errors.Is(werr, coordinator.ErrInteractiveWorkerNotFound) {
				r.focus.Detach()
				fmt.Fprintln(os.Stderr, "note: focused worker ended — routing this message to the parent session")
				// fall through to parent orchestrator with the same input
			} else {
				continue
			}
		}

		if handled, err := RunLocalPrefixToolIfAny(r.baseCtx, r.rt.Mock, r.orch, r.replSession, input, r.sink, r.rt.Workdir); handled {
			if err != nil {
				slog.Error("prefix input error", "err", err)
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			continue
		}

		input = ExpandInlineAtRefs(r.baseCtx, r.orch, input)
		runOrchestratorTurn(r.baseCtx, r.rt.Mock, r.rt.Cfg.Provider, r.rt.Cfg.Model(), r.orch, r.replSession, input, r.sink, setReqCancel, r.rt.Workdir)
	}
}

func runWorkerTurn(
	baseCtx context.Context,
	provider, model, taskID, userText string,
	sink orchestrator.StreamSink,
	setReqCancel func(context.CancelFunc),
) error {
	reqCtx, reqCancel := context.WithCancel(baseCtx)
	setReqCancel(reqCancel)
	err := coordinator.DeliverWorkerMessage(reqCtx, taskID, userText, sink)
	err = AugmentOrchestratorErr(provider, model, err)
	setReqCancel(nil)
	reqCancel()
	if err != nil && !(errors.Is(err, context.Canceled) && baseCtx.Err() == nil) {
		if !errors.Is(err, coordinator.ErrInteractiveWorkerNotFound) {
			slog.Error("worker turn error", "err", err)
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	return err
}

func runReadlineREPL(ctx context.Context, rt *ChatRuntime, orchOpts []orchestrator.Option) error {
	intCh := make(chan os.Signal, 1)
	signal.Notify(intCh, os.Interrupt)
	defer signal.Stop(intCh)

	if err := os.MkdirAll(rt.Cfg.UserConfigDir, 0o700); err != nil {
		return fmt.Errorf("readline: config dir: %w", err)
	}
	historyFile := replHistoryFile(rt.Cfg.UserConfigDir)
	focus := coordinator.NewFocusRouter()

	var getSlashCtx func() slashcmd.SlashContext
	slashCtxWrapper := func() slashcmd.SlashContext {
		if getSlashCtx == nil {
			return slashcmd.SlashContext{}
		}
		return getSlashCtx()
	}
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          replPrompt(rt.Sess, focus),
		HistoryFile:     historyFile,
		HistoryLimit:    replHistoryLimit,
		AutoComplete:    slashcmd.NewReadlineSlashAtCompleterWithSlashContext(rt.Workdir, slashCtxWrapper),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("readline: %w", err)
	}
	defer rl.Close()

	getPrompt := func() string { return replPrompt(rt.Sess, focus) }
	orchOpts = append(append([]orchestrator.Option(nil), orchOpts...), orchestrator.WithToolApprover(terminalToolApprover(rl, getPrompt, rt.Cfg.UIAppearance)))
	orch := orchestrator.New(rt.Cfg, rt.Client, rt.Sess, rt.Reg, rt.Policy, rt.HookReg, rt.Profile, orchOpts...)

	go func() { <-ctx.Done(); _ = rl.Close() }()

	logSink := &loggingTerminalSink{}
	repl := &readlineREPL{
		baseCtx: ctx,
		rt:      rt,
		slashEnv: slashcmd.SlashEnv{
			Workdir:          rt.Workdir,
			UserConfigDir:    rt.Cfg.UserConfigDir,
			Profs:            rt.Profs,
			UserAgentsDir:    rt.UserAgentsDir,
			ProjectAgentsDir: rt.ProjectAgentsDir,
			Doctor: func(ctx context.Context) (string, error) {
				return DoctorReportFromRuntime(ctx, rt), nil
			},
			Focus:   focus,
			ToolLog: func(n int) string { return logSink.FormatToolLog(n) },
		},
		orch:  orch,
		focus: focus,
		sink:  logSink,
	}
	repl.replSession = rt.Sess
	getSlashCtx = func() slashcmd.SlashContext {
		return slashcmd.SlashContext{
			SlashEnv: repl.slashEnv,
			Mem:      repl.rt.MemStore,
			Orch:     repl.orch,
			Sess:     &repl.replSession,
			Store:    repl.rt.Store,
		}
	}
	repl.slashEnv.ChatSubtitle = func() string {
		return FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), orch.ProfileName())
	}
	repl.slashEnv.SessionModel = func() string { return rt.Cfg.Model() }
	repl.slashEnv.SetSessionModel = func(id string) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("model id is empty")
		}
		if strings.ToLower(strings.TrimSpace(rt.Cfg.Provider)) != "ollama" {
			return fmt.Errorf("/model applies to Ollama only (current provider: %s)", rt.Cfg.Provider)
		}
		rt.Cfg.OllamaModel = id
		orch.SetConfig(rt.Cfg)
		return nil
	}
	repl.slashEnv.PlanGate = func() slashcmd.PlanGateConfig {
		return slashcmd.PlanGateConfig{
			RequireApplyApproval: rt.Cfg.PlanRequireApplyApproval,
			ApplyUseCoordinator:  rt.Cfg.PlanApplyUseCoordinator,
			AgentPickerHide:      slices.Clone(rt.Cfg.AgentPickerHiddenProfiles),
		}
	}
	repl.run(rl, intCh)

	slog.Info("saving session", "id", rt.Sess.ID, "messages", rt.Sess.Len())
	if err := rt.Store.Save(rt.Sess); err != nil {
		slog.Error("failed to save session", "err", err)
	}
	return nil
}
