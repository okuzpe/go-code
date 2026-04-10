package chat

import (
	"path/filepath"
	"strings"
)

// SessionIntroSystemText is the first system-style line in the TUI transcript (cwd-aware).
func SessionIntroSystemText(workdir string) string {
	w := strings.TrimSpace(workdir)
	if w == "" {
		return "No workspace — start goclaw from a project folder for best results."
	}
	if abs, err := filepath.Abs(w); err == nil {
		w = abs
	}
	return "Workspace: " + w
}
