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

	"github.com/chzyer/readline"
	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/slashcmd"
)

func replHistoryFile(userConfigDir string) string {
	return filepath.Join(userConfigDir, "history")
}

func replPrompt(s *session.Session, focus *coordinator.FocusRouter) string {
	if s == nil {
		return "> "
	}
	id := s.ID
	if len(id) > 8 {
		id = id[:8]
	}
	if focus != nil {
		if w := strings.TrimSpace(focus.Current()); w != "" {
			wid := w
			if len(wid) > 8 {
				wid = wid[:8]
			}
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
			preview = toolInput
			if len(preview) > 400 {
				preview = preview[:400] + "…"
			}
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
) {
	reqCtx, reqCancel := context.WithCancel(baseCtx)
	setReqCancel(reqCancel)
	var runErr error
	if mock {
		_, runErr = StreamMockAssistant(reqCtx, userText, sink, sess)
	} else {
		_, runErr = orch.RunStreaming(reqCtx, userText, sink)
	}
	runErr = AugmentOrchestratorErr(provider, model, runErr)
	setReqCancel(nil)
	reqCancel()
	if runErr != nil && !(errors.Is(runErr, context.Canceled) && baseCtx.Err() == nil) {
		slog.Error("orchestrator error", "err", runErr)
		fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
	}
}

type readlineREPL struct {
	baseCtx  context.Context
	rt       *ChatRuntime
	slashEnv slashcmd.SlashEnv
	orch     *orchestrator.Orchestrator
	focus    *coordinator.FocusRouter
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

	// sess is a local working copy of the session pointer. Slash commands (e.g. /new)
	// may replace it with a new *session.Session; we sync it back to r.rt.Sess after
	// each HandleSlash call (line below). The signal-handler goroutine only calls
	// reqCancelFn, which aborts the in-flight orchestrator turn — it never touches sess.
	// Once RunStreaming returns (before we touch sess again), the goroutine is guaranteed
	// not to be writing to sess, so this pattern is safe without a mutex.
	sess := r.rt.Sess

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
		rl.SetPrompt(replPrompt(sess, r.focus))
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

		sc := slashcmd.SlashContext{SlashEnv: r.slashEnv, Mem: r.rt.MemStore, Orch: r.orch, Sess: &sess, Store: r.rt.Store}
		handled, slashOut, quit, modelSubmit, sErr := slashcmd.HandleSlash(r.baseCtx, sc, input)
		r.rt.Sess = sess
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
				runOrchestratorTurn(r.baseCtx, r.rt.Mock, r.rt.Cfg.Provider, r.rt.Cfg.Model(), r.orch, sess, modelSubmit, &terminalSink{}, setReqCancel)
			}
			continue
		}

		if wid := strings.TrimSpace(r.focus.Current()); wid != "" {
			runWorkerTurn(r.baseCtx, r.rt.Cfg.Provider, r.rt.Cfg.Model(), wid, input, &terminalSink{}, setReqCancel)
			continue
		}

		if handled, err := RunLocalPrefixToolIfAny(r.baseCtx, r.rt.Mock, r.orch, sess, input, &terminalSink{}); handled {
			if err != nil {
				slog.Error("prefix input error", "err", err)
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			continue
		}

		input = ExpandInlineAtRefs(r.baseCtx, r.orch, input)
		runOrchestratorTurn(r.baseCtx, r.rt.Mock, r.rt.Cfg.Provider, r.rt.Cfg.Model(), r.orch, sess, input, &terminalSink{}, setReqCancel)
	}
}

func runWorkerTurn(
	baseCtx context.Context,
	provider, model, taskID, userText string,
	sink orchestrator.StreamSink,
	setReqCancel func(context.CancelFunc),
) {
	reqCtx, reqCancel := context.WithCancel(baseCtx)
	setReqCancel(reqCancel)
	err := coordinator.DeliverWorkerMessage(reqCtx, taskID, userText, sink)
	err = AugmentOrchestratorErr(provider, model, err)
	setReqCancel(nil)
	reqCancel()
	if err != nil && !(errors.Is(err, context.Canceled) && baseCtx.Err() == nil) {
		slog.Error("worker turn error", "err", err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
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

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          replPrompt(rt.Sess, focus),
		HistoryFile:     historyFile,
		HistoryLimit:    500,
		AutoComplete:    slashcmd.NewReadlineSlashAtCompleter(rt.Workdir),
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
			Focus: focus,
		},
		orch:  orch,
		focus: focus,
	}
	repl.run(rl, intCh)

	slog.Info("saving session", "id", rt.Sess.ID, "messages", rt.Sess.Len())
	if err := rt.Store.Save(rt.Sess); err != nil {
		slog.Error("failed to save session", "err", err)
	}
	return nil
}
