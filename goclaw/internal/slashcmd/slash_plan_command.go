package slashcmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/planfile"
	"github.com/okuzpe/goclaw/internal/session"
)

func handleSlashPlan(sc SlashContext, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	wd := strings.TrimSpace(sc.Workdir)
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
		return handleSlashPlanReview(wd, fields, hintsOut)
	case "approve":
		return handleSlashPlanApprove(wd, fields, hintsOut)
	case "revoke":
		return handleSlashPlanRevoke(wd, hintsOut)
	case "steps":
		return handleSlashPlanSteps(wd, fields, hintsOut)
	case "path":
		return true, planfile.Path(wd), false, "", nil
	case "init":
		return handleSlashPlanInit(wd)
	case "new":
		return handleSlashPlanNew(wd, fields, hintsOut)
	case "save":
		return handleSlashPlanSave(sc.Sess, sc.SlashEnv, fields, hintsOut)
	case "run", "apply":
		return handleSlashPlanRun(sc.Orch, sc.Sess, sc.SlashEnv, fields, hintsOut)
	case "template":
		return true, planfile.Template(), false, "", nil
	default:
		return true, "", false, "", fmt.Errorf("unknown /plan %q — use path, init, new, save, run, apply, template, review, approve, revoke, or steps", fields[1])
	}
}

func handleSlashPlanReview(wd string, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	pathTail := strings.TrimSpace(strings.Join(fields[2:], " "))
	out, rerr := PlanReviewOutput(wd, pathTail)
	if rerr != nil {
		return true, "", false, "", fmt.Errorf("/plan review: %w", rerr)
	}
	setFooterHint(hintsOut, "Review done — /plan approve then /apply-plan (or /plan run) when ready.")
	setTUIDocOverlay(hintsOut, "Plan review")
	return true, out, false, "", nil
}

func handleSlashPlanApprove(wd string, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
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
}

func handleSlashPlanRevoke(wd string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := planfile.ClearApproval(wd); err != nil {
		return true, "", false, "", fmt.Errorf("/plan revoke: %w", err)
	}
	setFooterHint(hintsOut, "Plan approval cleared.")
	return true, "cleared plan approval (" + planfile.MetaPath(wd) + ")", false, "", nil
}

func handleSlashPlanSteps(wd string, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	pathTail := strings.TrimSpace(strings.Join(fields[2:], " "))
	out, serr := PlanStepsOutput(wd, pathTail)
	if serr != nil {
		return true, "", false, "", fmt.Errorf("/plan steps: %w", serr)
	}
	setTUIDocOverlay(hintsOut, "Plan steps")
	return true, out, false, "", nil
}

func handleSlashPlanInit(wd string) (handled bool, out string, quit bool, modelSubmit string, err error) {
	created, ierr := planfile.Init(wd)
	if ierr != nil {
		return true, "", false, "", ierr
	}
	if created {
		return true, fmt.Sprintf("created %s", planfile.Path(wd)), false, "", nil
	}
	return true, fmt.Sprintf("already exists: %s", planfile.Path(wd)), false, "", nil
}

func handleSlashPlanNew(wd string, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
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
}

func handleSlashPlanSave(sess **session.Session, env SlashEnv, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if sess == nil || *sess == nil {
		return true, "", false, "", fmt.Errorf("/plan save: no messages in current session")
	}
	pathTailSave := strings.TrimSpace(strings.Join(fields[2:], " "))
	planPath, sErr := savePlanFromLastAssistant(strings.TrimSpace(env.Workdir), *sess, pathTailSave)
	if sErr != nil {
		return true, "", false, "", fmt.Errorf("/plan save: %w", sErr)
	}
	h := "Plan saved — /plan run to save+execute, or /apply-plan --preview then /apply-plan."
	if planGateFrom(env).RequireApplyApproval {
		h = "Plan saved — run /plan approve before /apply-plan or /plan run (plan_require_apply_approval is on)."
	}
	setFooterHint(hintsOut, h)
	return true, fmt.Sprintf("plan saved to %s\nRun /plan review → /plan approve if required, then /plan run or /apply-plan.", planPath), false, "", nil
}

func handleSlashPlanRun(orch *orchestrator.Orchestrator, sess **session.Session, env SlashEnv, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	hub, allSteps, pathTailRun := parsePlanRunFields(fields)
	if err := requireRunningAgent("plan run", orch); err != nil {
		return true, "", false, "", err
	}
	if sess == nil || *sess == nil {
		return true, "", false, "", fmt.Errorf("/plan run: no messages in current session")
	}
	planPath, sErr := savePlanFromLastAssistant(strings.TrimSpace(env.Workdir), *sess, pathTailRun)
	if sErr != nil {
		return true, "", false, "", fmt.Errorf("/plan run: %w", sErr)
	}
	notice, modelSubmit, aErr := applyPlanExecute(env, orch, strings.TrimSpace(env.Workdir), pathTailRun, hintsOut, ApplyPlanOptions{Hub: hub, Steps: allSteps})
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
}

func handleSlashApplyPlan(input string, env SlashEnv, orch *orchestrator.Orchestrator, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireRunningAgent("apply-plan", orch); err != nil {
		return true, "", false, "", err
	}
	wd := strings.TrimSpace(env.Workdir)
	if wd == "" {
		return true, "", false, "", fmt.Errorf("/apply-plan: workspace directory not set")
	}
	fields := strings.Fields(strings.TrimSpace(input))
	if len(fields) == 0 {
		return true, "", false, "", fmt.Errorf("/apply-plan: invalid command")
	}
	rest := strings.TrimSpace(strings.TrimPrefix(input, fields[0]))
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
}
