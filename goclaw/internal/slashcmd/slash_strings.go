package slashcmd

import (
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/planfile"
	"github.com/okuzpe/goclaw/internal/session"
)

// PopularSlashHint is a compact slash reference (e.g. after /help topics or automation banners).
func PopularSlashHint(workdir string) string {
	var b strings.Builder
	b.WriteString("Popular slash commands (most are local; /btw and /continue also send one user line to the model):\n")
	b.WriteString("  /help   /capabilities   /doctor   /plan   /apply-plan [--preview] [--hub]   /review   /btw   /continue   /copy   /export   /init   /memory   /model   /theme   /workers   /focus   /in   /detach   /back   /compact   /agents   /profile   /allow-writes   /resume   /clear   /quit\n")
	b.WriteString("Prefix input (see docs/goclaw/prefix-input-modes.md):  !cmd   @path   &task\n")
	if strings.TrimSpace(workdir) != "" {
		b.WriteString("Plan: ")
		b.WriteString(planfile.Path(workdir))
		b.WriteByte('\n')
	}
	return b.String()
}

// PreChatHelpSummary is a longer reference (e.g. after /help topics).
func PreChatHelpSummary(workdir string) string {
	var b strings.Builder
	b.WriteString("Slash commands (after chat starts; not sent to the model):\n")
	b.WriteString("  /help, help, ? — full list with session id and profile\n")
	b.WriteString("  /capabilities — structured overview (what the agent can do; not sent to the model)\n")
	b.WriteString("  TUI transcript scroll: PgUp/PgDn, Alt+arrows; mouse wheel opt-in (tui_mouse_scroll or GOCLAW_TUI_MOUSE_SCROLL=1)\n")
	b.WriteString("  TUI icons: tui_icons or GOCLAW_TUI_ICONS — emoji (default), unicode (▣), ascii, nerd (Nerd Fonts)\n")
	b.WriteString("  /plan path|init|save|run|review|approve|revoke|steps|template — .goclaw/plan.md (run = save + execute; optional --hub)\n")
	b.WriteString("  /init — create .goclaw/settings.json with coding defaults if missing\n")
	b.WriteString("  /apply-plan [--preview] [--hub] [path] — preview, or execute (general-purpose or coordinator; one turn)\n")
	b.WriteString("  /review [args] — inject git diff, switch to code-review (see docs/goclaw/code-review-workflow.md)\n")
	b.WriteString("  /memory list | add | delete — durable memory under ~/.goclaw/memory/\n")
	b.WriteString("  /workers, /focus or /in <id>, /back or /detach — interactive spawn_agent workers\n")
	b.WriteString("  /compact, /copy, /export, /edit, /init, /agents, /profile, /theme, /new, /save, /session, /sessions, /resume, /clear, /quit, /btw, /continue, /audit, /review\n")
	b.WriteString("Prefix: ! (bash), @ (read_file), & (spawn_agent) — single line; docs/goclaw/prefix-input-modes.md\n")
	b.WriteString("Flags: --no-tools, --session <id>, --profile <name>\n")
	if strings.TrimSpace(workdir) != "" {
		b.WriteString("Plan file: ")
		b.WriteString(planfile.Path(workdir))
		b.WriteByte('\n')
	}
	return b.String()
}

