package slashcmd

import (
	"fmt"
	"time"

	"github.com/okuzpe/goclaw/internal/session"
)

// UIHints carries optional TUI follow-up actions after a slash command. Readline callers ignore it.
type UIHints struct {
	RefreshWelcome  bool
	WelcomeProfile  string
	WelcomeSubtitle string
	// ReloadTranscript when non-nil means the in-memory session was replaced; the TUI should rebuild the transcript view.
	ReloadTranscript *session.Session
}

func clearHints(h *UIHints) {
	if h != nil {
		*h = UIHints{}
	}
}

func setWelcomeHints(h *UIHints, profile, subtitle string) {
	if h == nil {
		return
	}
	h.RefreshWelcome = true
	h.WelcomeProfile = profile
	h.WelcomeSubtitle = subtitle
}

func setReloadTranscript(h *UIHints, s *session.Session) {
	if h == nil {
		return
	}
	h.ReloadTranscript = s
}

// formatSessionModAge renders modification time as a short relative label, falling back to RFC3339 for very old files.
func formatSessionModAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d / time.Minute)
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	}
	if d < 48*time.Hour {
		h := int(d / time.Hour)
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	}
	if d < 30*24*time.Hour {
		days := int(d / (24 * time.Hour))
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
	return t.UTC().Format(time.RFC3339)
}
