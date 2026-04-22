package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/agentdemo/components"
)

func (m *Model) syncInputWidth() {
	if m.width <= 4 {
		m.input.SetWidth(0)
		return
	}
	w := m.width - 4
	if w < minComposeWidth {
		w = minComposeWidth
	}
	m.input.SetWidth(w)
}

func (m *Model) resizeInput() {
	lc := m.input.LineCount()
	if lc < 1 {
		lc = 1
	}
	if lc > inputMaxHeight {
		lc = inputMaxHeight
	}
	if m.inputLines == lc {
		return
	}
	m.inputLines = lc
	m.input.SetHeight(lc)
	m.layout()
}

func (m *Model) layout() {
	m.syncInputWidth()
	vpH := TranscriptViewportHeight(m.height, m.inputLines, m.showLogOverlay)
	if vpH < minTranscriptRows {
		vpH = minTranscriptRows
	}
	if m.width > 0 {
		m.viewport.SetWidth(m.width)
	}
	m.viewport.SetHeight(vpH)
	m.syncTranscript()
}

func (m *Model) syncTranscript() {
	body := m.transcriptBody()
	stick := m.viewport.TotalLineCount() == 0 || m.viewport.AtBottom()
	m.viewport.SetContent(body)
	if stick {
		m.viewport.GotoBottom()
	}
}

func (m *Model) transcriptBody() string {
	var b strings.Builder
	for _, bk := range m.blocks {
		b.WriteString(bk)
		b.WriteString("\n\n")
	}
	if m.streaming || strings.TrimSpace(m.streamSnapshot()) != "" {
		s := m.streamSnapshot()
		if strings.TrimSpace(s) != "" {
			b.WriteString(m.st.Accent.Render("Assistant"))
			b.WriteString("\n")
			b.WriteString(m.st.Dim.Render(s))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) statusLine() string {
	ph := m.phase
	if m.streaming && ph == PhaseThinking {
		ph = PhaseStreaming
	}
	return components.StatusStrip(m.width, m.st.Accent, m.st.Dim, m.st.Panel,
		m.cfg.Model, m.mode.String(), string(ph), m.spinner.View(), m.approxTokenEstimate(), m.cfg.Unsafe)
}

func (m *Model) agentBar() string {
	sep := m.st.Dim.Render(strings.Repeat("─", m.width))
	var content string
	switch m.phase {
	case PhaseThinking:
		// Spinner frame rendered as plain string — no double-wrap.
		content = m.st.Accent.Render("[") + m.spinner.View() + m.st.Accent.Render("]") +
			m.st.Dim.Render(" Thinking...")
	case PhaseStreaming:
		content = m.st.Accent.Render("[▸]") + m.st.Dim.Render(" Writing...")
	case PhaseExecuting:
		tool := m.currentTool
		if tool == "" {
			tool = "tool"
		}
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("[") +
			m.spinner.View() +
			lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("]") +
			m.st.Dim.Render(" Running → "+tool)
	default:
		switch m.lastResult {
		case "ok":
			content = m.st.Accent.Render("[✓]") + m.st.Dim.Render(" Ready")
		case "error":
			content = m.st.Err.Render("[✖]") + m.st.Dim.Render(" Error")
		default:
			content = m.st.Dim.Render("[·] Ready")
		}
	}
	bar := lipgloss.NewStyle().Width(m.width).Render(content)
	return sep + "\n" + bar
}
