package slashcmd

import (
	"path/filepath"
	"strings"
)

// SessionLocationBanner is a short cwd line after startup (non-TTY banner / automation).
func SessionLocationBanner(workdir string) string {
	w := strings.TrimSpace(workdir)
	if w == "" {
		return ""
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

// UserCapabilitiesGuide is the full structured overview (slash /capabilities, no LLM tokens).
// Markdown for TUI glamour; keep plain-language section titles for tests and readability.
func UserCapabilitiesGuide() string {
	var b strings.Builder
	b.WriteString("Here's what you can do with **goclaw**:\n\n")
	b.WriteString("## Code & Development\n\n")
	b.WriteString("- Write code — new features, scripts, full applications\n")
	b.WriteString("- Fix bugs — paste an error or describe the problem\n")
	b.WriteString("- Refactor — clean up, restructure, or optimize existing code\n")
	b.WriteString("- Explain code — understand any file or function in the workspace\n\n")
	b.WriteString("## File & Project Management\n\n")
	b.WriteString("- Explore projects — structure, find files, search patterns (glob, grep)\n")
	b.WriteString("- Edit files — targeted changes across the workspace (write_file, edit_file, patch)\n")
	b.WriteString("- Create files — source, configs, docs (within workspace rules)\n\n")
	b.WriteString("## Terminal & Commands\n\n")
	b.WriteString("- Run shell commands — bash (single command) or script (when enabled) for build, test, install\n")
	b.WriteString("- Automate tasks — steps the tools and permission policy allow\n\n")
	b.WriteString("## Git & GitHub\n\n")
	b.WriteString("- Commits & PRs — via bash when permitted; paste diffs for review\n")
	b.WriteString("- Code review — /review runs git diff in the workspace, switches to the code-review profile, and streams one structured review turn (see docs/goclaw/code-review-workflow.md)\n\n")
	b.WriteString("## Research & Q&A\n\n")
	b.WriteString("- Web search — docs, articles, solutions (web_search)\n")
	b.WriteString("- Answer questions — technologies, frameworks, concepts (plain chat)\n")
	b.WriteString("- Read URLs — fetch and summarize pages you provide (web_fetch)\n\n")
	b.WriteString("---\n\n")
	b.WriteString("## How to use me\n\n")
	b.WriteString("- Just describe what you want in plain language\n")
	b.WriteString("- Paste code, errors, or file paths directly\n")
	b.WriteString("- Use /help to see available commands\n")
	b.WriteString("- Project setup: /init creates .goclaw/settings.json with coding defaults when the file is missing (optional starter CLAUDE.md and .gitignore hint for settings.local.json)\n")
	b.WriteString("- Sessions: /sessions lists saved ids; /resume <id> loads one without restarting the binary (current session is saved first); /clear clears the TUI transcript (**Ctrl+L**)\n\n")
	b.WriteString("What are you working on?\n")
	return strings.TrimSpace(b.String())
}
