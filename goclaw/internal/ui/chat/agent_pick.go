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
		m.agentPickFullText = th.ModalTitle.Render("No agents available")
		return
	}
	if m.agentPickCursor >= len(items) {
		m.agentPickCursor = len(items) - 1
	}
	var b strings.Builder
	b.Grow(len(items)*96 + 384)
	b.WriteString(th.ModalTitle.Render("Agent profile"))
	b.WriteString("\n\n")
	for i, name := range items {
		line := name
		if p, ok := agentProfileByName(m, name); ok {
			line = name + " — " + p.Summary()
		}
		if i == m.agentPickCursor {
			b.WriteString(th.SlashPickerName.Render("▸ " + line))
		} else {
			b.WriteString(th.ModalBody.Render("  " + line))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.FooterDim.Render("↑↓ move · Enter apply · Esc cancel"))
	b.WriteString("\n")
	b.WriteString(th.FooterDim.Render("Same as /profile <name> · custom agents from ~/.goclaw/agents and project .goclaw/agents"))
	m.agentPickFullText = b.String()
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
	handled, out, quit, modelSubmit, hints, err := m.slashHandle("/agents " + name)
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
			m.runModelSubmit(modelSubmit)
		}
		if quit {
			// /agents does not quit; ignore
			_ = quit
		}
	}
}
