package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/spf13/cobra"
)

// FullscreenChatRunner runs the Bubble Tea TUI. Implemented in cmd/goclaw so tests of
// package app do not import internal/ui/chat (Windows non-TTY init can block test binaries).
type FullscreenChatRunner interface {
	RunFullscreenChat(ctx context.Context, rt *ChatRuntime) error
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
// fullscreen runs the TUI when selected; it must be non-nil whenever the TUI path is reachable.
func RunChat(cmd *cobra.Command, version string, _ []string, fullscreen FullscreenChatRunner) error {
	readlineFlag, err := cmd.Flags().GetBool("readline")
	if err != nil {
		return err
	}
	tuiFlag, err := cmd.Flags().GetBool("tui")
	if err != nil {
		return err
	}
	useJSON, err := automationUsesJSON(cmd)
	if err != nil {
		return err
	}

	forceReadline := readlineFlag || strings.TrimSpace(os.Getenv("GOCLAW_USE_READLINE")) == "1"
	explicitTUI := tuiFlag || strings.TrimSpace(os.Getenv("GOCLAW_USE_TUI")) == "1"
	denyTUI := strings.TrimSpace(os.Getenv("GOCLAW_USE_TUI")) == "0"
	if useJSON && explicitTUI && !forceReadline {
		return errors.New("--output-format json and --json-output cannot be used with --tui or GOCLAW_USE_TUI=1")
	}

	mockFlag, err := cmd.Flags().GetBool("mock")
	if err != nil {
		return err
	}

	// Default TUI on an interactive terminal; opt out with GOCLAW_USE_TUI=0 or readline flags.
	useTUI := isTTY(os.Stdout) && !forceReadline && (explicitTUI || !denyTUI)
	ttyChat := isTTY(os.Stdout) && isTTY(os.Stdin)

	earlyCfg, workdir, err := loadMergedConfigForRun(cmd)
	if err != nil {
		return err
	}
	if ShouldRunOnboarding(ttyChat, useJSON, mockFlag, earlyCfg.UserConfigDir) {
		if err := RunOnboarding(version, workdir, useTUI, earlyCfg); err != nil {
			if errors.Is(err, ErrOnboardingAborted) {
				return nil
			}
			return err
		}
	}

	rt, err := PrepareChatRuntime(cmd)
	if err != nil {
		return err
	}
	maybeWarnOllamaUnreachable(rt.Cfg)
	defer func() {
		for _, s := range rt.McpSessions {
			_ = s.Close()
		}
	}()
	defer func() {
		_ = rt.HookReg.Fire(context.Background(), hooks.Event{Type: hooks.SessionEnd})
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	if useJSON {
		return RunChatJSONOutput(ctx, rt)
	}

	// Fullscreen TUI shows welcome panel + footer; skip ASCII banner to avoid scrollback flash and duplicate UX.
	if !useTUI {
		printStartupBanner(version, rt.Cfg.Provider, rt.Cfg.Model(), rt.Profile.Name, rt.Sess.ID, rt.Workdir, rt.DisableTools)
	}

	if !useTUI && isTTY(os.Stdout) {
		if banner := slashcmd.SessionLocationBanner(rt.Workdir); banner != "" {
			fmt.Println(banner)
			fmt.Println()
		}
		fmt.Print(slashcmd.PopularSlashHint(rt.Workdir))
		fmt.Println()
	}

	if useTUI {
		if fullscreen == nil {
			return errors.New("goclaw: TUI mode requires a fullscreen runner (cmd wiring bug)")
		}
		signal.Reset(os.Interrupt)
		err := fullscreen.RunFullscreenChat(ctx, rt)
		slog.Info("saving session", "id", rt.Sess.ID, "messages", rt.Sess.Len())
		if saveErr := rt.Store.Save(rt.Sess); saveErr != nil {
			slog.Error("failed to save session", "err", saveErr)
		}
		return err
	}

	return runReadlineREPL(ctx, rt, rt.OrchOpts)
}

// automationUsesJSON returns true when stdout should emit JSON for one-line stdin automation
// (--json-output or --output-format json). If --json-output is set, JSON mode wins even when
// --output-format is text.
func automationUsesJSON(cmd *cobra.Command) (bool, error) {
	jsonOut, err := cmd.Flags().GetBool("json-output")
	if err != nil {
		return false, err
	}
	format, err := cmd.Flags().GetString("output-format")
	if err != nil {
		return false, err
	}
	switch strings.TrimSpace(strings.ToLower(format)) {
	case "", "text":
		return jsonOut, nil
	case "json":
		return true, nil
	default:
		return false, fmt.Errorf("invalid --output-format %q (use text or json)", format)
	}
}

// RunPrompt runs a single user turn from argv text (see `goclaw prompt`) and exits.
func RunPrompt(cmd *cobra.Command, args []string) error {
	line := strings.TrimSpace(strings.Join(args, " "))
	if line == "" {
		return errors.New(`prompt: need a non-empty message (example: goclaw prompt "summarize README.md")`)
	}

	readlineFlag, err := cmd.Flags().GetBool("readline")
	if err != nil {
		return err
	}
	tuiFlag, err := cmd.Flags().GetBool("tui")
	if err != nil {
		return err
	}
	useJSON, err := automationUsesJSON(cmd)
	if err != nil {
		return err
	}

	forceReadline := readlineFlag || strings.TrimSpace(os.Getenv("GOCLAW_USE_READLINE")) == "1"
	explicitTUI := tuiFlag || strings.TrimSpace(os.Getenv("GOCLAW_USE_TUI")) == "1"
	if useJSON && explicitTUI && !forceReadline {
		return errors.New("--output-format json and --json-output cannot be used with --tui or GOCLAW_USE_TUI=1")
	}
	if !useJSON && explicitTUI && !forceReadline {
		return errors.New("prompt: use readline or text output for one-shot prompts; --tui is for interactive chat only")
	}

	rt, err := PrepareChatRuntime(cmd)
	if err != nil {
		return err
	}
	maybeWarnOllamaUnreachable(rt.Cfg)
	defer func() {
		for _, s := range rt.McpSessions {
			_ = s.Close()
		}
	}()
	defer func() {
		_ = rt.HookReg.Fire(context.Background(), hooks.Event{Type: hooks.SessionEnd})
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	if useJSON {
		return RunChatJSONOutputFromLine(ctx, rt, line)
	}
	return RunChatTextOutputFromLine(ctx, rt, line)
}

// loadMergedConfigForRun mirrors the config merge applied in PrepareChatRuntime (without building clients)
// so onboarding can run before the full runtime exists.
func loadMergedConfigForRun(cmd *cobra.Command) (config.Config, string, error) {
	workdir, err := os.Getwd()
	if err != nil {
		return config.Config{}, "", fmt.Errorf("get working directory: %w", err)
	}
	cfg := config.Default()
	cfg, err = config.Load(cfg, workdir)
	if err != nil {
		return config.Config{}, "", fmt.Errorf("load config: %w", err)
	}
	if ep := strings.TrimSpace(os.Getenv("GOCLAW_AGENT_PROFILE")); ep != "" {
		cfg.AgentProfile = ep
	}
	if cmd != nil {
		if p, err := cmd.Flags().GetString("profile"); err == nil && strings.TrimSpace(p) != "" {
			cfg.AgentProfile = strings.TrimSpace(p)
		}
		if vals, err := cmd.Flags().GetStringSlice("plugin-dir"); err == nil && len(vals) > 0 {
			cfg.PluginDirs = append(cfg.PluginDirs, vals...)
		}
	}
	return cfg, workdir, nil
}
