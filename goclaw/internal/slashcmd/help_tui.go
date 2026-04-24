package slashcmd

import (
	"strings"

	"github.com/okuzpe/goclaw/internal/ui/icons"
)

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

// TUIHelpShortcutsText is a static shortcuts block (legacy plain text; Unicode icon preset).
// Prefer TUIHelpShortcutsMarkdown with the active TUI icon set.
func TUIHelpShortcutsText() string {
	return TUIHelpShortcutsMarkdown(icons.Unicode)
}

// TUIHelpShortcutsMarkdown is markdown for the shortcuts block prepended to the TUI help overlay (English).
func TUIHelpShortcutsMarkdown(st icons.Set) string {
	g := st.WorkspaceGlyph()
	var b strings.Builder
	b.WriteString("## Shortcuts\n\n")
	b.WriteString("> ")
	b.WriteString(g)
	b.WriteString(" Footer/workspace glyphs follow **tui_icons** / **GOCLAW_TUI_ICONS** (emoji · unicode · ascii · nerd).\n\n")
	b.WriteString("- **Enter** — send message\n")
	b.WriteString("- **Ctrl+P** — open the profile picker (same as bare `/profile` in the fullscreen TUI)\n")
	b.WriteString("- **Ctrl+Shift+←** / **Ctrl+Shift+→** — cycle the active mode/profile (order from **tui_profile_cycle** in settings; default build → plan). The top context bar updates immediately with the active mode/profile and model.\n")
	b.WriteString("- **Shift+Enter** / **Alt+Enter** — newline in the input\n")
	b.WriteString("- **Slash then type** — filter slash commands (single-line input only)\n")
	b.WriteString("- **Tab** — complete slash commands or the argument under the cursor (profiles, /memory …, paths for /export)\n")
	b.WriteString("- **Esc** — close this help panel (when open)\n")
	b.WriteString("- **Ctrl+C** — quit the TUI (session saves on exit)\n")
	b.WriteString("- **Ctrl+L** — clear transcript (TUI)\n")
	b.WriteString("- **Ctrl+M** — cycle interact mode (**chat** → **code** → **agent**); footer shows `mode:<mode>`. Optional default: **tui_interact_mode** in settings or **GOCLAW_TUI_INTERACT_MODE**. Some terminals send Ctrl+M as carriage return — if nothing changes, check the terminal profile.\n")
	b.WriteString("- **Ctrl+T** — open the session tool log (`/tools` does the same in the fullscreen TUI)\n")
	b.WriteString("- **/clear** — clear the transcript (same idea as Ctrl+L)\n")
	b.WriteString("- **/undo** — revert the last successful write_file, edit_file, or patch from this session\n")
	b.WriteString("- **/edit** — compose a multiline message in $EDITOR\n")
	b.WriteString("- **/copy** — copy plain session text to the system clipboard\n")
	b.WriteString("- **/export path.txt** — save plain session text to a file\n\n")
	b.WriteString("### Transcript\n\n")
	b.WriteString("PgUp/PgDn and Alt+arrows scroll the transcript anytime. Ctrl+B enters focused browse mode for ↑↓ j/k and tool-card drill-down. Mouse wheel is opt-in (**tui_mouse_scroll** or **GOCLAW_TUI_MOUSE_SCROLL=1**) so normal terminal selection stays usable. /copy or /export for the full session.\n\n")
	b.WriteString("## Prefix input\n\n")
	b.WriteString("Single line; same permissions as tools — see docs/goclaw/prefix-input-modes.md.\n\n")
	b.WriteString("- **!cmd** — run bash tool (allowlisted shell)\n")
	b.WriteString("- **@path** — read_file in the workspace (path list + Tab)\n")
	b.WriteString("- **&task** — spawn_agent (worker profile build/general-purpose; parent must allow spawn_agent)\n")
	b.WriteString("- **/btw text** — side question — one wrapped message to the model\n")
	b.WriteString("- **/continue** — follow-up — keep working on your last user request (sent to the model)\n\n")
	b.WriteString("For Esc vs streaming vs exit, follow the footer hint line under the input.\n\n")
	b.WriteString("## Docs\n\n")
	b.WriteString("- docs/goclaw/usage.md (monorepo)\n")
	b.WriteString("- CLAUDE.md in the goclaw module\n\n")
	b.WriteString("## Architecture note\n\n")
	b.WriteString("- **Orchestrator** — The agent loop runtime — drives one agent turn (LLM + tools + repeat). Every profile runs inside an orchestrator. Not user-visible; it is the engine.\n")
	b.WriteString("- **Coordinator** — Advanced hub profile (`--profile coordinator`). spawn_agent delegates to workers; the parent session has no direct read/write/shell tools. Synthesizes worker results.\n")
	b.WriteString("- **Build** — Primary direct-coding mode in this session (implemented by the internal `general-purpose` profile): explore → implement → verify with full tools. Tool cards stay compact in the transcript; use **Ctrl+T** or `/tools` for the full history.\n")
	return strings.TrimSpace(b.String())
}
