package slashcmd

import (
	"context"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"strings"

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

func slashNextStepError(cause, nextStep string) error {
	cause = strings.TrimSpace(cause)
	nextStep = strings.TrimSpace(nextStep)
	if nextStep == "" {
		return errors.New(cause)
	}
	return fmt.Errorf("%s\nnext step: %s", cause, nextStep)
}

func isHelpAlias(input string) bool {
	low := strings.ToLower(strings.TrimSpace(input))
	return low == "help" || low == "?"
}

// normalizeCommandPrefix converts a leading ':' to '/' so ':cmd' is treated identically to '/cmd'.
func normalizeCommandPrefix(input string) string {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, ":") && !strings.HasPrefix(trimmed, "::") {
		return "/" + trimmed[1:]
	}
	return input
}

func parseSlashCommand(input string) (fields []string, cmd string, ok bool) {
	trimmed := strings.TrimSpace(normalizeCommandPrefix(input))
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return nil, "", false
	}
	fields = strings.Fields(trimmed)
	if len(fields) == 0 {
		return nil, "", false
	}
	return fields, strings.ToLower(strings.TrimPrefix(fields[0], "/")), true
}

func requireRunningAgent(command string, orch *orchestrator.Orchestrator) error {
	if orch != nil {
		return nil
	}
	return fmt.Errorf("/%s requires a running agent", strings.TrimPrefix(strings.TrimSpace(command), "/"))
}

func requireSessionStore(command string, store *session.Store) error {
	if store != nil {
		return nil
	}
	return fmt.Errorf("/%s: no session store configured", strings.TrimPrefix(strings.TrimSpace(command), "/"))
}

func requireActiveSession(command string, sess **session.Session) error {
	if sess != nil && *sess != nil {
		return nil
	}
	return fmt.Errorf("/%s: no active session", strings.TrimPrefix(strings.TrimSpace(command), "/"))
}

