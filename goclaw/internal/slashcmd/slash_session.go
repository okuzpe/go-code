package slashcmd

import (
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/session"
)

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
		return nil, fmt.Errorf("ambiguous session prefix %q — use full id from /sessions", raw)
	}
	return store.Load(matches[0])
}
