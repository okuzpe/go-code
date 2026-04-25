package slashcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
)

func handleSlashAudit(env SlashEnv, orch *orchestrator.Orchestrator, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireRunningAgent("audit", orch); err != nil {
		return true, "", false, "", err
	}
	if env.Profs == nil {
		return true, "", false, "", fmt.Errorf("/audit: profile map not configured")
	}
	gp, ok := env.Profs["general-purpose"]
	if !ok {
		return true, "", false, "", fmt.Errorf("/audit: build profile missing")
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
}

func handleSlashReview(ctx context.Context, env SlashEnv, orch *orchestrator.Orchestrator, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
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
}

func handleSlashResearch(orch *orchestrator.Orchestrator, fields []string) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireRunningAgent("research", orch); err != nil {
		return true, "", false, "", err
	}
	query := strings.TrimSpace(strings.Join(fields[1:], " "))
	if query == "" {
		return true, "", false, "", fmt.Errorf("usage: /research <query>\nexample: /research best practices for Go HTTP middleware")
	}
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
}
