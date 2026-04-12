package slashcmd

import (
	"context"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/planfile"
	"github.com/okuzpe/goclaw/internal/session"

	"golang.org/x/term"
)

// SlashEnv carries workspace paths and profile lookup for slash commands.
type SlashEnv struct {
	Workdir       string
	UserConfigDir string // ~/.goclaw — for /theme merge-write
	// DisableInteractiveThemePick skips arrow-key /theme picker in plain REPL (e.g. when stdin is not a TTY).
	DisableInteractiveThemePick bool
	// DisableInteractiveAgentPick skips arrow-key /agents picker (fullscreen TUI uses its own overlay).
	DisableInteractiveAgentPick bool
	Profs                       map[string]agents.Profile
	UserAgentsDir               string // for hot-reload of custom profiles on /profile
	ProjectAgentsDir            string
	Doctor                      func(ctx context.Context) (string, error)
	// Focus is optional; when set, /focus (/in) and /detach (/back, /hub) route input to interactive workers.
	Focus *coordinator.FocusRouter
	// ChatSubtitle optional; after profile switches returns window subtitle (e.g. provider · model · profile).
	ChatSubtitle func() string
	// SessionModel returns the process default model id for the active provider (optional; for /model).
	SessionModel func() string
	// SetSessionModel updates the in-process default model id where supported (optional; for /model).
	SetSessionModel func(id string) error
	// ToolLog optional; returns formatted tool history text for the /tools command (readline mode only).
	ToolLog func(n int) string
}

// SlashContext carries dependencies for HandleSlash (memory, orchestrator, session pointer, disk store).
type SlashContext struct {
	SlashEnv
	Mem   *memory.Store
	Orch  *orchestrator.Orchestrator
	Sess  **session.Session
	Store *session.Store
}

// ErrReplQuit is returned by HandleSlash for /quit and /exit so the REPL can save and exit cleanly.
var ErrReplQuit = errors.New("repl quit")

