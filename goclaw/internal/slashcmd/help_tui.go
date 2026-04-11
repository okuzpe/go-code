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
  Shift+Enter / Alt+Enter   newline in the input
  / then type        filter slash commands (single-line input only)
  Tab                complete /command
  Esc                close this help panel (when open)
  Ctrl+C             quit the TUI (session saves on exit)
  Ctrl+L             clear transcript (TUI)
  /clear             clear the terminal screen (readline; same idea as Ctrl+L)
  /edit              compose a multiline message in $EDITOR
  /copy              copy plain session text to the system clipboard
  /export path.txt   save plain session text to a file

Mouse: mouse reporting is off — select text normally with the mouse. Scroll with
  PgUp/PgDn, j/k, Ctrl+U/Ctrl+D. Use /copy or /export for the full session.

Prefix input (single line; same permissions as tools — see docs/goclaw/prefix-input-modes.md)
  !cmd               run bash tool (allowlisted shell)
  @path               read_file in the workspace (TUI: path list + Tab; readline: Tab)
  &task               spawn_agent (general-purpose; hub profile)
  /btw text           side question — one wrapped message to the model

For Esc vs streaming vs exit, follow the footer hint line under the input.

Docs: docs/goclaw/usage.md (monorepo) and CLAUDE.md in the goclaw module.
`)
}
