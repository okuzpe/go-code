package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/agents"
)

func bareAgentsSlashInput(raw string) bool {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) != 1 {
		return false
	}
	return strings.EqualFold(strings.TrimPrefix(fields[0], "/"), "agents")
}

func bareProfileSlashInput(raw string) bool {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) != 1 {
		return false
	}
	return strings.EqualFold(strings.TrimPrefix(fields[0], "/"), "profile")
}

func (m *Model) agentPickNames() []string {
	profs, err := agents.AllWithCustom(m.userAgentsDir, m.projectAgentsDir)
	if err != nil || len(profs) == 0 {
		profs = agents.All()
	}
	names := agents.SortedKeys(profs)
	if len(m.agentPickerHidden) == 0 {
		return names
	}
	hide := make(map[string]struct{}, len(m.agentPickerHidden))
	for _, h := range m.agentPickerHidden {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			hide[h] = struct{}{}
		}
	}
	var out []string
	for _, n := range names {
		if _, skip := hide[strings.ToLower(n)]; skip {
			continue
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return names
	}
	return out
}

func (m *Model) refreshAgentPickOverlay() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	items := m.agentPickNames()
	if m.agentPickCursor < 0 {
		m.agentPickCursor = 0
	}
	if len(items) == 0 {
		m.agentPickFullText = th.OverlayTitle.Render("No Agents Available")
		return
	}
	if m.agentPickCursor >= len(items) {
		m.agentPickCursor = len(items) - 1
	}
	rows := make([]listPickerItem, 0, len(items))
	for i, name := range items {
		line := name
		if p, ok := agentProfileByName(m, name); ok {
			line = name + " — " + p.Summary()
		}
		rows = append(rows, listPickerItem{label: line, selected: i == m.agentPickCursor})
	}
	m.agentPickFullText = renderListPicker(
		th,
		"Profile",
		rows,
		"↑↓ move · Enter apply · Esc cancel",
		"Primary flow: /profile or Ctrl+P · custom profiles from ~/.goclaw/agents and project .goclaw/agents",
	)
}

func agentProfileByName(m *Model, name string) (agents.Profile, bool) {
	profs, err := agents.AllWithCustom(m.userAgentsDir, m.projectAgentsDir)
	if err != nil {
		profs = agents.All()
	}
	p, ok := profs[name]
	return p, ok
}

func (m *Model) openAgentPicker() {
	m.exitTranscriptBrowse()
	m.exitConfirmDeadline = time.Time{}
	m.docOverlayOpen = false
	m.docOverlayTitle = ""
	m.docOverlaySourceMD = ""
	m.themePickOpen = false
	m.themePickFullText = ""
	items := m.agentPickNames()
	m.agentPickCursor = 0
	cur := strings.TrimSpace(m.activeAgentProfile)
	for i, name := range items {
		if name == cur {
			m.agentPickCursor = i
			break
		}
	}
	m.agentPickOpen = true
	m.refreshAgentPickOverlay()
	m.syncViewportKeyMapForOverlay()
	m.layout()
	m.viewport.GotoTop()
}

func (m *Model) closeAgentPicker() {
	m.agentPickOpen = false
	m.agentPickFullText = ""
	m.syncViewportKeyMapForCompose()
	m.layout()
	m.viewport.GotoBottom()
}

func (m *Model) moveAgentPickCursor(delta int) {
	items := m.agentPickNames()
	if len(items) == 0 {
		return
	}
	n := len(items)
	m.agentPickCursor = (m.agentPickCursor + delta%n + n) % n
	m.refreshAgentPickOverlay()
	m.layout()
	m.viewport.GotoTop()
}

func (m *Model) applyAgentPick() {
	items := m.agentPickNames()
	if len(items) == 0 {
		m.closeAgentPicker()
		m.appendError("no agents available")
		return
	}
	if m.agentPickCursor < 0 || m.agentPickCursor >= len(items) {
		m.closeAgentPicker()
		m.appendError("invalid agent selection")
		return
	}
	name := items[m.agentPickCursor]
	m.closeAgentPicker()
	if m.slashHandle == nil {
		m.appendError("slash handler not configured")
		return
	}
	handled, out, quit, modelSubmit, hints, err := m.slashHandle("/profile " + name)
	if err != nil {
		m.appendError(fmt.Sprintf("%v", err))
		return
	}
	if handled {
		if strings.TrimSpace(out) != "" {
			if hints.TUIDocOverlay {
				m.openDocOverlay(hints.TUIDocTitle, out)
			} else {
				m.appendSystem(out)
			}
		}
		m.applySlashHints(hints)
		if strings.TrimSpace(modelSubmit) != "" && m.submitter != nil && m.submitter.fn != nil {
			m.runModelSubmit(modelSubmit, m.interactMode)
		}
		if quit {
			// /agents does not quit; ignore
			_ = quit
		}
	}
}
