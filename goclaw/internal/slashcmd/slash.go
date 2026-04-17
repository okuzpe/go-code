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
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/planfile"
	"github.com/okuzpe/goclaw/internal/session"

	"golang.org/x/term"
)

// PlanGateConfig mirrors plan workflow flags from runtime settings (see config.Config).
type PlanGateConfig struct {
	RequireApplyApproval bool
	ApplyUseCoordinator  bool
	AgentPickerHide      []string
}

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
	// ToolLog optional; when set, /tools prints formatted tool history as plain text. Fullscreen TUI
	// normally uses Ctrl+T instead; HandleSlash returns a hint when ToolLog is nil.
	ToolLog func(n int) string
	// PlanGate optional; supplies plan approval / hub / agent-picker flags from settings. Nil → all off.
	PlanGate func() PlanGateConfig
	// FullscreenTUI is true when slash commands run inside the Bubble Tea chat (cmd/goclaw fullscreen runner).
	// Used to avoid raw stdout control sequences that would fight the TUI (e.g. /clear).
	FullscreenTUI bool
	// OnProfileChange optional; invoked after a successful /profile or /agents hot-switch so the host can
	// mirror agents.Profile (e.g. ChatRuntime.Profile for doctor and auto-profile helpers).
	OnProfileChange func(agents.Profile)
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
// hintsOut when non-nil receives TUI refresh hints (welcome bar, transcript rebuild); pass nil for non-TUI callers.
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
		setTUIDocOverlay(hintsOut, "Doctor")
		doc := "## Doctor\n\n" + MarkdownFencedPlain(out)
		return true, doc, false, "", nil

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
			_, _ = fmt.Sscan(fields[1], &n)
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
		setTUIDocOverlay(hintsOut, "Sessions")
		var b strings.Builder
		b.WriteString("## Saved sessions\n\n")
		for _, e := range entries {
			age := formatSessionModAge(e.ModTime)
			b.WriteString("- `")
			b.WriteString(e.ID)
			b.WriteString("` — ")
			b.WriteString(age)
			b.WriteString(" — (")
			b.WriteString(e.ModTime.UTC().Format(time.RFC3339))
			b.WriteString(")\n")
		}
		return true, strings.TrimSpace(b.String()), false, "", nil

	case "clear":
		if env.FullscreenTUI {
			if hintsOut != nil {
				hintsOut.TUIClearTranscript = true
			}
			return true, "(transcript cleared)", false, "", nil
		}
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
		setTUIDocOverlay(hintsOut, "Capabilities")
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

	case "continue":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/continue requires a running agent")
		}
		if sess == nil || *sess == nil {
			return true, "", false, "", fmt.Errorf("/continue: no active session")
		}
		prev := orchestrator.LastUserNaturalText((*sess).Messages)
		if prev == "" {
			return true, "", false, "", fmt.Errorf(`/continue: no prior user message in this session — send a request first`)
		}
		r := []rune(strings.TrimSpace(prev))
		snippet := string(r)
		if len(r) > 48 {
			snippet = string(r[:48]) + "…"
		}
		const continueSubmit = `Continue working on the prior request: use tools to complete pending edits (write_file, edit_file, patch) or bash/script to verify, or state clearly what blocks you.`
		return true, fmt.Sprintf("(follow-up for: %s)\n", snippet), false, continueSubmit, nil

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
		setTUIDocOverlay(hintsOut, "Init")
		md := "## /init\n\n" + MarkdownFencedPlain(msg)
		return true, md, false, "", nil

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
			setTUIDocOverlay(hintsOut, "Memory")
			var b strings.Builder
			b.WriteString("## Memory entries\n\n")
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
			setTUIDocOverlay(hintsOut, "Agents")
			return true, formatAgentsList(profs, orch.ProfileName(), env), false, "", nil
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

	case "allow-writes":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/allow-writes requires a running agent")
		}
		for _, toolName := range []string{"write_file", "edit_file", "patch"} {
			orch.SetToolPermission(toolName, permissions.ModeAllow)
		}
		hint := "workspace write tools auto-approved for this session (write_file, edit_file, patch)"
		setFooterHint(hintsOut, hint)
		return true, hint, false, "", nil

	case "plan":
		wd := strings.TrimSpace(env.Workdir)
		if wd == "" {
			return true, "", false, "", fmt.Errorf("/plan: workspace directory not set")
		}
		if len(fields) < 2 {
			return true, "", false, "", fmt.Errorf(`usage: /plan path | init | new | save | run | template | review | approve | revoke | steps
path     — show default plan file path
init     — create .goclaw/plan.md from template if missing
new NAME — create .goclaw/plans/<name>.md from mini template (name is sanitized to a filename stem)
save     — save last assistant message to plan file (optional path; default .goclaw/plan.md)
run      — save then execute one turn (optional --hub; optional plan path; alias: apply)
template — print the default plan template to the terminal
review   — show plan excerpt, approval status, and parsed ## Steps (optional path)
approve  — record approval for the current plan file hash (optional path)
revoke   — clear recorded approval (plan.meta.json)
steps    — list parsed ## Steps only (optional path)`)
		}
		sub := strings.ToLower(fields[1])
		switch sub {
		case "review":
			pathTail := strings.TrimSpace(strings.Join(fields[2:], " "))
			out, rerr := PlanReviewOutput(wd, pathTail)
			if rerr != nil {
				return true, "", false, "", fmt.Errorf("/plan review: %w", rerr)
			}
			setFooterHint(hintsOut, "Review done — /plan approve then /apply-plan (or /plan run) when ready.")
			setTUIDocOverlay(hintsOut, "Plan review")
			return true, out, false, "", nil
		case "approve":
			pathTail := strings.TrimSpace(strings.Join(fields[2:], " "))
			p := planfile.ResolvePlanArg(wd, pathTail)
			body, rerr := planfile.Read(p)
			if rerr != nil {
				return true, "", false, "", fmt.Errorf("/plan approve: %w", rerr)
			}
			if err := planfile.WriteApproval(wd, p, body); err != nil {
				return true, "", false, "", fmt.Errorf("/plan approve: %w", err)
			}
			setFooterHint(hintsOut, "Plan approved on disk — you can run /apply-plan or /plan run.")
			return true, fmt.Sprintf("approval saved for %s (content hash recorded in %s)", p, planfile.MetaPath(wd)), false, "", nil
		case "revoke":
			if err := planfile.ClearApproval(wd); err != nil {
				return true, "", false, "", fmt.Errorf("/plan revoke: %w", err)
			}
			setFooterHint(hintsOut, "Plan approval cleared.")
			return true, "cleared plan approval (" + planfile.MetaPath(wd) + ")", false, "", nil
		case "steps":
			pathTail := strings.TrimSpace(strings.Join(fields[2:], " "))
			out, serr := PlanStepsOutput(wd, pathTail)
			if serr != nil {
				return true, "", false, "", fmt.Errorf("/plan steps: %w", serr)
			}
			setTUIDocOverlay(hintsOut, "Plan steps")
			return true, out, false, "", nil
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
		case "new":
			if len(fields) < 3 {
				return true, "", false, "", fmt.Errorf(`usage: /plan new <name> — creates .goclaw/plans/<name>.md (sanitized) if it does not exist`)
			}
			raw := strings.TrimSpace(strings.Join(fields[2:], " "))
			p, created, ierr := planfile.InitMiniPlan(wd, raw)
			if ierr != nil {
				return true, "", false, "", fmt.Errorf("/plan new: %w", ierr)
			}
			if created {
				rel, _ := filepath.Rel(wd, p)
				setFooterHint(hintsOut, "Mini plan created — edit the file, then /apply-plan "+filepath.ToSlash(rel))
				return true, fmt.Sprintf("created %s\nOpen or edit this file, then /plan review (optional path) → /apply-plan (same path).", p), false, "", nil
			}
			setFooterHint(hintsOut, "Mini plan file already exists — edit it or pick another name.")
			return true, fmt.Sprintf("already exists: %s", p), false, "", nil
		case "save":
			if sc.Sess == nil || *sc.Sess == nil {
				return true, "", false, "", fmt.Errorf("/plan save: no messages in current session")
			}
			pathTailSave := strings.TrimSpace(strings.Join(fields[2:], " "))
			planPath, sErr := savePlanFromLastAssistant(wd, *sc.Sess, pathTailSave)
			if sErr != nil {
				return true, "", false, "", fmt.Errorf("/plan save: %w", sErr)
			}
			h := "Plan saved — /plan run to save+execute, or /apply-plan --preview then /apply-plan."
			if planGateFrom(env).RequireApplyApproval {
				h = "Plan saved — run /plan approve before /apply-plan or /plan run (plan_require_apply_approval is on)."
			}
			setFooterHint(hintsOut, h)
			return true, fmt.Sprintf("plan saved to %s\nRun /plan review → /plan approve if required, then /plan run or /apply-plan.", planPath), false, "", nil
		case "run", "apply":
			hub, pathTailRun := parsePlanRunFields(fields)
			if orch == nil {
				return true, "", false, "", fmt.Errorf("/plan run requires a running agent")
			}
			if sc.Sess == nil || *sc.Sess == nil {
				return true, "", false, "", fmt.Errorf("/plan run: no messages in current session")
			}
			planPath, sErr := savePlanFromLastAssistant(wd, *sc.Sess, pathTailRun)
			if sErr != nil {
				return true, "", false, "", fmt.Errorf("/plan run: %w", sErr)
			}
			notice, modelSubmit, aErr := applyPlanExecute(env, orch, wd, pathTailRun, hintsOut, ApplyPlanOptions{Hub: hub})
			if aErr != nil {
				return true, "", false, "", fmt.Errorf("/plan run: %w", aErr)
			}
			setFooterHint(hintsOut, "Plan saved and execution started — one model turn; follow up if the plan is large.")
			combined := fmt.Sprintf("plan saved to %s\n%s", planPath, notice)
			return true, combined, false, modelSubmit, nil
		case "template":
			return true, planfile.Template(), false, "", nil
		default:
			return true, "", false, "", fmt.Errorf("unknown /plan %q — use path, init, new, save, run, apply, template, review, approve, revoke, or steps", fields[1])
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
		rest := strings.TrimSpace(strings.TrimPrefix(s, fields[0]))
		pathTail, preview, hub := parseApplyPlanRest(rest)
		p := planfile.ResolvePlanArg(wd, pathTail)
		body, rerr := planfile.Read(p)
		if rerr != nil {
			return true, "", false, "", rerr
		}
		if preview {
			out := formatPlanPreviewOutput(p, body)
			setFooterHint(hintsOut, "Review complete — run /apply-plan to execute (or /apply-plan --preview again).")
			setTUIDocOverlay(hintsOut, "Plan preview")
			return true, out, false, "", nil
		}
		notice, msg, err := applyPlanExecute(env, orch, wd, pathTail, hintsOut, ApplyPlanOptions{Hub: hub})
		if err != nil {
			return true, "", false, "", fmt.Errorf("/apply-plan: %w", err)
		}
		return true, notice, false, msg, nil

	case "audit":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/audit requires a running agent")
		}
		if env.Profs == nil {
			return true, "", false, "", fmt.Errorf("/audit: profile map not configured")
		}
		gp, ok := env.Profs["general-purpose"]
		if !ok {
			return true, "", false, "", fmt.Errorf("/audit: general-purpose profile missing")
		}
		target := strings.TrimSpace(strings.Join(fields[1:], " "))
		if target == "" {
			target = strings.TrimSpace(env.Workdir)
		}
		orch.SetProfile(gp)
		for _, toolName := range []string{"read_file", "glob", "grep", "todo_write"} {
			orch.SetToolPermission(toolName, permissions.ModeAllow)
		}
		auditMsg := fmt.Sprintf(
			"Audit the project at %q. "+
				"Step 1: run glob to map the project tree. "+
				"Step 2: read_file on key source files to understand the codebase. "+
				"Step 3: identify concrete gaps (missing error handling, missing tests, dead code, security issues, type safety, documentation). "+
				"Step 4: for each gap found, apply the fix immediately with edit_file or write_file — do NOT produce a suggestion list. "+
				"Step 5: run bash or script to verify the build or tests pass. "+
				"Step 6: report one short paragraph summarizing what was found and what was changed.",
			target,
		)
		notice := fmt.Sprintf("switched to profile general-purpose; starting project audit: %s", target)
		sub := ""
		if env.ChatSubtitle != nil {
			sub = env.ChatSubtitle()
		}
		setWelcomeHints(hintsOut, orch, sub)
		return true, notice, false, auditMsg, nil

	case "review":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/review requires a running agent")
		}
		if env.Profs == nil {
			return true, "", false, "", fmt.Errorf("/review: profile map not configured")
		}
		cp, ok := env.Profs["code-review"]
		if !ok {
			return true, "", false, "", fmt.Errorf("/review: code-review profile missing")
		}
		wd := strings.TrimSpace(env.Workdir)
		if wd == "" {
			return true, "", false, "", fmt.Errorf("/review: workspace directory not set")
		}
		argv, aerr := reviewGitDiffArgv(fields)
		if aerr != nil {
			return true, "", false, "", aerr
		}
		cmdLine, diffBody, rerr := runReviewGitDiff(ctx, wd, argv)
		if rerr != nil {
			return true, "", false, "", fmt.Errorf("/review: %w", rerr)
		}
		orch.SetProfile(cp)
		for _, toolName := range []string{"read_file", "glob", "grep", "todo_write"} {
			orch.SetToolPermission(toolName, permissions.ModeAllow)
		}
		reviewMsg := fmt.Sprintf(
			"## Code review request\n\n"+
				"Workspace: %q\n"+
				"Git: %s\n\n"+
				"Review the following unified diff. Do not use write_file, edit_file, or patch. "+
				"Produce severity-tagged findings as described in your profile.\n\n"+
				"## Diff\n\n%s\n",
			wd, cmdLine, diffBody,
		)
		notice := fmt.Sprintf("switched to profile code-review; injected diff from: %s", cmdLine)
		sub := ""
		if env.ChatSubtitle != nil {
			sub = env.ChatSubtitle()
		}
		setWelcomeHints(hintsOut, orch, sub)
		return true, notice, false, reviewMsg, nil

	default:
		return true, "", false, "", fmt.Errorf("unknown command /%s — try /help", cmd)
	}
}
