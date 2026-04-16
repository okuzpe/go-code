package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/telegram"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// telegramInteractive reports whether stdin and stdout are both terminals (wizard safe).
func telegramInteractive() bool {
	if !isTTY(os.Stdout) {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// RunTelegramConfigure runs an interactive wizard that merges Telegram keys into the user settings.local.json.
func RunTelegramConfigure(_ *cobra.Command, _ []string) error {
	if !telegramInteractive() {
		return errors.New("telegram configure: requires an interactive terminal (stdin and stdout must be TTYs); edit ~/.goclaw/settings.local.json or set GOCLAW_TELEGRAM_BOT_TOKEN and GOCLAW_TELEGRAM_ALLOWED_USER_IDS")
	}
	workdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("telegram configure: get working directory: %w", err)
	}
	cfg := config.Default()
	cfg, err = config.Load(cfg, workdir)
	if err != nil {
		return fmt.Errorf("telegram configure: load config: %w", err)
	}
	if err := runTelegramConfigureWizard(cfg); err != nil {
		return err
	}
	path := config.UserSettingsLocalPath(cfg.UserConfigDir)
	fmt.Fprintf(os.Stdout, "Updated %s. Run: goclaw telegram start\n", path)
	return nil
}

func runTelegramConfigureWizard(cfg config.Config) error {
	path := config.UserSettingsLocalPath(cfg.UserConfigDir)
	fmt.Fprintf(os.Stdout, "Telegram setup — values merge into %s\n", path)
	fmt.Fprintln(os.Stdout, "See docs/goclaw/telegram-bridge.md. Input is echoed; use a private terminal.")
	fmt.Fprintln(os.Stdout, "")

	reader := bufio.NewReader(os.Stdin)

	token, err := readNonEmptyLine(reader, "Bot token (from BotFather): ")
	if err != nil {
		return err
	}

	rawIDs, err := readNonEmptyLine(reader, "Your Telegram allowlist: user id(s) (digits or #digits) and/or @YourPublicUsername, comma-separated. Your personal account, not the bot: ")
	if err != nil {
		return err
	}
	client := telegram.NewClient(strings.TrimSpace(token))
	resolveCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ids, err := telegram.ParseAllowlistEntries(resolveCtx, client, rawIDs)
	if err != nil {
		return fmt.Errorf("telegram configure: %w", err)
	}
	if len(ids) == 0 {
		return errors.New("telegram configure: need at least one allowlist entry")
	}

	sessionLine, err := readLine(reader, "Optional session id to resume (Enter to skip): ")
	if err != nil {
		return err
	}
	sessionLine = strings.TrimSpace(sessionLine)

	patch := map[string]any{
		"telegram_bot_token":        strings.TrimSpace(token),
		"telegram_allowed_user_ids": ids,
	}
	if sessionLine != "" {
		patch["telegram_session_id"] = sessionLine
	}
	if err := config.MergeWriteSettings(path, patch); err != nil {
		return fmt.Errorf("telegram configure: write settings: %w", err)
	}
	return nil
}

func readLine(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(os.Stdout, prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimSuffix(line, "\n"), nil
}

func readNonEmptyLine(reader *bufio.Reader, prompt string) (string, error) {
	for {
		line, err := readLine(reader, prompt)
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
		fmt.Fprintln(os.Stdout, "(value required)")
	}
}

