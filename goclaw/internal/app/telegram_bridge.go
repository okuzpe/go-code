package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/telegram"
	"github.com/spf13/cobra"
)

// RunTelegramStart runs the Telegram bridge, prompting to merge ~/.goclaw/settings.local.json when token or allowlist is missing (TTY only).
func RunTelegramStart(cmd *cobra.Command, _ []string) error {
	cfg, launchDir, token, err := telegramLoadPreflight(cmd)
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" || len(cfg.TelegramAllowedUserIDs) == 0 {
		if !telegramInteractive() {
			return errors.New("telegram start: missing bot token or telegram_allowed_user_ids; run in a terminal for guided setup, run `goclaw telegram configure`, or set GOCLAW_TELEGRAM_BOT_TOKEN and GOCLAW_TELEGRAM_ALLOWED_USER_IDS")
		}
		if err := runTelegramConfigureWizard(cfg); err != nil {
			return err
		}
		cfg, launchDir, err = loadMergedConfigForRun(cmd)
		if err != nil {
			return fmt.Errorf("telegram start: reload config: %w", err)
		}
		if err := applyTelegramSessionDefault(cmd, cfg); err != nil {
			return fmt.Errorf("telegram start: apply session default: %w", err)
		}
		token, err = cfg.ResolveTelegramBotToken(launchDir)
		if err != nil {
			return err
		}
		if strings.TrimSpace(token) == "" || len(cfg.TelegramAllowedUserIDs) == 0 {
			return errors.New("telegram start: configuration still incomplete after setup (check token and telegram_allowed_user_ids in ~/.goclaw/settings.local.json)")
		}
	}
	if err := telegramAssertComplete(token, cfg); err != nil {
		return err
	}
	return telegramRunBridgeLoop(cmd, token)
}

// RunTelegramBridge long-polls the Telegram Bot API and runs one orchestrator turn per allowlisted text message (no prompts; fails if unset).
func RunTelegramBridge(cmd *cobra.Command, _ []string) error {
	cfg, _, token, err := telegramLoadPreflight(cmd)
	if err != nil {
		return err
	}
	if err := telegramAssertComplete(token, cfg); err != nil {
		return fmt.Errorf("%w; for guided setup run `goclaw telegram start` in a terminal", err)
	}
	return telegramRunBridgeLoop(cmd, token)
}

func telegramLoadPreflight(cmd *cobra.Command) (cfg config.Config, launchDir string, token string, err error) {
	cfg, launchDir, err = loadMergedConfigForRun(cmd)
	if err != nil {
		return config.Config{}, "", "", err
	}
	if err := applyTelegramSessionDefault(cmd, cfg); err != nil {
		return config.Config{}, "", "", err
	}
	tok, err := cfg.ResolveTelegramBotToken(launchDir)
	if err != nil {
		return config.Config{}, "", "", err
	}
	return cfg, launchDir, tok, nil
}

func applyTelegramSessionDefault(cmd *cobra.Command, cfg config.Config) error {
	if sid := strings.TrimSpace(cfg.TelegramSessionID); sid != "" {
		if s, err := cmd.Flags().GetString("session"); err == nil && strings.TrimSpace(s) == "" {
			if err := cmd.Flags().Set("session", sid); err != nil {
				return fmt.Errorf("telegram: set session from telegram_session_id: %w", err)
			}
		}
	}
	return nil
}

func telegramAssertComplete(token string, cfg config.Config) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("telegram bridge: set telegram_bot_token or telegram_bot_token_file in settings (prefer ~/.goclaw/settings.local.json), or set GOCLAW_TELEGRAM_BOT_TOKEN")
	}
	if len(cfg.TelegramAllowedUserIDs) == 0 {
		return errors.New("telegram bridge: set non-empty telegram_allowed_user_ids (or GOCLAW_TELEGRAM_ALLOWED_USER_IDS) so only your Telegram user id can use the bot")
	}
	return nil
}

