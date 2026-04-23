package slashcmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
)

func handleSlashSessions(store *session.Store, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireSessionStore("sessions", store); err != nil {
		return true, "", false, "", err
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
		b.WriteString("` - ")
		b.WriteString(age)
		b.WriteString(" - (")
		b.WriteString(e.ModTime.UTC().Format(time.RFC3339))
		b.WriteString(")\n")
	}
	return true, strings.TrimSpace(b.String()), false, "", nil
}

func handleSlashResume(orch *orchestrator.Orchestrator, sess **session.Session, store *session.Store, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireSessionStore("resume", store); err != nil {
		return true, "", false, "", err
	}
	if err := requireRunningAgent("resume", orch); err != nil {
		return true, "", false, "", err
	}
	if sess == nil {
		return true, "", false, "", fmt.Errorf("/resume: session pointer missing")
	}
	if len(fields) < 2 {
		return true, "", false, "", slashNextStepError(`usage: /resume <session_id_or_prefix>
use /sessions to list saved ids; current session is auto-saved before switching`, "run /sessions, then /resume <id>")
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
}

func handleSlashNew(orch *orchestrator.Orchestrator, sess **session.Session, store *session.Store) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireRunningAgent("new", orch); err != nil {
		return true, "", false, "", err
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
}

func handleSlashSave(sess **session.Session, store *session.Store) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireSessionStore("save", store); err != nil {
		return true, "", false, "", err
	}
	if err := requireActiveSession("save", sess); err != nil {
		return true, "", false, "", err
	}
	if err := store.Save(*sess); err != nil {
		return true, "", false, "", fmt.Errorf("save session: %w", err)
	}
	return true, fmt.Sprintf("(session saved: %s, %d messages)", (*sess).ID, (*sess).Len()), false, "", nil
}

func handleSlashSession(sess **session.Session) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if sess == nil || *sess == nil {
		return true, "(no session)", false, "", nil
	}
	return true, fmt.Sprintf("session id: %s\nmessages: %d", (*sess).ID, (*sess).Len()), false, "", nil
}

// resolveSessionForResume loads a session by full id or unique id prefix.
func resolveSessionForResume(store *session.Store, raw string) (*session.Session, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty session id")
	}
	s, err := store.Load(raw)
	if err != nil {
		return nil, err
	}
	if s != nil {
		return s, nil
	}
	ids, err := store.ListIDs()
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, id := range ids {
		if strings.HasPrefix(id, raw) {
			matches = append(matches, id)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no saved session matches %q (use /sessions for ids)", raw)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous session prefix %q â€” use full id from /sessions", raw)
	}
	return store.Load(matches[0])
}
