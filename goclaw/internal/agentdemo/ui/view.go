package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) View() tea.View {
	m.layout()
	status := m.statusLine()
	vp := m.viewport.View()
	inputBox := m.st.Panel.Width(m.width).Render(m.input.View())
	main := lipgloss.JoinVertical(lipgloss.Left, status, vp, m.agentBar(), inputBox)
	if m.showLogOverlay && len(m.logLines) > 0 {
		logText := strings.Join(m.logLines[max(0, len(m.logLines)-12):], "\n")
		logBox := m.st.Overlay.Width(m.width).Render(
			m.st.Title.Render(" session log ") + "\n" + m.st.Dim.Render(logText) + "\n" + m.st.Dim.Render("Esc close"))
		main = lipgloss.JoinVertical(lipgloss.Left, main, logBox)
	}
	v := tea.NewView(main)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
