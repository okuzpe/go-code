package chat

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/text"
	"github.com/okuzpe/goclaw/internal/ui/icons"
)

// agentLaneHint is a short lowercase gloss for the active profile (optional; omitted when there is no room).
func agentLaneHint(profileName string) string {
	switch strings.ToLower(strings.TrimSpace(profileName)) {
	case "coordinator":
		return "hub"
	case "plan":
		return "draft"
	case "explore":
		return "read-only"
	case "general-purpose":
		return "full tools"
	case "builder":
		return "forge"
	case "code-review":
		return "review"
	case "verification":
		return "checks"
	case "guide", "statusline":
		return "chat"
	default:
		return ""
	}
}

func deckBrackets(st icons.Set) (open, close string) {
	switch st {
	case icons.ASCII:
		return "<", ">"
	default:
		return "\u2039", "\u203a" // ‹ › — single-column, no Lip Gloss box
	}
}

func chordLabel(st icons.Set, termWidth int) string {
	switch st {
	case icons.ASCII:
		return "^+left/right"
	default:
		if termWidth < 52 {
			return "⇧^ ←→"
		}
		return "shift+ctrl ←→"
	}
}

func traceDots(th *Theme, width int) string {
	if width < 1 {
		return ""
	}
	return th.AgentDeckTrace.Render(strings.Repeat("·", width))
}

func buildAgentDeckCluster(th *Theme, st icons.Set, name, hint string, maxW int) string {
	if maxW < 8 {
		return th.SlashPickerName.Render(text.TruncateRunes(name, max(1, maxW/2)))
	}
	o, c := deckBrackets(st)
	try := func(prof, h string) string {
		s := th.AgentDeckBracket.Render(o) + th.SlashPickerName.Render(prof) + th.AgentDeckBracket.Render(c)
		if strings.TrimSpace(h) != "" {
			s += th.AgentDeckMicro.Render(" · "+h)
		}
		return s
	}
	prof := name
	h := hint
	for lipgloss.Width(try(prof, h)) > maxW && h != "" {
		h = ""
	}
	for lipgloss.Width(try(prof, h)) > maxW && utf8.RuneCountInString(prof) > 1 {
		prof = text.TruncateRunes(prof, utf8.RuneCountInString(prof)-1)
	}
	return try(prof, h)
}

// agentDeckRailView is a single-line profile rail (no multi-line Lip Gloss borders — they collide with the viewport).
func (m *Model) agentDeckRailView() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	w := m.width
	name := strings.TrimSpace(m.activeAgentProfile)
	if name == "" {
		return ""
	}

	st := th.Icons
	rail := th.AgentDeckRail.Render(st.AgentDeckRailGlyph())
	railW := lipgloss.Width(rail)

	chord := th.AgentDeckChord.Render(chordLabel(st, w))
	chordW := lipgloss.Width(chord)

	const minTrace = 2
	maxCluster := w - railW - 1 - chordW - minTrace
	if maxCluster < 6 {
		maxCluster = 6
	}

	cluster := buildAgentDeckCluster(th, st, name, agentLaneHint(name), maxCluster)
	clusterW := lipgloss.Width(cluster)

	traceW := w - railW - 1 - clusterW - chordW
	for traceW < minTrace && maxCluster > 4 {
		maxCluster--
		cluster = buildAgentDeckCluster(th, st, name, agentLaneHint(name), maxCluster)
		clusterW = lipgloss.Width(cluster)
		traceW = w - railW - 1 - clusterW - chordW
	}
	if traceW < 0 {
		traceW = 0
	}

	row := rail + " " + cluster + traceDots(th, traceW) + chord
	return lipgloss.NewStyle().Width(w).MaxWidth(w).Render(row)
}
