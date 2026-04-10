package chat

import (
	"path/filepath"
	"strings"
)

// SessionIntroSystemText is the first system-style line in the TUI transcript (cwd-aware).
func SessionIntroSystemText(workdir string) string {
	w := strings.TrimSpace(workdir)
	if w == "" {
		return strings.TrimSpace(`
---
No workspace directory was passed in — for best results, start goclaw from a project folder.

What project or task can I help with today?

Ask what I can do in plain language, or run /capabilities for the full guide. Use /help for slash commands.
---
`)
	}
	if abs, err := filepath.Abs(w); err == nil {
		w = abs
	}
	return strings.TrimSpace(`
---
You are in directory:

  ` + w + `

What project or task can I help with today?

Ask what I can do in plain language, or run /capabilities for the full guide. Use /help for slash commands.
---
`)
}