func requireFocusRouter(command string, env SlashEnv) error {
	if env.Focus != nil {
		return nil
	}
	return fmt.Errorf("focus routing not enabled (/%s)", strings.TrimPrefix(strings.TrimSpace(command), "/"))
}

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

	s := strings.TrimSpace(normalizeCommandPrefix(input))
	if s == "" {
		return false, "", false, "", nil
	}
	if isHelpAlias(s) {
		return true, replHelpText(env, sess, orch), false, "", nil
	}
	fields, cmd, ok := parseSlashCommand(s)
	if !ok {
		return false, "", false, "", nil
	}

	switch cmd {
	case "help":
		return true, replHelpText(env, sess, orch), false, "", nil

	case "btw":
		if err := requireRunningAgent("btw", orch); err != nil {
			return true, "", false, "", err
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
		return handleSlashModel(env, orch, fields, hintsOut)

	case "tools":
		return handleSlashTools(env, fields)

	case "quit", "exit":
		return true, "bye", true, "", ErrReplQuit

	case "sessions":
		return handleSlashSessions(store, hintsOut)

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
		return handleSlashResume(orch, sess, store, fields, hintsOut)

	case "new":
		return handleSlashNew(orch, sess, store)

	case "save":
		return handleSlashSave(sess, store)

	case "session":
		return handleSlashSession(sess)

	case "theme":
		return handleSlashTheme(env, fields)

	case "capabilities":
		setTUIDocOverlay(hintsOut, "Capabilities")
		return true, UserCapabilitiesGuide(), false, "", nil

	case "copy":
		const maxClipBytes = 768 * 1024
		if err := requireActiveSession("copy", sess); err != nil {
			return true, "", false, "", err
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
		if err := requireRunningAgent("compact", orch); err != nil {
			return true, "", false, "", err
		}
		if sess == nil || *sess == nil {
			return true, "", false, "", fmt.Errorf("/compact: no active session")
		}
		before := (*sess).Len()
		tokensBefore := orchestrator.SessionMessagesTokenEstimate((*sess).Messages)
		orch.ForceCompact()
		after := (*sess).Len()
		tokensAfter := orchestrator.SessionMessagesTokenEstimate((*sess).Messages)
		return true, fmt.Sprintf("(compaction applied: %d → %d messages, ~%d → ~%d tokens; tail preserved)", before, after, tokensBefore, tokensAfter), false, "", nil

	case "continue":
		if err := requireRunningAgent("continue", orch); err != nil {
			return true, "", false, "", err
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
		continueSubmit := orchestrator.ContinueFollowUpPrompt((*sess).Messages)
		return true, fmt.Sprintf("(follow-up for: %s)\n", snippet), false, continueSubmit, nil

	case "undo":
		if err := requireRunningAgent("undo", orch); err != nil {
			return true, "", false, "", err
		}
		msg, err := orch.UndoLastCheckpoint()
		if err != nil {
			return true, "", false, "", err
		}
		setFooterHint(hintsOut, "Last file change reverted from the session checkpoint.")
		return true, msg, false, "", nil

	case "edit":
		if err := requireRunningAgent("edit", orch); err != nil {
			return true, "", false, "", err
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
		return handleSlashProfile(env, orch, fields, hintsOut)

	case "mode":
		return handleSlashMode(env, orch, fields, hintsOut)

	case "agents":
		return handleSlashAgents(env, orch, fields, hintsOut)

	case "allow-writes":
		return handleSlashAllowWrites(orch, hintsOut)

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
			hub, allSteps, pathTailRun := parsePlanRunFields(fields)
			if err := requireRunningAgent("plan run", orch); err != nil {
				return true, "", false, "", err
			}
			if sc.Sess == nil || *sc.Sess == nil {
				return true, "", false, "", fmt.Errorf("/plan run: no messages in current session")
			}
			planPath, sErr := savePlanFromLastAssistant(wd, *sc.Sess, pathTailRun)
			if sErr != nil {
				return true, "", false, "", fmt.Errorf("/plan run: %w", sErr)
			}
			notice, modelSubmit, aErr := applyPlanExecute(env, orch, wd, pathTailRun, hintsOut, ApplyPlanOptions{Hub: hub, Steps: allSteps})
			if aErr != nil {
				return true, "", false, "", fmt.Errorf("/plan run: %w", aErr)
			}
			if allSteps {
				setFooterHint(hintsOut, "Plan saved — multi-step execution queued (one turn per ## Steps line).")
			} else {
				setFooterHint(hintsOut, "Plan saved and execution started — one model turn; follow up if the plan is large.")
			}
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
		if err := requireFocusRouter("detach", env); err != nil {
			return true, "", false, "", err
		}
		env.Focus.Detach()
		return true, "focus: coordinator (parent session)", false, "", nil

	case "focus", "in":
		if err := requireFocusRouter("focus", env); err != nil {
			return true, "", false, "", err
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
			return true, "", false, "", slashNextStepError(fmt.Sprintf("no unique interactive worker matches prefix %q", prefix), "run /workers and choose a longer prefix")
		}
		env.Focus.FocusTaskID(full)
		out := fmt.Sprintf("focus: worker %s (input goes here until /back or /detach)", full)
		if snap, ok := coordinator.SnapshotInteractiveWorker(full); ok && strings.TrimSpace(snap) != "" {
			out = "[worker history]\n" + snap + "\n" + out
		}
		return true, out, false, "", nil

	case "apply-plan":
		if err := requireRunningAgent("apply-plan", orch); err != nil {
			return true, "", false, "", err
		}
		wd := strings.TrimSpace(env.Workdir)
		if wd == "" {
			return true, "", false, "", fmt.Errorf("/apply-plan: workspace directory not set")
		}
		rest := strings.TrimSpace(strings.TrimPrefix(s, fields[0]))
		pathTail, preview, hub, allSteps := parseApplyPlanRest(rest)
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
		notice, msg, err := applyPlanExecute(env, orch, wd, pathTail, hintsOut, ApplyPlanOptions{Hub: hub, Steps: allSteps})
		if err != nil {
			return true, "", false, "", fmt.Errorf("/apply-plan: %w", err)
		}
		return true, notice, false, msg, nil

	case "audit":
		if err := requireRunningAgent("audit", orch); err != nil {
			return true, "", false, "", err
		}
		if env.Profs == nil {
			return true, "", false, "", fmt.Errorf("/audit: profile map not configured")
		}
		gp, ok := env.Profs["general-purpose"]
		if !ok {
			return true, "", false, "", fmt.Errorf("/audit: build profile missing (internal general-purpose not loaded)")
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
		notice := fmt.Sprintf("switched to mode build; starting project audit: %s", target)
		sub := ""
		if env.ChatSubtitle != nil {
			sub = env.ChatSubtitle()
		}
		setWelcomeHints(hintsOut, orch, sub)
		return true, notice, false, auditMsg, nil

	case "review":
		if err := requireRunningAgent("review", orch); err != nil {
			return true, "", false, "", err
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

	case "research":
		if err := requireRunningAgent("research", orch); err != nil {
			return true, "", false, "", err
		}
		query := strings.TrimSpace(strings.Join(fields[1:], " "))
		if query == "" {
			return true, "", false, "", fmt.Errorf("usage: /research <query>\nexample: /research best practices for Go HTTP middleware")
		}
		// Build a self-contained prompt: web search → synthesize → save plan.
		slug := researchSlug(query)
		planPath := ".goclaw/plans/research-" + slug + ".md"
		researchMsg := fmt.Sprintf(
			"## Research task\n\n"+
				"Goal: research the following topic and produce an actionable implementation plan.\n\n"+
				"**Query:** %s\n\n"+
				"## Instructions\n\n"+
				"1. Use web_search with 2–3 targeted queries to find recent best practices, examples, and relevant documentation.\n"+
				"2. Synthesize the findings into a concrete, numbered step-by-step implementation plan tailored to this workspace.\n"+
				"3. Write the plan to `%s` using write_file.\n"+
				"4. End with a one-paragraph summary of what you found and what the plan covers.\n\n"+
				"Do not ask for clarification — make a reasonable interpretation and start searching immediately.",
			query, planPath,
		)
		return true, fmt.Sprintf("researching: %q — plan will be saved to %s", query, planPath), false, researchMsg, nil

	default:
		return true, "", false, "", slashNextStepError(fmt.Sprintf("unknown command /%s", cmd), "run /help for the full command list")
	}
}
