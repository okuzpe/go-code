package slashcmd

import (
	"path/filepath"
	"strings"
)

// SessionLocationBanner is a short cwd line for the readline REPL after startup.
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
func UserCapabilitiesGuide() string {
	return strings.TrimSpace(`
Here's what you can do with goclaw:

Code & Development
  - Write code — new features, scripts, full applications
  - Fix bugs — paste an error or describe the problem
  - Refactor — clean up, restructure, or optimize existing code
  - Explain code — understand any file or function in the workspace

File & Project Management
  - Explore projects — structure, find files, search patterns (glob, grep)
  - Edit files — targeted changes across the workspace (write_file, edit_file)
  - Create files — source, configs, docs (within workspace rules)

Terminal & Commands
  - Run shell commands — bash (single command) or script (when enabled) for build, test, install
  - Automate tasks — steps the tools and permission policy allow

Git & GitHub
  - Commits & PRs — via bash when permitted; paste diffs for review
  - Code review — analyze changes and suggest improvements

Research & Q&A
  - Web search — docs, articles, solutions (web_search)
  - Answer questions — technologies, frameworks, concepts (plain chat)
  - Read URLs — fetch and summarize pages you provide (web_fetch)

---
How to use me:
  - Just describe what you want in plain language
  - Paste code, errors, or file paths directly
  - Use /help to see available commands

What are you working on?
`)
}
