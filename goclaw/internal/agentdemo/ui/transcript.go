package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
)

func (m *Model) appendLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > 200 {
		m.logLines = m.logLines[len(m.logLines)-200:]
	}
}

func (m *Model) appendUserBlock(text string) {
	m.blocks = append(m.blocks,
		m.st.User.Render("You")+"\n"+m.st.Dim.Render(text))
}

func (m *Model) appendSystemBlock(text string) {
	m.blocks = append(m.blocks, m.st.Dim.Render(text))
}

func (m *Model) appendErrorBlock(text string) {
	m.blocks = append(m.blocks, m.st.Err.Render(text))
}

func (m *Model) appendToolRunning(name string) {
	m.toolRunning = true
	m.spinnerActive = true
	m.phase = "executing"
	line := m.st.Tool.Render("● tool ") + m.st.Dim.Render(name+" …")
	m.blocks = append(m.blocks, line)
}

func (m *Model) appendToolSummary(summary string) {
	m.toolRunning = false
	m.spinnerActive = false
	if m.phase == "executing" {
		m.phase = "idle"
	}
	m.blocks = append(m.blocks, m.st.ToolDim.Render("  "+summary))
}

func (m *Model) streamAppend(delta string) {
	m.streamMu.Lock()
	m.stream.WriteString(delta)
	m.streamMu.Unlock()
}

func (m *Model) streamReset() {
	m.streamMu.Lock()
	m.stream.Reset()
	m.streamMu.Unlock()
}

func (m *Model) streamSnapshot() string {
	m.streamMu.Lock()
	s := m.stream.String()
	m.streamMu.Unlock()
	return s
}

func (m *Model) glamourMarkdown(src string) (string, error) {
	w := m.width - 6
	if w < 36 {
		w = 36
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(w),
		glamour.WithStandardStyle("dark"),
	)
	if err != nil {
		return "", err
	}
	out, err := r.Render(strings.TrimSpace(src))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (m *Model) finalizeAssistantTurn() {
	raw := m.streamSnapshot()
	m.streamReset()
	if strings.TrimSpace(raw) == "" {
		return
	}
	rendered, err := m.glamourMarkdown(raw)
	if err != nil || strings.TrimSpace(rendered) == "" {
		rendered = m.st.Assistant.Render(raw)
	}
	m.blocks = append(m.blocks, m.st.Accent.Render("Assistant")+"\n"+rendered)
}

func (m *Model) approxTokenEstimate() int {
	n := 0
	for _, b := range m.blocks {
		n += utf8.RuneCountInString(b)
	}
	n += utf8.RuneCountInString(m.streamSnapshot())
	return max(1, n*3/4)
}

func (m *Model) welcomeBlock() {
	host := strings.TrimSpace(m.cfg.OllamaHost)
	if host == "" {
		host = "http://127.0.0.1:11434"
	}
	mod := strings.TrimSpace(m.cfg.Model)
	m.appendSystemBlock(fmt.Sprintf(
		"goclaw agentdemo — Ollama %s · model %s\n"+
			"Commands: help · clear · demo-tool · quit\n"+
			"Keys: Ctrl+C cancel stream or quit · Ctrl+M mode · Ctrl+E logs · Tab complete\n"+
			"Note: Ctrl+M may be eaten by the terminal (carriage return); use the visible mode in the status strip.",
		host, mod))
}
