package slashcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/okuzpe/goclaw/internal/config"
)

func tryInteractiveThemePick(userConfigDir, _ string, current string, disable bool) (out string, usedTTY bool, err error) {
	if disable {
		return "", false, nil
	}
	fd := int(os.Stdin.Fd())
	items := config.ThemePresetList()
	if len(items) == 0 {
		return "", false, nil
	}
	start := 0
	for i, name := range items {
		if name == current {
			start = i
			break
		}
	}
	preset, res, perr := pickListTTY(fd, os.Stdin, os.Stdout, "TUI appearance — ↑↓ move · Enter apply · Esc cancel", items, start)
	if perr != nil {
		return "", false, nil
	}
	switch res {
	case ttyListPickNone:
		return "", false, nil
	case ttyListPickCancelled:
		return "/theme cancelled.", true, nil
	case ttyListPickChosen:
		userPath := filepath.Join(userConfigDir, "settings.json")
		if err := config.MergeWriteSettings(userPath, map[string]any{"ui_appearance": preset}); err != nil {
			return "", true, err
		}
		msg := fmt.Sprintf("ui_appearance set to %q (merged into %s).\nRestart the TUI to apply the theme fully.", preset, userPath)
		return msg, true, nil
	default:
		return "", false, nil
	}
}
