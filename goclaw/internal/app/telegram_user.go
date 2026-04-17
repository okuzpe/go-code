package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/telegram"
	"github.com/spf13/cobra"
)

// RunTelegramUserAdd resolves allowlist entries (digits, #digits, @PublicUsername) and merges them
// into telegram_allowed_user_ids in ~/.goclaw/settings.local.json (deduplicated, sorted).
func RunTelegramUserAdd(_ *cobra.Command, args []string) error {
	csv := flattenTelegramAllowlistArgs(args)
	if csv == "" {
		return errors.New(`telegram user add: need at least one entry (numeric id, #id, or @PublicUsername); example: goclaw telegram user add 123456789`)
	}

	cfg, launchDir, err := loadMergedConfigForRun(nil)
	if err != nil {
		return fmt.Errorf("telegram user add: load config: %w", err)
	}
	token, err := cfg.ResolveTelegramBotToken(launchDir)
	if err != nil {
		return fmt.Errorf("telegram user add: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("telegram user add: set telegram_bot_token (or telegram_bot_token_file / GOCLAW_TELEGRAM_BOT_TOKEN) so @username entries can be resolved")
	}

	client := telegram.NewClient(token)
	resolveCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	newIDs, err := telegram.ParseAllowlistEntries(resolveCtx, client, csv)
	if err != nil {
		return fmt.Errorf("telegram user add: %w", err)
	}

	path := config.UserSettingsLocalPath(cfg.UserConfigDir)
	existing, err := readTelegramAllowlistFromLocalFile(path)
	if err != nil {
		return fmt.Errorf("telegram user add: read %s: %w", path, err)
	}
	merged := mergeDedupeSortedInt64(existing, newIDs)

	if err := config.MergeWriteSettings(path, map[string]any{
		"telegram_allowed_user_ids": merged,
	}); err != nil {
		return fmt.Errorf("telegram user add: write settings: %w", err)
	}

	if v := strings.TrimSpace(os.Getenv("GOCLAW_TELEGRAM_ALLOWED_USER_IDS")); v != "" {
		_, _ = fmt.Fprintln(os.Stderr, "telegram user add: note: GOCLAW_TELEGRAM_ALLOWED_USER_IDS is set; it replaces the JSON allowlist at runtime. Unset it to use the file you just updated.")
	}

	_, _ = fmt.Fprintf(os.Stdout, "Updated %s — telegram_allowed_user_ids now has %d entr(y/ies).\n", path, len(merged))
	return nil
}

func flattenTelegramAllowlistArgs(args []string) string {
	var parts []string
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		for _, p := range strings.Split(a, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				parts = append(parts, p)
			}
		}
	}
	return strings.Join(parts, ",")
}

func readTelegramAllowlistFromLocalFile(path string) ([]int64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m struct {
		TelegramAllowedUserIDs []int64 `json:"telegram_allowed_user_ids"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m.TelegramAllowedUserIDs, nil
}

func mergeDedupeSortedInt64(a, b []int64) []int64 {
	seen := make(map[int64]struct{}, len(a)+len(b))
	for _, id := range a {
		seen[id] = struct{}{}
	}
	for _, id := range b {
		seen[id] = struct{}{}
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