func replHelpText(env SlashEnv, sess **session.Session, orch *orchestrator.Orchestrator) string {
	id := "(none)"
	if sess != nil && *sess != nil {
		id = (*sess).ID
	}
	var b strings.Builder
	b.WriteString("Slash commands (not sent to the model):\n")
	b.WriteString("  /help, help, ?   — this text\n")
	b.WriteString("  TUI transcript: PgUp/PgDn · Alt+arrows · mouse wheel opt-in (tui_mouse_scroll true or GOCLAW_TUI_MOUSE_SCROLL=1)\n")
	b.WriteString("  /capabilities    — what I can help with (overview; not sent to the model)\n")
	b.WriteString("  /doctor          — health check (config, provider reachability, session paths)\n")
	b.WriteString("  /session         — show full session id and message count\n")
	b.WriteString("  /sessions        — list saved session ids (same as --list-sessions, without restart)\n")
	b.WriteString("  /resume <id>     — load a saved session (auto-saves current session first; use /sessions for ids)\n")
	b.WriteString("  /clear           — clear the transcript (TUI: Ctrl+L)\n")
	b.WriteString("  /quit, /exit     — save session and exit (same shutdown path as Ctrl+C)\n")
	b.WriteString("  /new             — save current session to disk, start a fresh empty session\n")
	b.WriteString("  /save            — write current session JSONL without exiting\n")
	b.WriteString("  /compact         — force context compaction (keep recent tail)\n")
	b.WriteString("  /copy            — copy plain session transcript to the system clipboard (truncates if very large)\n")
	b.WriteString("  /export <path>   — write plain session transcript to a file (relative paths use workspace when set)\n")
	b.WriteString("  /edit            — compose a multiline message in $EDITOR (vi, notepad, …)\n")
	b.WriteString("  /init            — create .goclaw/settings.json with coding defaults if missing\n")
	b.WriteString("  /memory list     — list memory files under ~/.goclaw/memory/\n")
	b.WriteString("  /memory add <type> <name> <text...>  — types: user | feedback | project | reference\n")
	b.WriteString("  /memory delete <file.md> — remove one file (see list for basename)\n")
	b.WriteString("  /agents [name]   — list agents or switch (TUI: Ctrl+P picker when bare /agents)\n")
	b.WriteString("  /profile <name>  — switch agent profile (same as /agents <name>)\n")
	b.WriteString("  /allow-writes    — auto-approve write_file, edit_file, patch for this session (no per-call prompts)\n")
	if env.SetSessionModel != nil && env.SessionModel != nil {
		b.WriteString("  /model [id]      — show or set the Ollama model tag for this session\n")
	}
	b.WriteString("  /theme [preset]  — show or set TUI ui_appearance (restart TUI to apply)\n")
	b.WriteString("  /workers — list workers; /focus or /in <prefix> — jump into worker; /back or /detach — return to coordinator\n")
	b.WriteString("  /plan path|init|save|run|review|approve|revoke|steps|template — plan workflow + approval gate helpers\n")
	b.WriteString("  /apply-plan [--preview] [--hub] [path] — preview, or execute (general-purpose or coordinator with --hub; one turn)\n")
	b.WriteString("  /audit [path]    — switch to general-purpose; audit-and-fix workflow on path (default: workspace)\n")
	b.WriteString("  /review [args]   — inject git diff; switch to code-review (read-only; see docs/goclaw/code-review-workflow.md)\n")
	b.WriteString("  /btw <text>      — side question: one user message with a brief-aside preamble (sent to the model)\n")
	b.WriteString("  /continue        — follow-up: ask the model to keep working on your last user request (sent to the model)\n")
	b.WriteString("  Ctrl+C           — exit (session is saved on shutdown)\n")
	b.WriteString("\nPrefix input (before model; same tools and permissions; single line each; see docs/goclaw/prefix-input-modes.md):\n")
	b.WriteString("  !<command>       — bash tool\n")
	b.WriteString("  @<path>          — read_file in the workspace\n")
	b.WriteString("  &<task>          — spawn_agent (general-purpose; requires spawn_agent on the active profile)\n")
	b.WriteString("\nRestart CLI flags: --session <id>  --list-sessions  --no-tools  --profile <name>\n")
	b.WriteString("Env: interactive chat requires a TTY; GOCLAW_USE_TUI=0 on a TTY is unsupported — use --output-format json for pipes.\n")
	b.WriteString("Env: GOCLAW_TUI_MOUSE_SCROLL=1 enables mouse wheel on the TUI transcript (default off; see tui_mouse_scroll in settings).\n")
	b.WriteString("Env: GOCLAW_TUI_ICONS sets footer/workspace glyphs (emoji|unicode|ascii|nerd); see tui_icons in settings.\n")
	b.WriteString("Env: GOCLAW_AGENT_PROFILE overrides agent_profile from settings (e.g. coordinator or general-purpose).\n")
	b.WriteString("CLI subcommand: goclaw sessions list (same as --list-sessions)\n")
	b.WriteString("Current session id: ")
	b.WriteString(id)
	b.WriteByte('\n')
	if orch != nil {
		b.WriteString("Active profile (this session): ")
		b.WriteString(orch.ProfileName())
		b.WriteByte('\n')
	}
	if strings.TrimSpace(env.Workdir) != "" {
		b.WriteString("Workspace plan file: ")
		b.WriteString(planfile.Path(env.Workdir))
		b.WriteByte('\n')
	}
	return b.String()
}

func previewRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if max <= 0 || s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "…"
}

// sortedProfileNames returns a comma-separated sorted list of profile names for error messages.
func sortedProfileNames(profs map[string]agents.Profile) string {
	if len(profs) == 0 {
		return ""
	}
	return strings.Join(agents.SortedKeys(profs), ", ")
}