// HandleSlash processes REPL slash commands. Returns handled=true if input was consumed.
// modelSubmit is non-empty when the caller should send that text to the model (e.g. /edit).
// quit with ErrReplQuit means the REPL should exit after printing out.
// hintsOut when non-nil receives TUI refresh hints (welcome bar, transcript rebuild); readline callers may pass nil.
func HandleSlash(ctx context.Context, sc SlashContext, input string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	mem := sc.Mem
	orch := sc.Orch
	sess := sc.Sess
	store := sc.Store
	env := sc.SlashEnv
	clearHints(hintsOut)

	s := strings.TrimSpace(input)
	if s == "" {
		return false, "", false, "", nil
	}
	low := strings.ToLower(s)
	if low == "help" || low == "?" {
		return true, replHelpText(env, sess, orch), false, "", nil
	}
	if !strings.HasPrefix(s, "/") {
		return false, "", false, "", nil
	}

	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false, "", false, "", nil
	}
	cmd := strings.ToLower(strings.TrimPrefix(fields[0], "/"))

	switch cmd {
	case "help":
		return true, replHelpText(env, sess, orch), false, "", nil

	case "btw":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/btw requires a running agent")
		}
		rest := strings.TrimSpace(strings.Join(fields[1:], " "))
		if rest == "" {
			return true, "", false, "", fmt.Errorf(`usage: /btw your question or note`)
		}
		return true, "", false, inputprefix.BtwRewrite(rest), nil

	case "doctor":
		if env.Doctor == nil {
			return true, "", false, "", fmt.Errorf("/doctor: not available (missing wiring)")
		}
		out, derr := env.Doctor(ctx)
		if derr != nil {
			return true, "", false, "", derr
		}
		return true, out, false, "", nil

	case "model":
		if env.SetSessionModel == nil || env.SessionModel == nil {
			return true, "", false, "", fmt.Errorf("/model: not available in this mode")
		}
		if len(fields) < 2 {
			return true, fmt.Sprintf("current model: %s\nusage: /model <id>", env.SessionModel()), false, "", nil
		}
		id := strings.TrimSpace(strings.Join(fields[1:], " "))
		if id == "" {
			return true, "", false, "", fmt.Errorf("usage: /model <id>")
		}
		if err := env.SetSessionModel(id); err != nil {
			return true, "", false, "", err
		}
		sub := ""
		if env.ChatSubtitle != nil {
			sub = env.ChatSubtitle()
		}
		if orch != nil {
			setWelcomeHints(hintsOut, orch, sub)
		}
		return true, fmt.Sprintf("model set to %q (this session)", id), false, "", nil

	case "tools":
		if env.ToolLog == nil {
			return true, "(tool history not available in this mode — use Ctrl+T in the TUI)", false, "", nil
		}
		n := 0
		if len(fields) >= 2 {
			fmt.Sscan(fields[1], &n)
		}
		return true, env.ToolLog(n), false, "", nil

	case "quit", "exit":
		return true, "bye", true, "", ErrReplQuit

	case "sessions":
		if store == nil {
			return true, "", false, "", fmt.Errorf("/sessions: no session store configured")
		}
		entries, err := store.ListSessionEntries()
		if err != nil {
			return true, "", false, "", err
		}
		if len(entries) == 0 {
			return true, "(no saved sessions on disk)", false, "", nil
		}
		var b strings.Builder
		for _, e := range entries {
			age := formatSessionModAge(e.ModTime)
			b.WriteString(e.ID)
			b.WriteString("  ")
			b.WriteString(age)
			b.WriteString("  (")
			b.WriteString(e.ModTime.UTC().Format(time.RFC3339))
			b.WriteString(")\n")
		}
		return true, strings.TrimSuffix(b.String(), "\n"), false, "", nil

	case "clear":
		if term.IsTerminal(int(os.Stdout.Fd())) {
			_, _ = fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
			return true, "(screen cleared)", false, "", nil
		}
		return true, "(screen clear skipped — stdout is not a terminal)", false, "", nil

	case "resume":
		if store == nil {
			return true, "", false, "", fmt.Errorf("/resume: no session store configured")
		}
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/resume requires a running agent")
		}
		if sess == nil {
			return true, "", false, "", fmt.Errorf("/resume: session pointer missing")
		}
		if len(fields) < 2 {
			return true, "", false, "", fmt.Errorf(`usage: /resume <session_id_or_prefix>
use /sessions to list saved ids; current session is auto-saved before switching`)
		}
		arg := strings.TrimSpace(strings.Join(fields[1:], " "))
		if *sess != nil {
			if err := store.Save(*sess); err != nil {
				return true, "", false, "", fmt.Errorf("/resume: save current session: %w", err)
			}
		}
		loaded, rerr := resolveSessionForResume(store, arg)
		if rerr != nil {
			return true, "", false, "", fmt.Errorf("/resume: %w", rerr)
		}
		if loaded == nil {
			return true, "", false, "", fmt.Errorf("/resume: session not found for %q", arg)
		}
		*sess = loaded
		orch.ReplaceSession(loaded)
		setReloadTranscript(hintsOut, loaded)
		return true, fmt.Sprintf("resumed session %s (%d messages).", loaded.ID, loaded.Len()), false, "", nil

	case "new":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/new requires a running agent (internal error)")
		}
		if sess == nil {
			return true, "", false, "", fmt.Errorf("/new: session pointer missing")
		}
		if store != nil && *sess != nil {
			if err := store.Save(*sess); err != nil {
				return true, "", false, "", fmt.Errorf("save current session before /new: %w (fix disk or permissions; session not reset)", err)
			}
		}
		next := session.New()
		*sess = next
		orch.ReplaceSession(next)
		return true, fmt.Sprintf("new empty session (previous transcript saved if a store is configured).\nnew session id: %s", next.ID), false, "", nil

	case "save":
		if store == nil {
			return true, "", false, "", fmt.Errorf("/save: no session store configured")
		}
		if sess == nil || *sess == nil {
			return true, "", false, "", fmt.Errorf("/save: no active session")
		}
		if err := store.Save(*sess); err != nil {
			return true, "", false, "", fmt.Errorf("save session: %w", err)
		}
		return true, fmt.Sprintf("(session saved: %s, %d messages)", (*sess).ID, (*sess).Len()), false, "", nil

	case "session":
		if sess == nil || *sess == nil {
			return true, "(no session)", false, "", nil
		}
		return true, fmt.Sprintf("session id: %s\nmessages: %d", (*sess).ID, (*sess).Len()), false, "", nil

	case "theme":
		return handleSlashTheme(env, fields)

	case "capabilities":
		return true, UserCapabilitiesGuide(), false, "", nil

	case "copy":
		const maxClipBytes = 768 * 1024
		if sess == nil || *sess == nil {
			return true, "", false, "", fmt.Errorf("/copy: no active session")
		}
		body := (*sess).PlainTranscript()
		if strings.TrimSpace(body) == "" {
			return true, "(nothing to copy — session transcript is empty)", false, "", nil
		}
		clipBody := body
		note := ""
		if len(clipBody) > maxClipBytes {
			clipBody = clipBody[:maxClipBytes]
			note = fmt.Sprintf(" (truncated to %d bytes for clipboard)", maxClipBytes)
		}
		if err := clipboard.WriteAll(clipBody); err != nil {
			return true, "", false, "", fmt.Errorf("/copy: clipboard: %w — try /export path.txt", err)
		}
		return true, fmt.Sprintf("(copied %d bytes to clipboard)%s", len(clipBody), note), false, "", nil

	case "export":
		if sess == nil || *sess == nil {
			return true, "", false, "", fmt.Errorf("/export: no active session")
		}
		if len(fields) < 2 {
			return true, "", false, "", fmt.Errorf(`usage: /export <path>  (plain session text; relative paths are under the workspace when set)`)
		}
		path := strings.TrimSpace(strings.Join(fields[1:], " "))
		if path == "" || strings.Contains(path, "..") {
			return true, "", false, "", fmt.Errorf("/export: invalid path")
		}
		body := (*sess).PlainTranscript()
		if strings.TrimSpace(body) == "" {
			return true, "(session is empty — nothing written)", false, "", nil
		}
		outPath := path
		if !filepath.IsAbs(path) && strings.TrimSpace(env.Workdir) != "" {
			outPath = filepath.Join(env.Workdir, path)
		}
		outPath = filepath.Clean(outPath)
		if err := os.WriteFile(outPath, []byte(body), 0o600); err != nil {
			return true, "", false, "", fmt.Errorf("/export: write %s: %w", outPath, err)
		}
		return true, fmt.Sprintf("(wrote %d bytes to %s)", len(body), outPath), false, "", nil

	case "compact":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/compact requires a running agent")
		}
		if sess == nil || *sess == nil {
			return true, "", false, "", fmt.Errorf("/compact: no active session")
		}
		before := (*sess).Len()
		orch.ForceCompact()
		after := (*sess).Len()
		return true, fmt.Sprintf("(compaction applied: %d → %d messages; older turns summarized; tail preserved)", before, after), false, "", nil

	case "edit":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/edit requires a running agent")
		}
		body, eerr := openPromptEditor(ctx, env.Workdir)
		if eerr != nil {
			return true, "", false, "", eerr
		}
		if body == "" {
			return true, "(no message from editor — nothing sent)", false, "", nil
		}
		return true, "(sending message from editor…)", false, body, nil

	case "init":
		msg, ierr := handleSlashProjectInit(env)
		if ierr != nil {
			return true, "", false, "", ierr
		}
		return true, msg, false, "", nil

	case "memory":
		if len(fields) < 2 {
			return true, "", false, "", fmt.Errorf(`usage: /memory list | /memory add <user|feedback|project|reference> <name> <words...>
example: /memory add project style Prefer tabs over spaces for Go imports`)
		}
		sub := strings.ToLower(fields[1])
		switch sub {
		case "list":
			list, lerr := mem.List()
			if lerr != nil {
				return true, "", false, "", lerr
			}
			if len(list) == 0 {
				return true, "(no memory entries)", false, "", nil
			}
			var b strings.Builder
			for _, e := range list {
				b.WriteString("- ")
				b.WriteString(e.Filename)
				b.WriteString(" [")
				b.WriteString(string(e.Type))
				b.WriteString("] ")
				b.WriteString(e.Name)
				if e.Description != "" {
					b.WriteString(" — ")
					b.WriteString(e.Description)
				}
				if preview := previewRunes(e.Body, 80); preview != "" {
					b.WriteString(" | ")
					b.WriteString(preview)
				}
				b.WriteByte('\n')
			}
			return true, strings.TrimSuffix(b.String(), "\n"), false, "", nil

		case "add":
			if len(fields) < 5 {
				return true, "", false, "", fmt.Errorf(`usage: /memory add <user|feedback|project|reference> <one-word-name> <text...>
example: /memory add user prefs Use British spelling in docs`)
			}
			typ := memory.Type(fields[2])
			switch typ {
			case memory.TypeUser, memory.TypeFeedback, memory.TypeProject, memory.TypeReference:
			default:
				return true, "", false, "", fmt.Errorf("invalid type %q — use user, feedback, project, or reference", fields[2])
			}
			name := fields[3]
			body := strings.Join(fields[4:], " ")
			if body == "" {
				return true, "", false, "", fmt.Errorf("memory text cannot be empty (add words after the name)")
			}
			desc := body
			if len(desc) > 160 {
				desc = desc[:160] + "…"
			}
			base, serr := mem.Save(memory.Entry{
				Type:        typ,
				Name:        name,
				Description: desc,
				Body:        body,
			})
			if serr != nil {
				return true, "", false, "", serr
			}
			return true, fmt.Sprintf("saved memory entry %q (%s)", base, typ), false, "", nil

		case "delete":
			if len(fields) < 3 {
				return true, "", false, "", fmt.Errorf(`usage: /memory delete <filename.md>
use /memory list to see basenames (e.g. mynote_a1b2c3d4.md)`)
			}
			base := fields[2]
			if err := mem.Delete(base); err != nil {
				return true, "", false, "", err
			}
			return true, fmt.Sprintf("deleted memory file %q", base), false, "", nil

		default:
			return true, "", false, "", fmt.Errorf("unknown /memory %q — use list, add, or delete", fields[1])
		}

	case "profile":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/profile requires a running agent")
		}
		if len(fields) < 2 {
			profs, _ := agents.AllWithCustom(env.UserAgentsDir, env.ProjectAgentsDir)
			return true, "", false, "", fmt.Errorf("usage: /profile <name>\nnames: %s", agents.JoinSortedProfileKeys(profs))
		}
		msg, err := switchOrchestratorProfile(orch, env, fields[1])
		if err != nil {
			return true, "", false, "", err
		}
		sub := ""
		if env.ChatSubtitle != nil {
			sub = env.ChatSubtitle()
		}
		setWelcomeHints(hintsOut, orch, sub)
		return true, msg, false, "", nil

	case "agents":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/agents requires a running agent")
		}
		profs, _ := agents.AllWithCustom(env.UserAgentsDir, env.ProjectAgentsDir)
		if len(fields) < 2 {
			if out, used, ierr := tryInteractiveAgentsPick(env, orch, hintsOut); ierr != nil {
				return true, "", false, "", ierr
			} else if used {
				return true, out, false, "", nil
			}
			return true, formatAgentsList(profs, orch.ProfileName()), false, "", nil
		}
		msg, err := switchOrchestratorProfile(orch, env, fields[1])
		if err != nil {
			return true, "", false, "", err
		}
		sub := ""
		if env.ChatSubtitle != nil {
			sub = env.ChatSubtitle()
		}
		setWelcomeHints(hintsOut, orch, sub)
		return true, msg, false, "", nil

	case "plan":
		wd := strings.TrimSpace(env.Workdir)
		if wd == "" {
			return true, "", false, "", fmt.Errorf("/plan: workspace directory not set")
		}
		if len(fields) < 2 {
			return true, "", false, "", fmt.Errorf(`usage: /plan path | /plan init | /plan save | /plan template
path     — show default plan file path
init     — create .goclaw/plan.md from template if missing
save     — save last assistant message in this session to .goclaw/plan.md
template — print the template to the terminal`)
		}
		sub := strings.ToLower(fields[1])
		switch sub {
		case "path":
			return true, planfile.Path(wd), false, "", nil
		case "init":
			created, ierr := planfile.Init(wd)
			if ierr != nil {
				return true, "", false, "", ierr
			}
			if created {
				return true, fmt.Sprintf("created %s", planfile.Path(wd)), false, "", nil
			}
			return true, fmt.Sprintf("already exists: %s", planfile.Path(wd)), false, "", nil
		case "save":
			if sc.Sess == nil || *sc.Sess == nil || len((*sc.Sess).Messages) == 0 {
				return true, "", false, "", fmt.Errorf("/plan save: no messages in current session")
			}
			lastText := ""
			for i := len((*sc.Sess).Messages) - 1; i >= 0; i-- {
				m := (*sc.Sess).Messages[i]
				if m.Role == "assistant" && strings.TrimSpace(m.Content) != "" {
					lastText = m.Content
					break
				}
			}
			if lastText == "" {
				return true, "", false, "", fmt.Errorf("/plan save: no assistant message in session to save")
			}
			if mkErr := os.MkdirAll(filepath.Join(wd, planfile.Subdir), 0o700); mkErr != nil {
				return true, "", false, "", fmt.Errorf("/plan save: mkdir: %w", mkErr)
			}
			planPath := planfile.Path(wd)
			if writeErr := os.WriteFile(planPath, []byte(lastText+"\n"), 0o600); writeErr != nil {
				return true, "", false, "", fmt.Errorf("/plan save: write: %w", writeErr)
			}
			setFooterHint(hintsOut, "Plan saved — /apply-plan --preview to review, then /apply-plan to execute.")
			return true, fmt.Sprintf("plan saved to %s\nRun /apply-plan --preview to review, then /apply-plan to execute.", planPath), false, "", nil
		case "template":
			return true, planfile.Template(), false, "", nil
		default:
			return true, "", false, "", fmt.Errorf("unknown /plan %q — use path, init, save, or template", fields[1])
		}

	case "workers":
		list := coordinator.ListInteractiveWorkers()
		if len(list) == 0 {
			return true, "(no interactive workers — spawn with spawn_agent interactive: true)", false, "", nil
		}
		var b strings.Builder
		for _, w := range list {
			b.WriteString(w.TaskID)
			b.WriteString("  ")
			b.WriteString(w.Profile)
			b.WriteString("  ")
			b.WriteString(w.Status)
			if strings.TrimSpace(w.Summary) != "" {
				b.WriteString("  — ")
				b.WriteString(previewRunes(w.Summary, 72))
			}
			b.WriteByte('\n')
		}
		return true, strings.TrimSuffix(b.String(), "\n"), false, "", nil

	case "detach", "back", "parent", "hub":
		if env.Focus == nil {
			return true, "", false, "", fmt.Errorf("focus routing not enabled (/detach, /back)")
		}
		env.Focus.Detach()
		return true, "focus: coordinator (parent session)", false, "", nil

	case "focus", "in":
		if env.Focus == nil {
			return true, "", false, "", fmt.Errorf("focus routing not enabled (/focus, /in)")
		}
		if len(fields) < 2 {
			return true, "", false, "", fmt.Errorf(`usage: /focus <task_id_prefix> | /focus parent   (alias: /in)
parent — return to coordinator (same as /back or /detach)
use /workers to list interactive worker ids`)
		}
		arg := strings.TrimSpace(strings.ToLower(fields[1]))
		if arg == "parent" || arg == ".." || arg == "coordinator" {
			env.Focus.Detach()
			return true, "focus: coordinator (parent session)", false, "", nil
		}
		prefix := strings.TrimSpace(fields[1])
		full, ok := coordinator.ResolveInteractiveTaskID(prefix)
		if !ok {
			return true, "", false, "", fmt.Errorf("no unique interactive worker matches prefix %q (try /workers)", prefix)
		}
		env.Focus.FocusTaskID(full)
		out := fmt.Sprintf("focus: worker %s (input goes here until /back or /detach)", full)
		if snap, ok := coordinator.SnapshotInteractiveWorker(full); ok && strings.TrimSpace(snap) != "" {
			out = "[worker history]\n" + snap + "\n" + out
		}
		return true, out, false, "", nil

	case "apply-plan":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/apply-plan requires a running agent")
		}
		wd := strings.TrimSpace(env.Workdir)
		if wd == "" {
			return true, "", false, "", fmt.Errorf("/apply-plan: workspace directory not set")
		}
		if env.Profs == nil {
			return true, "", false, "", fmt.Errorf("/apply-plan: profile map not configured")
		}
		gp, ok := env.Profs["general-purpose"]
		if !ok {
			return true, "", false, "", fmt.Errorf("/apply-plan: general-purpose profile missing")
		}
		rest := strings.TrimSpace(strings.TrimPrefix(s, fields[0]))
		pathTail, preview := parseApplyPlanRest(rest)
		p := planfile.ResolvePlanArg(wd, pathTail)
		body, rerr := planfile.Read(p)
		if rerr != nil {
			return true, "", false, "", rerr
		}
		if preview {
			out := formatPlanPreviewOutput(p, body)
			setFooterHint(hintsOut, "Review complete — run /apply-plan to execute (or /apply-plan --preview again).")
			return true, out, false, "", nil
		}
		orch.SetProfile(gp)
		msg := planfile.HandoffUserMessage(p, body)
		notice := fmt.Sprintf("switched to profile general-purpose; executing plan: %s", p)
		sub := ""
		if env.ChatSubtitle != nil {
			sub = env.ChatSubtitle()
		}
		setWelcomeHints(hintsOut, orch, sub)
		setFooterHint(hintsOut, "")
		return true, notice, false, msg, nil

	default:
		return true, "", false, "", fmt.Errorf("unknown command /%s — try /help", cmd)
	}
}

