package slashcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/planfile"
	"github.com/okuzpe/goclaw/internal/session"
)

// savePlanFromLastAssistant writes the latest non-empty assistant message to a plan file.
// destArg selects the path: empty → default .goclaw/plan.md; otherwise ResolvePlanArg(wd, destArg)
// (e.g. .goclaw/plans/refactor-auth.md). The resolved path must stay under the workspace root.
func savePlanFromLastAssistant(wd string, sess *session.Session, destArg string) (planPath string, err error) {
	if strings.TrimSpace(wd) == "" {
		return "", fmt.Errorf("workspace directory not set")
	}
	if sess == nil || len(sess.Messages) == 0 {
		return "", fmt.Errorf("no messages in current session")
	}
	lastText := ""
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		m := sess.Messages[i]
		if m.Role == "assistant" && strings.TrimSpace(m.Content) != "" {
			lastText = m.Content
			break
		}
	}
	if lastText == "" {
		return "", fmt.Errorf("no assistant message in session to save")
	}
	planPath = planfile.ResolvePlanArg(wd, strings.TrimSpace(destArg))
	if err := planfile.EnsurePlanPathUnderWorkspace(wd, planPath); err != nil {
		return "", err
	}
	parent := filepath.Dir(planPath)
	if mkErr := os.MkdirAll(parent, 0o700); mkErr != nil {
		return "", fmt.Errorf("mkdir: %w", mkErr)
	}
	if writeErr := os.WriteFile(planPath, []byte(lastText+"\n"), 0o600); writeErr != nil {
		return "", fmt.Errorf("write: %w", writeErr)
	}
	return planPath, nil
}

// ApplyPlanOptions configures /apply-plan and /plan run execution.
type ApplyPlanOptions struct {
	// Hub when true selects coordinator profile for execution (also combined with settings plan_apply_use_coordinator).
	Hub bool
	// Steps when true and the plan has a parsed ## Steps section, queues one model turn per step via UIHints.ModelSubmitQueue.
	Steps bool
}

func planGateFrom(env SlashEnv) PlanGateConfig {
	if env.PlanGate == nil {
		return PlanGateConfig{}
	}
	return env.PlanGate()
}

// applyPlanExecute switches profile and returns the handoff user message for the model.
func applyPlanExecute(env SlashEnv, orch *orchestrator.Orchestrator, wd, pathTail string, hintsOut *UIHints, opt ApplyPlanOptions) (notice string, modelSubmit string, err error) {
	if orch == nil {
		return "", "", fmt.Errorf("requires a running agent")
	}
	if strings.TrimSpace(wd) == "" {
		return "", "", fmt.Errorf("workspace directory not set")
	}
	if env.Profs == nil {
		return "", "", fmt.Errorf("profile map not configured")
	}
	gate := planGateFrom(env)
	p := planfile.ResolvePlanArg(wd, pathTail)
	body, rerr := planfile.Read(p)
	if rerr != nil {
		return "", "", rerr
	}
	if gate.RequireApplyApproval {
		if err := planfile.VerifyApproval(wd, p, body); err != nil {
			return "", "", err
		}
	}
	useCoord := opt.Hub || gate.ApplyUseCoordinator
	steps := planfile.ParseImplementationSteps(body)
	hOpts := planfile.HandoffOptions{UseCoordinator: useCoord, ParsedSteps: steps}

	if opt.Steps && len(steps) > 0 {
		msgs := planfile.StepExecutionUserMessages(p, body, steps, hOpts)
		if len(msgs) == 0 {
			return "", "", fmt.Errorf("plan steps execution produced no messages")
		}
		setModelSubmitQueue(hintsOut, msgs[1:])
		notice := ""
		if useCoord {
			coord, ok := env.Profs["coordinator"]
			if !ok {
				return "", "", fmt.Errorf("coordinator profile missing (required for hub plan execution)")
			}
			orch.SetProfile(coord)
			notice = fmt.Sprintf("switched to profile coordinator; executing plan steps (%d turns): %s", len(msgs), p)
		} else {
			gp, ok := env.Profs["general-purpose"]
			if !ok {
				return "", "", fmt.Errorf("general-purpose profile missing")
			}
			orch.SetProfile(gp)
			notice = fmt.Sprintf("switched to mode build; executing plan steps (%d turns): %s", len(msgs), p)
		}
		sub := ""
		if env.ChatSubtitle != nil {
			sub = env.ChatSubtitle()
		}
		setWelcomeHints(hintsOut, orch, sub)
		setFooterHint(hintsOut, "")
		return notice, msgs[0], nil
	}

	msg := planfile.HandoffUserMessageWithOptions(p, body, hOpts)

	if useCoord {
		coord, ok := env.Profs["coordinator"]
		if !ok {
			return "", "", fmt.Errorf("coordinator profile missing (required for hub plan execution)")
		}
		orch.SetProfile(coord)
		notice = fmt.Sprintf("switched to profile coordinator; executing plan: %s", p)
	} else {
		gp, ok := env.Profs["general-purpose"]
		if !ok {
			return "", "", fmt.Errorf("general-purpose profile missing")
		}
		orch.SetProfile(gp)
		notice = fmt.Sprintf("switched to mode build; executing plan: %s", p)
	}
	sub := ""
	if env.ChatSubtitle != nil {
		sub = env.ChatSubtitle()
	}
	setWelcomeHints(hintsOut, orch, sub)
	setFooterHint(hintsOut, "")
	return notice, msg, nil
}

// PlanReviewOutput builds review text: preview excerpt, approval line, and parsed steps.
func PlanReviewOutput(workdir, pathTail string) (string, error) {
	p := planfile.ResolvePlanArg(workdir, pathTail)
	body, err := planfile.Read(p)
	if err != nil {
		return "", err
	}
	out := formatPlanPreviewOutput(p, body)
	out += "\n\n## Approval\n\n" + planfile.ApprovalStatus(workdir, p, body)
	steps := planfile.ParseImplementationSteps(body)
	if len(steps) > 0 {
		out += fmt.Sprintf("\n\n## Parsed steps (%d)\n\n", len(steps))
		for i, s := range steps {
			out += fmt.Sprintf("%d. %s\n", i+1, s)
		}
	} else {
		out += "\n\n## Parsed steps\n\n_(none — add a \"## Steps\" section with numbered lines for clearer orchestration.)_"
	}
	return strings.TrimSpace(out), nil
}

// PlanStepsOutput lists parsed steps only (no full body).
func PlanStepsOutput(workdir, pathTail string) (string, error) {
	p := planfile.ResolvePlanArg(workdir, pathTail)
	body, err := planfile.Read(p)
	if err != nil {
		return "", err
	}
	steps := planfile.ParseImplementationSteps(body)
	display := filepath.ToSlash(p)
	if len(steps) == 0 {
		return fmt.Sprintf("## Plan steps\n\n- **File:** `%s`\n\nNo numbered/bullet steps found under a \"## Steps\" heading.", display), nil
	}
	var b strings.Builder
	b.WriteString("## Plan steps\n\n")
	b.WriteString(fmt.Sprintf("- **File:** `%s`\n\n", display))
	b.WriteString(fmt.Sprintf("### Parsed ## Steps (%d)\n\n", len(steps)))
	for i, s := range steps {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
	}
	return strings.TrimSpace(b.String()), nil
}
