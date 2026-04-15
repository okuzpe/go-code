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

// applyPlanExecute switches the orchestrator to general-purpose and returns the handoff user message
// for the model (same as a non-preview /apply-plan). pathTail is optional relative plan path (see planfile.ResolvePlanArg).
func applyPlanExecute(env SlashEnv, orch *orchestrator.Orchestrator, wd, pathTail string, hintsOut *UIHints) (notice string, modelSubmit string, err error) {
	if orch == nil {
		return "", "", fmt.Errorf("requires a running agent")
	}
	if strings.TrimSpace(wd) == "" {
		return "", "", fmt.Errorf("workspace directory not set")
	}
	if env.Profs == nil {
		return "", "", fmt.Errorf("profile map not configured")
	}
	gp, ok := env.Profs["general-purpose"]
	if !ok {
		return "", "", fmt.Errorf("general-purpose profile missing")
	}
	p := planfile.ResolvePlanArg(wd, pathTail)
	body, rerr := planfile.Read(p)
	if rerr != nil {
		return "", "", rerr
	}
	orch.SetProfile(gp)
	msg := planfile.HandoffUserMessage(p, body)
	notice = fmt.Sprintf("switched to profile general-purpose; executing plan: %s", p)
	sub := ""
	if env.ChatSubtitle != nil {
		sub = env.ChatSubtitle()
	}
	setWelcomeHints(hintsOut, orch, sub)
	setFooterHint(hintsOut, "")
	return notice, msg, nil
}