// PopularSlashHint is printed once after the startup banner on readline REPL (claw-style flow).
func PopularSlashHint(workdir string) string {
	var b strings.Builder
	b.WriteString("Popular slash commands (not sent to the model):\n")
	b.WriteString("  /help   /capabilities   /doctor   /plan   /apply-plan [--preview]   /btw   /copy   /export   /init   /memory   /model   /theme   /workers   /focus   /in   /detach   /back   /compact   /agents   /profile   /resume   /clear   /quit\n")
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
	b.WriteString("  /plan path|init|save|template — workspace plan under .goclaw/plan.md\n")
	b.WriteString("  /init — create .goclaw/settings.json with coding defaults if missing\n")
	b.WriteString("  /apply-plan [--preview] [path] — preview plan on disk, or execute (switch to general-purpose, stream one turn)\n")
	b.WriteString("  /memory list | add | delete — durable memory under ~/.goclaw/memory/\n")
	b.WriteString("  /workers, /focus or /in <id>, /back or /detach — interactive spawn_agent workers\n")
	b.WriteString("  /compact, /copy, /export, /edit, /init, /agents, /profile, /theme, /new, /save, /session, /sessions, /resume, /clear, /quit, /btw\n")
	b.WriteString("Prefix: ! (bash), @ (read_file), & (spawn_agent) — single line; docs/goclaw/prefix-input-modes.md\n")
	b.WriteString("Flags: --readline (line REPL), --no-tools, --session <id>, --profile <name>\n")
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
	b.WriteString("  TUI transcript: PgUp/PgDn · Alt+arrows · mouse wheel (default on; off via settings or GOCLAW_TUI_MOUSE_SCROLL=0)\n")
	b.WriteString("  /capabilities    — what I can help with (overview; not sent to the model)\n")
	b.WriteString("  /doctor          — health check (config, provider reachability, session paths)\n")
	b.WriteString("  /session         — show full session id and message count\n")
	b.WriteString("  /sessions        — list saved session ids (same as --list-sessions, without restart)\n")
	b.WriteString("  /resume <id>     — load a saved session (auto-saves current session first; use /sessions for ids)\n")
	b.WriteString("  /clear           — clear the terminal screen (readline; TUI uses Ctrl+L)\n")
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
	b.WriteString("  /agents [name]   — list agents or switch (arrow picker when bare in readline TTY)\n")
	b.WriteString("  /profile <name>  — switch agent profile (same as /agents <name>)\n")
	if env.SetSessionModel != nil && env.SessionModel != nil {
		b.WriteString("  /model [id]      — show or set the default model for this session (Ollama / openai_compatible)\n")
	}
	b.WriteString("  /theme [preset]  — show or set TUI ui_appearance (restart TUI to apply)\n")
	b.WriteString("  /workers — list workers; /focus or /in <prefix> — jump into worker; /back or /detach — return to coordinator\n")
	b.WriteString("  /plan path|init|save|template — default plan path, create from template, save last message, or print template\n")
	b.WriteString("  /apply-plan [--preview] [path] — preview plan on disk, or execute (switch to general-purpose, stream one turn)\n")
	b.WriteString("  /btw <text>      — side question: one user message with a brief-aside preamble (sent to the model)\n")
	b.WriteString("  Ctrl+C           — exit (session is saved on shutdown)\n")
	b.WriteString("\nPrefix input (before model; same tools and permissions; single line each; see docs/goclaw/prefix-input-modes.md):\n")
	b.WriteString("  !<command>       — bash tool\n")
	b.WriteString("  @<path>          — read_file in the workspace\n")
	b.WriteString("  &<task>          — spawn_agent (general-purpose; requires spawn_agent on the active profile)\n")
	b.WriteString("\nRestart CLI flags: --session <id>  --list-sessions  --no-tools  --readline  --profile <name>\n")
	b.WriteString("Env: default UI on a TTY is fullscreen TUI; GOCLAW_USE_TUI=0 or GOCLAW_USE_READLINE=1 uses line readline.\n")
	b.WriteString("Env: GOCLAW_TUI_MOUSE_SCROLL=1 enables mouse wheel on the TUI transcript (default off; see tui_mouse_scroll in settings).\n")
	b.WriteString("Env: GOCLAW_AGENT_PROFILE overrides agent_profile from settings (e.g. general-purpose).\n")
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
