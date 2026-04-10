package slashcmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
)

func handleSlashTheme(env SlashEnv, fields []string) (handled bool, out string, quit bool, modelSubmit string, err error) {
	dir := strings.TrimSpace(env.UserConfigDir)
	if dir == "" {
		return true, "", false, "", fmt.Errorf("/theme: user config directory not configured")
	}
	if env.Workdir == "" {
		return true, "", false, "", fmt.Errorf("/theme: workspace directory not configured")
	}

	if len(fields) < 2 {
		cfg := config.Default()
		cfg.UserConfigDir = dir
		merged, err := config.Load(cfg, env.Workdir)
		if err != nil {
			return true, "", false, "", fmt.Errorf("load config: %w", err)
		}
		var b strings.Builder
		b.WriteString("TUI appearance (ui_appearance): ")
		b.WriteString(merged.UIAppearance)
		b.WriteString("\n\nPresets:\n  auto")
		for _, name := range config.UIAppearanceChoices {
			b.WriteString("\n  ")
			b.WriteString(name)
		}
		b.WriteString("\n\nUsage: /theme <preset>  — merges into ")
		b.WriteString(filepath.Join(dir, "settings.json"))
		b.WriteString("\nRestart the fullscreen TUI to apply colors fully.")
		return true, b.String(), false, "", nil
	}

	preset, err := parseThemePresetArg(fields[1])
	if err != nil {
		return true, "", false, "", err
	}
	userPath := filepath.Join(dir, "settings.json")
	if err := config.MergeWriteSettings(userPath, map[string]any{"ui_appearance": preset}); err != nil {
		return true, "", false, "", fmt.Errorf("/theme: write settings: %w", err)
	}
	msg := fmt.Sprintf("ui_appearance set to %q (merged into %s).\nRestart the TUI to apply the theme fully.", preset, userPath)
	return true, msg, false, "", nil
}

func parseThemePresetArg(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", fmt.Errorf("/theme: need a preset name (see /theme with no args)")
	}
	if s == config.UIAppearanceAuto {
		return config.UIAppearanceAuto, nil
	}
	for _, c := range config.UIAppearanceChoices {
		if s == c {
			return c, nil
		}
	}
	return "", fmt.Errorf("/theme: unknown preset %q — run /theme for the list", raw)
}
