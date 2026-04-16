package slashcmd

import "strings"

// PlainHelpREPLRequest reports whether input is handled by the same paths as /help in HandleSlash
// (slash /help, or bare help / ?).
func PlainHelpREPLRequest(input string) bool {
	s := strings.TrimSpace(input)
	if s == "" {
		return false
	}
	low := strings.ToLower(s)
	if low == "help" || low == "?" {
		return true
	}
	if !strings.HasPrefix(s, "/") {
		return false
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false
	}
	return strings.ToLower(strings.TrimPrefix(fields[0], "/")) == "help"
}

// TUIHelpShortcutsText is a static shortcuts block prepended to the TUI help overlay (English).
func TUIHelpShortcutsText() string {
	return strings.TrimSpace(`
Shortcuts
  Enter              send message
  Ctrl+P             open agent profile picker (same as /agents Enter)
  Shift+Enter / Alt+Enter   newline in the input
  / then type        filter slash commands (single-line input only)
  Tab                complete /command or the argument under the cursor (profiles, /memory …, paths for /export)
  Esc                close this help panel (when open)
  Ctrl+C             quit the TUI (session saves on exit)
  Ctrl+L             clear transcript (TUI)
  /clear             clear the terminal screen (readline; same idea as Ctrl+L)
  /edit              compose a multiline message in $EDITOR
  /copy              copy plain session text to the system clipboard
  /export path.txt   save plain session text to a file

Transcript: Ctrl+B browse (↑↓ j/k scroll); PgUp/PgDn and Alt+arrows always. Mouse wheel is opt-in (tui_mouse_scroll true or GOCLAW_TUI_MOUSE_SCROLL=1) so normal terminal selection stays usable. /copy or /export for the full session.

Prefix input (single line; same permissions as tools — see docs/goclaw/prefix-input-modes.md)
  !cmd               run bash tool (allowlisted shell)
  @path               read_file in the workspace (TUI: path list + Tab; readline: Tab)
  &task               spawn_agent (worker profile general-purpose; parent must allow spawn_agent)
  /btw text           side question — one wrapped message to the model
  /continue           follow-up — keep working on your last user request (sent to the model)

For Esc vs streaming vs exit, follow the footer hint line under the input.

Docs: docs/goclaw/usage.md (monorepo) and CLAUDE.md in the goclaw module.

Architecture note
  Orchestrator     The agent loop runtime — drives one agent turn (LLM + tools + repeat).
                   Every profile runs inside an orchestrator. Not user-visible; it is the engine.
  Coordinator      Default hub profile (--profile coordinator). spawn_agent delegates to workers;
                   the parent session has no direct read/write/shell tools. Synthesizes worker results.
  General-purpose  Direct coding in this session: explore → implement → verify with full tools (no hub).
                   Tool cards: short result lines when useful; Ctrl+T (TUI) or /tools (readline) for history.
`)

}
