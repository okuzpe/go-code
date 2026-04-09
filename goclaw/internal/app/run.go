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

	rt, err := PrepareChatRuntime(cmd)
	if err != nil {
		return err
	}
	defer func() {
		for _, s := range rt.McpSessions {
			_ = s.Close()
		}
	}()
	defer func() {
		_ = rt.HookReg.Fire(context.Background(), hooks.Event{Type: hooks.SessionEnd})
	}()

	forceReadline := readlineFlag || strings.TrimSpace(os.Getenv("GOCLAW_USE_READLINE")) == "1"
	wantTUI := tuiFlag || strings.TrimSpace(os.Getenv("GOCLAW_USE_TUI")) == "1"
	useTUI := isTTY(os.Stdout) && !forceReadline && wantTUI

	printStartupBanner(version, rt.Cfg.Provider, rt.Cfg.Model(), rt.Profile.Name, rt.Sess.ID, rt.Workdir, rt.DisableTools, useTUI)

	if !useTUI && isTTY(os.Stdout) {
		fmt.Print(slashcmd.PopularSlashHint(rt.Workdir))
		fmt.Println()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

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
