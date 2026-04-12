package slashcmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
)

// UIHints carries optional TUI follow-up actions after a slash command. Readline callers ignore it.
type UIHints struct {
	RefreshWelcome  bool
	WelcomeProfile  string
	WelcomeSubtitle string
	// WelcomeFileWriteToolsHidden when non-nil updates welcome panel hints (matches agents.Profile.AllowsWorkspaceFileWrites).
	WelcomeFileWriteToolsHidden *bool
	// WelcomeHubDelegatesCoding when non-nil updates welcome panel hints (spawn_agent delegation).
	WelcomeHubDelegatesCoding *bool
	// FooterHint when non-nil updates the TUI footer line (e.g. after /plan save). Empty string clears. Nil leaves unchanged.
	FooterHint *string
	// ReloadTranscript when non-nil means the in-memory session was replaced; the TUI should rebuild the transcript view.
	ReloadTranscript *session.Session
}

func clearHints(h *UIHints) {
	if h != nil {
		*h = UIHints{}
	}
}

// setWelcomeHints refreshes the TUI welcome strip from the orchestrator's active profile
// (so /profile updates file-tool and hub hints, not only the profile name).
func setWelcomeHints(h *UIHints, orch *orchestrator.Orchestrator, subtitle string) {
	if h == nil || orch == nil {
		return
	}
	p := orch.ActiveProfile()
	h.RefreshWelcome = true
	h.WelcomeProfile = p.Name
	h.WelcomeSubtitle = subtitle
	hide := !p.AllowsWorkspaceFileWrites()
	h.WelcomeFileWriteToolsHidden = &hide
	hub := p.AllowsSpawnAgentDelegation()
	h.WelcomeHubDelegatesCoding = &hub
}

func setFooterHint(h *UIHints, line string) {
	if h == nil {
		return
	}
	s := strings.TrimSpace(line)
	h.FooterHint = &s
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
