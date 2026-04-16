package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveTelegramBotToken returns the bot token from explicit value, env (already merged in Load),
// or telegram_bot_token_file. Returns ("", nil) when nothing is configured.
func (c Config) ResolveTelegramBotToken(launchDir string) (string, error) {
	if tok := strings.TrimSpace(c.TelegramBotToken); tok != "" {
		return tok, nil
	}
	path := strings.TrimSpace(c.TelegramBotTokenFile)
	if path == "" {
		return "", nil
	}
	full := path
	if !filepath.IsAbs(path) {
		full = filepath.Join(launchDir, path)
	}
	full = filepath.Clean(full)
	b, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("read telegram_bot_token_file %s: %w", full, err)
	}
	return strings.TrimSpace(string(b)), nil
}