func telegramRunBridgeLoop(cmd *cobra.Command, token string) error {
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	maybeWarnOllamaUnreachable(rt)

	slog.Info("telegram bridge listening", "allowed_user_count", len(rt.Cfg.TelegramAllowedUserIDs), "session_id", rt.Sess.ID)

	client := telegram.NewClient(token)
	allowed := telegram.AllowedUserIDs(rt.Cfg.TelegramAllowedUserIDs)
	var offset int64

	for {
		// NotifyContext only ends on SIGINT/SIGTERM — any ctx error is a deliberate stop.
		if ctx.Err() != nil {
			slog.Info("telegram bridge: stopped")
			return nil
		}
		updates, err := client.GetUpdates(ctx, offset)
		if err != nil {
			// After Ctrl+C, Windows HTTP sometimes returns an error that does not unwrap
			// to context.Canceled even though the request context is already done.
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				slog.Info("telegram bridge: stopped")
				return nil
			}
			slog.Warn("telegram getUpdates failed", "err", err)
			select {
			case <-ctx.Done():
				slog.Info("telegram bridge: stopped")
				return nil
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if n := len(updates); n > 0 {
			slog.Info("telegram bridge: received updates", "count", n)
		}
		for _, u := range updates {
			userID, chatID, text, ok := telegram.UserAndChatFromUpdate(u)
			if !ok {
				if u.Message != nil && u.Message.From != nil {
					slog.Debug("telegram bridge: skipped update (only plain text messages are handled)", "update_id", u.UpdateID, "from_user_id", u.Message.From.ID)
				}
				continue
			}
			if _, ok := allowed[userID]; !ok {
				slog.Info("telegram bridge: ignored message (sender not in telegram_allowed_user_ids); compare from_user_id with your id from @userinfobot or settings",
					"from_user_id", userID)
				continue
			}
			slog.Info("telegram bridge: running turn", "from_user_id", userID, "chat_id", chatID, "text_preview", truncate(strings.TrimSpace(text), 120))
			if err := runTelegramTurn(ctx, rt, client, chatID, text); err != nil {
				if ctx.Err() != nil || errors.Is(err, context.Canceled) {
					slog.Info("telegram bridge: stopped")
					return nil
				}
				slog.Warn("telegram bridge: turn failed", "err", err)
				msg := fmt.Sprintf("[goclaw] turn error: %v", err)
				if sendErr := client.SendMessageText(ctx, chatID, msg); sendErr != nil {
					if ctx.Err() != nil || errors.Is(sendErr, context.Canceled) {
						slog.Info("telegram bridge: stopped")
						return nil
					}
					slog.Warn("telegram bridge: failed to send error reply", "err", sendErr)
				}
			}
		}
		if len(updates) > 0 {
			offset = telegram.NextOffset(updates)
		}
	}
}

func runTelegramTurn(ctx context.Context, rt *ChatRuntime, tg *telegram.Client, chatID int64, line string) error {
	if rt.Mock {
		reply, err := StreamMockAssistant(ctx, line, NopStreamSink{}, rt.Sess)
		if err != nil {
			return err
		}
		if err := tg.SendMessageText(ctx, chatID, reply); err != nil {
			return err
		}
		slog.Info("telegram bridge: reply sent", "chat_id", chatID, "reply_runes", utf8.RuneCountInString(reply))
		if err := rt.Store.Save(rt.Sess); err != nil {
			slog.Warn("telegram bridge: save session failed", "err", err)
		}
		return nil
	}

	orch := orchestrator.New(rt.Cfg, rt.Client, rt.Sess, rt.Reg, rt.Policy, rt.HookReg, rt.Profile, withAutomationOutputToolApprover(rt.OrchOpts)...)
	stopTyping := startTelegramTypingLoop(ctx, tg, chatID)
	defer stopTyping()
	t0 := time.Now()
	slog.Info("telegram bridge: model turn started", "provider", rt.Cfg.Provider, "model", rt.Cfg.Model())
	reply, err := orch.RunStreaming(ctx, line, NopStreamSink{})
	elapsed := time.Since(t0)
	if err != nil {
		slog.Warn("telegram bridge: model turn failed", "err", err, "elapsed", elapsed)
		return err
	}
	slog.Info("telegram bridge: model turn completed", "elapsed", elapsed, "reply_runes", utf8.RuneCountInString(reply))
	if err := tg.SendMessageText(ctx, chatID, reply); err != nil {
		return fmt.Errorf("send telegram reply: %w", err)
	}
	slog.Info("telegram bridge: reply sent", "chat_id", chatID, "reply_runes", utf8.RuneCountInString(reply))
	if err := rt.Store.Save(rt.Sess); err != nil {
		slog.Warn("telegram bridge: save session failed", "err", err)
	}
	return nil
}

// startTelegramTypingLoop calls sendChatAction(typing) until cancel; Telegram hides the indicator after ~5s, so refresh every 4s while the model runs.
func startTelegramTypingLoop(parent context.Context, tg *telegram.Client, chatID int64) (stop func()) {
	child, cancel := context.WithCancel(parent)
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		sendOnce := func() {
			c, c2 := context.WithTimeout(context.Background(), sendChatActionHTTPTimeout)
			defer c2()
			if err := tg.SendTyping(c, chatID); err != nil {
				slog.Debug("telegram bridge: sendChatAction typing failed", "err", err)
			}
		}
		sendOnce()
		for {
			select {
			case <-child.Done():
				return
			case <-ticker.C:
				sendOnce()
			}
		}
	}()
	return cancel
}

const sendChatActionHTTPTimeout = 12 * time.Second
