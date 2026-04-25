package slashcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
	"golang.org/x/term"
)

const maxSlashClipboardBytes = 768 * 1024

func handleSlashBTW(orch *orchestrator.Orchestrator, fields []string) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireRunningAgent("btw", orch); err != nil {
		return true, "", false, "", err
	}
	rest := strings.TrimSpace(strings.Join(fields[1:], " "))
	if rest == "" {
		return true, "", false, "", fmt.Errorf(`usage: /btw your question or note`)
	}
	return true, "", false, inputprefix.BtwRewrite(rest), nil
}

func handleSlashDoctor(ctx context.Context, env SlashEnv, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
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
}

func handleSlashClear(env SlashEnv, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
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
}

func handleSlashCopy(sess **session.Session) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireActiveSession("copy", sess); err != nil {
		return true, "", false, "", err
	}
	body := (*sess).PlainTranscript()
	if strings.TrimSpace(body) == "" {
		return true, "(nothing to copy — session transcript is empty)", false, "", nil
	}
	clipBody := body
	note := ""
	if len(clipBody) > maxSlashClipboardBytes {
		clipBody = clipBody[:maxSlashClipboardBytes]
		note = fmt.Sprintf(" (truncated to %d bytes for clipboard)", maxSlashClipboardBytes)
	}
	if err := clipboard.WriteAll(clipBody); err != nil {
		return true, "", false, "", fmt.Errorf("/copy: clipboard: %w — try /export path.txt", err)
	}
	return true, fmt.Sprintf("(copied %d bytes to clipboard)%s", len(clipBody), note), false, "", nil
}

func handleSlashExport(env SlashEnv, sess **session.Session, fields []string) (handled bool, out string, quit bool, modelSubmit string, err error) {
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
}

func handleSlashCompact(orch *orchestrator.Orchestrator, sess **session.Session) (handled bool, out string, quit bool, modelSubmit string, err error) {
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
}

func handleSlashContinue(orch *orchestrator.Orchestrator, sess **session.Session) (handled bool, out string, quit bool, modelSubmit string, err error) {
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
}

func handleSlashUndo(orch *orchestrator.Orchestrator, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireRunningAgent("undo", orch); err != nil {
		return true, "", false, "", err
	}
	msg, err := orch.UndoLastCheckpoint()
	if err != nil {
		return true, "", false, "", err
	}
	setFooterHint(hintsOut, "Last file change reverted from the session checkpoint.")
	return true, msg, false, "", nil
}

func handleSlashEdit(ctx context.Context, env SlashEnv, orch *orchestrator.Orchestrator) (handled bool, out string, quit bool, modelSubmit string, err error) {
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
}

func handleSlashInit(env SlashEnv, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	msg, ierr := handleSlashProjectInit(env)
	if ierr != nil {
		return true, "", false, "", ierr
	}
	setTUIDocOverlay(hintsOut, "Init")
	md := "## /init\n\n" + MarkdownFencedPlain(msg)
	return true, md, false, "", nil
}
