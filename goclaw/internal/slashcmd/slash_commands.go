package slashcmd

import "strings"

// SlashCommandSuggest is one row for TUI slash picking (Name includes leading /).
type SlashCommandSuggest struct {
	Name    string
	Summary string
	Group   string
}

// slashCommandTable is the canonical list of root REPL / commands. Keep sorted by Name.
var slashCommandTable = []SlashCommandSuggest{
	{"/agents", "List or switch agent profile (built-in + custom *.md)", "workers"},
	{"/apply-plan", "Execute saved plan (see --preview); optional path; switches to general-purpose", "build"},
	{"/audit", "Scan project for gaps and auto-fix them (review-and-fix workflow). Optional: /audit <path>", "build"},
	{"/back", "Return to coordinator session (same as /detach)", "workers"},
	{"/btw", "Side question: rewrite and send one user message to the model", "build"},
	{"/capabilities", "Print full capability guide (no model call)", "start"},
	{"/clear", "Clear the transcript (TUI: same idea as Ctrl+L)", "session"},
	{"/compact", "Force context compaction on the current session", "session"},
	{"/continue", "Send a follow-up to finish pending work (same session context)", "build"},
	{"/copy", "Copy plain session transcript to the system clipboard", "session"},
	{"/detach", "Stop routing input to the focused worker", "workers"},
	{"/doctor", "Print health report: Ollama, MCP, permissions, paths", "start"},
	{"/edit", "Open $EDITOR to compose a multiline message", "build"},
	{"/exit", "Save session and quit", "session"},
	{"/export", "Write plain session transcript to a file (path argument)", "session"},
	{"/focus", "Route input to a coordinator worker task id", "workers"},
	{"/help", "List slash commands and usage", "start"},
	{"/hub", "Return to coordinator (alias of /detach)", "workers"},
	{"/in", "Focus a worker by task id prefix (same as /focus)", "workers"},
	{"/init", "Create .goclaw/settings.json with coding defaults if missing", "build"},
	{"/memory", "list | add | delete durable memory entries", "session"},
	{"/model", "Show or set default Ollama model tag for this session", "session"},
	{"/new", "Start a new session (saves current)", "session"},
	{"/parent", "Return to coordinator (same as /detach)", "workers"},
	{"/plan", "path | init | new | save | run | review | approve | revoke | steps | template (plan.md + .goclaw/plans/)", "build"},
	{"/profile", "Switch agent profile (hot-reloads custom agents)", "workers"},
	{"/quit", "Save session and quit", "session"},
	{"/research", "Web research + plan: /research <query> — searches the web and builds a step-by-step plan saved to .goclaw/plans/", "build"},
	{"/resume", "Load a saved session from disk (auto-saves current session first)", "session"},
	{"/review", "Git diff review (read-only): /review | /review --staged | /review rev [rev] | /review rev -- path", "build"},
	{"/save", "Persist session without exiting", "session"},
	{"/session", "Show current session id and message count", "session"},
	{"/sessions", "List saved session ids on disk", "session"},
	{"/theme", "Set TUI appearance preset in settings", "session"},
	{"/tools", "Show tool call history (plain text when wired); TUI: prefer Ctrl+T; /tools N shows full output of step N", "session"},
	{"/workers", "List coordinator workers and task ids", "workers"},
}

func slashFirstToken(line string) string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/") {
		return ""
	}
	for i := 1; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			return line[:i]
		}
	}
	return line
}

func filterSlashByToken(token string) []SlashCommandSuggest {
	tok := strings.ToLower(token)
	if tok == "" {
		return nil
	}
	var out []SlashCommandSuggest
	for _, c := range slashCommandTable {
		if strings.HasPrefix(strings.ToLower(c.Name), tok) {
			out = append(out, c)
		}
	}
	return out
}

// TUISlashSuggestions returns commands whose name prefix-matches the first /token in buffer.
// The buffer must be a single logical line (no newlines). Empty or non-slash input returns nil.
func TUISlashSuggestions(buffer string) []SlashCommandSuggest {
	if strings.Contains(buffer, "\n") {
		return nil
	}
	raw := strings.TrimSpace(buffer)
	if raw == "" || !strings.HasPrefix(raw, "/") {
		return nil
	}
	return filterSlashByToken(slashFirstToken(raw))
}

// SlashTabExpand applies prefix completion for a single-line slash buffer (Tab-longest-prefix style).
// If ok is false, the caller should forward the key to the input widget.
func SlashTabExpand(line string) (replacement string, ok bool) {
	if strings.Contains(line, "\n") {
		return "", false
	}
	raw := strings.TrimSpace(line)
	if raw == "" || !strings.HasPrefix(raw, "/") {
		return "", false
	}
	tok := slashFirstToken(raw)
	matches := filterSlashByToken(tok)
	if len(matches) == 0 {
		return "", false
	}
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.Name
	}
	lcp := longestCommonPrefix(names)
	rest := strings.TrimSpace(strings.TrimPrefix(raw, tok))
	if len(lcp) > len(tok) {
		if rest != "" {
			return lcp + " " + rest, true
		}
		return lcp, true
	}
	pick := matches[0].Name
	if strings.EqualFold(tok, pick) {
		if rest == "" {
			return pick + " ", true
		}
		return "", false
	}
	if rest != "" {
		return pick + " " + rest, true
	}
	return pick + " ", true
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	ref := strs[0]
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		for j := 1; j < len(strs); j++ {
			s := strs[j]
			if i >= len(s) || s[i] != c {
				return ref[:i]
			}
		}
	}
	return ref
}
