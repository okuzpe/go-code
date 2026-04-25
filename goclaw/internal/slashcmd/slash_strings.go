package slashcmd

import (
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/planfile"
	"github.com/okuzpe/goclaw/internal/session"
)

func slashNamesByGroup(group string) string {
	names := make([]string, 0, len(slashCommandTable))
	for _, cmd := range slashCommandTable {
		if cmd.Group == group {
			names = append(names, cmd.Name)
		}
	}
	return strings.Join(names, "   ")
}

// PopularSlashHint is a compact slash reference (e.g. after /help topics or automation banners).
func PopularSlashHint(workdir string) string {
	var b strings.Builder
	b.WriteString("Quick start by intent:\n")
	b.WriteString("  Start:      ")
	b.WriteString(slashNamesByGroup("start"))
	b.WriteByte('\n')
	b.WriteString("  Build flow: ")
	b.WriteString(slashNamesByGroup("build"))
	b.WriteByte('\n')
	b.WriteString("  Session:    ")
	b.WriteString(slashNamesByGroup("session"))
	b.WriteByte('\n')
	b.WriteString("  Workers:    ")
	b.WriteString(slashNamesByGroup("workers"))
	b.WriteByte('\n')
	b.WriteString("Prefix input (see docs/goclaw/prefix-input-modes.md):  !cmd   @path   &task\n")
	if strings.TrimSpace(workdir) != "" {
		b.WriteString("Plan: ")
		b.WriteString(planfile.Path(workdir))
		b.WriteByte('\n')
	}
	return b.String()
}

// PreChatHelpSummary is a short pre-chat pointer; /help (ReplHelpMarkdown) is the full reference.
func PreChatHelpSummary(workdir string) string {
	var b strings.Builder
	b.WriteString("Slash commands are not sent to the model. /help - full list (session, mode/profile, TUI keys); /capabilities - what the agent can do.\n")
	b.WriteString("Primary flow: /mode /plan /apply-plan /continue /memory /theme /quit\n")
	b.WriteString("Advanced: /profile /review /audit /workers - line prefixes ! @ & - docs/goclaw/prefix-input-modes.md\n")
	if strings.TrimSpace(workdir) != "" {
		b.WriteString("Plan file: ")
		b.WriteString(planfile.Path(workdir))
		b.WriteByte('\n')
	}
	return b.String()
}

