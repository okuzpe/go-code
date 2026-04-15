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

// savePlanFromLastAssistant writes the latest non-empty assistant message to the default workspace plan file.
func savePlanFromLastAssistant(wd string, sess *session.Session) (planPath string, err error) {
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
	if mkErr := os.MkdirAll(filepath.Join(wd, planfile.Subdir), 0o700); mkErr != nil {
		return "", fmt.Errorf("mkdir: %w", mkErr)
	}
	planPath = planfile.Path(wd)
	if writeErr := os.WriteFile(planPath, []byte(lastText+"\n"), 0o600); writeErr != nil {
		return "", fmt.Errorf("write: %w", writeErr)
	}
	return planPath, nil
}

// ApplyPlanOptions configures /apply-plan and /plan run execution.
type ApplyPlanOptions struct {
	// Hub when true selects coordinator profile for execution (also combined with settings plan_apply_use_coordinator).
	Hub bool
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
		notice = fmt.Sprintf("switched to profile general-purpose; executing plan: %s", p)
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
	out += "\n\n" + planfile.ApprovalStatus(workdir, p, body)
	steps := planfile.ParseImplementationSteps(body)
	if len(steps) > 0 {
		out += fmt.Sprintf("\n\nParsed ## Steps (%d):\n", len(steps))
		for i, s := range steps {
			out += fmt.Sprintf("  %d. %s\n", i+1, s)
		}
	} else {
		out += "\n\nParsed ## Steps: (none — add a \"## Steps\" section with numbered lines for clearer orchestration.)"
	}
	return out, nil
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
		return fmt.Sprintf("File: %s\nNo numbered/bullet steps found under a \"## Steps\" heading.", display), nil
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("File: %s\nParsed ## Steps (%d):\n", display, len(steps)))
	for i, s := range steps {
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s))
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}