// ReplHelpMarkdown is the /help body as markdown (TUI overlay + tests; not sent to the model).
func ReplHelpMarkdown(env SlashEnv, sess **session.Session, orch *orchestrator.Orchestrator) string {
	id := "(none)"
	if sess != nil && *sess != nil {
		id = (*sess).ID
	}
	var b strings.Builder
	b.WriteString("## Slash commands\n\n")
	b.WriteString("Not sent to the model.\n\n")
	b.WriteString("### Primary flow\n\n")
	b.WriteString("- `/help`, `help`, `?` — this text\n")
	b.WriteString("- **TUI transcript** — PgUp/PgDn · Alt+arrows · mouse wheel opt-in (`tui_mouse_scroll` or `GOCLAW_TUI_MOUSE_SCROLL=1`); Ctrl+B enters focused browse mode\n")
	b.WriteString("- `/capabilities` — overview of what the agent can do\n")
	b.WriteString("- `/doctor` — health check (config, provider reachability, session paths)\n")
	b.WriteString("- `/session` — full session id and message count\n")
	b.WriteString("- `/sessions` — list saved session ids (same as `--list-sessions`)\n")
	b.WriteString("- `/resume <id>` — load a saved session (auto-saves current first; use `/sessions` for ids)\n")
	b.WriteString("- `/clear` — clear the transcript (TUI: **Ctrl+L**)\n")
	b.WriteString("- `/quit`, `/exit` — save session and exit (same path as **Ctrl+C**)\n")
	b.WriteString("- `/new` — save current session, start a fresh empty session\n")
	b.WriteString("- `/save` — write current session JSONL without exiting\n")
	b.WriteString("- `/compact` — force context compaction (keep recent tail)\n")
	b.WriteString("- `/copy` — copy plain session transcript to the clipboard (truncates if very large)\n")
	b.WriteString("- `/export <path>` — write plain transcript to a file (relative paths use workspace when set)\n")
	b.WriteString("- `/edit` — compose a multiline message in `$EDITOR`\n")
	b.WriteString("- `/init` — create `.goclaw/settings.json` with coding defaults if missing\n")
	b.WriteString("- `/memory list` — list memory files under `~/.goclaw/memory/`\n")
	b.WriteString("- `/memory add <type> <name> <text...>` — types: `user` | `feedback` | `project` | `reference`\n")
	b.WriteString("- `/memory delete <file.md>` — remove one file (see list for basename)\n")
	b.WriteString("- `/mode <build|plan>` — switch primary mode\n")
	b.WriteString("- `/allow-writes` — auto-approve `write_file`, `edit_file`, `patch` for this session\n")
	if env.SetSessionModel != nil && env.SessionModel != nil {
		b.WriteString("- `/model [id]` — show or set the Ollama model tag for this session\n")
	}
	b.WriteString("- `/theme [preset]` — show or set TUI `ui_appearance` (bare `/theme` opens the live picker in the fullscreen TUI)\n")
	b.WriteString("- `/plan path|init|new|save|run|review|approve|revoke|steps|template` — plan workflow + mini plans under `.goclaw/plans/`\n")
	b.WriteString("- `/apply-plan [--preview] [--hub] [--steps] [path]` — preview or execute; `--steps` runs one turn per `## Steps` line when present\n")
	b.WriteString("- `/btw <text>` — side question (sent to the model)\n")
	b.WriteString("- `/continue` — follow-up on your last user request (sent to the model)\n")
	b.WriteString("\n### Advanced\n\n")
	b.WriteString("- `/profile <name>` — switch advanced profile / compatibility alias (TUI: **Ctrl+P** or bare `/profile`)\n")
	b.WriteString("- `/agents [name]` — compatibility alias for profile listing/switching\n")
	b.WriteString("- `/workers` — list workers; `/focus` or `/in <prefix>` — jump into worker; `/back` or `/detach` — return to coordinator\n")
	b.WriteString("- `/audit [path]` — switch to build; audit-and-fix on path (default: workspace)\n")
	b.WriteString("- `/review [args]` — inject git diff; code-review profile (`docs/goclaw/code-review-workflow.md`)\n")
	b.WriteString("- **Ctrl+C** — exit (session saves on shutdown)\n")
	b.WriteString("\n## Prefix input\n\n")
	b.WriteString("Before model; same tools and permissions; single line each — `docs/goclaw/prefix-input-modes.md`.\n\n")
	b.WriteString("- `!<command>` — bash tool\n")
	b.WriteString("- `@<path>` — read_file in the workspace\n")
	b.WriteString("- `&<task>` — spawn_agent (requires `spawn_agent` on the active profile)\n")
	b.WriteString("\n## CLI and environment\n\n")
	b.WriteString("- Restart flags: `--session <id>` `--list-sessions` `--no-tools` `--mode <build|plan>` `--profile <name>` (`--profile` is the advanced override)\n")
	b.WriteString("- Interactive chat needs a TTY; `GOCLAW_USE_TUI=0` on a TTY is unsupported — use `--output-format json` for pipes.\n")
	b.WriteString("- `GOCLAW_TUI_MOUSE_SCROLL=1` — mouse wheel on transcript (see `tui_mouse_scroll` in settings).\n")
	b.WriteString("- `GOCLAW_TUI_ICONS` / `tui_icons` — footer glyphs: emoji, unicode, ascii, nerd.\n")
	b.WriteString("- `GOCLAW_AGENT_PROFILE` — overrides `agent_profile` from settings.\n")
	b.WriteString("- `goclaw sessions list` — same as `--list-sessions`.\n")
	b.WriteString("\n## Session\n\n")
	b.WriteString("- **Current session id:** `" + id + "`\n")
	if orch != nil {
		b.WriteString("- **Active mode/profile:** `" + agents.DisplayProfileName(orch.ProfileName()) + "`\n")
	}
	if strings.TrimSpace(env.Workdir) != "" {
		b.WriteString("- **Workspace plan file:** `" + planfile.Path(env.Workdir) + "`\n")
		b.WriteString("- **Mini plans directory:** `" + planfile.PlansDir(env.Workdir) + "`\n")
	}
	return strings.TrimSpace(b.String())
}

func replHelpText(env SlashEnv, sess **session.Session, orch *orchestrator.Orchestrator) string {
	return ReplHelpMarkdown(env, sess, orch)
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

// researchSlug converts a research query into a short filename-safe slug (max 32 chars).
func researchSlug(query string) string {
	var b strings.Builder
	limit := 32
	wrote := 0
	prevDash := false
	for _, r := range strings.ToLower(query) {
		if wrote >= limit {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			wrote++
			prevDash = false
		} else if !prevDash && wrote > 0 {
			b.WriteByte('-')
			wrote++
			prevDash = true
		}
	}
	result := strings.TrimRight(b.String(), "-")
	if result == "" {
		return "query"
	}
	return result
}

// sortedProfileNames returns a comma-separated sorted list of profile names for error messages.
func sortedProfileNames(profs map[string]agents.Profile) string {
	if len(profs) == 0 {
		return ""
	}
	keys := agents.UserFacingSortedKeys(profs)
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, agents.DisplayProfileName(k))
	}
	return strings.Join(names, ", ")
}
