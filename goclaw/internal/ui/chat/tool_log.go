package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/text"
)

const (
	toolLogContentCap      = 64 * 1024 // 64 KB display cap for detail view (UTF-8–safe prefix)
	toolLogSummaryMaxRunes = 60
)

// openToolLog opens the tool history overlay at the most recent entry.
func (m *Model) openToolLog() {
	if len(m.toolLog) == 0 {
		return
	}
	m.exitTranscriptBrowse()
	m.helpOpen = false
	m.helpFullText = ""
	m.themePickOpen = false
	m.themePickFullText = ""
	m.agentPickOpen = false
	m.agentPickFullText = ""
	m.toolLogCursor = len(m.toolLog) - 1
	m.toolLogDetail = false
	m.toolLogOpen = true
	m.refreshToolLogOverlay()
	m.syncViewportKeyMapForOverlay()
	m.layout()
	m.viewport.GotoTop()
}

// closeToolLog closes the tool history overlay and returns to the transcript.
func (m *Model) closeToolLog() {
	m.toolLogOpen = false
	m.toolLogDetail = false
	m.toolLogText = ""
	m.syncViewportKeyMapForCompose()
	m.layout()
	m.viewport.GotoBottom()
}

// moveToolLogCursor moves the cursor by delta (±1) within the list view and refreshes.
func (m *Model) moveToolLogCursor(delta int) {
	if len(m.toolLog) == 0 {
		return
	}
	m.toolLogCursor += delta
	if m.toolLogCursor < 0 {
		m.toolLogCursor = 0
	}
	if m.toolLogCursor >= len(m.toolLog) {
		m.toolLogCursor = len(m.toolLog) - 1
	}
	m.refreshToolLogOverlay()
	m.layout()
}

// refreshToolLogOverlay re-renders the overlay text into m.toolLogText.
func (m *Model) refreshToolLogOverlay() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}

	if m.toolLogDetail && m.toolLogCursor >= 0 && m.toolLogCursor < len(m.toolLog) {
		entry := m.toolLog[m.toolLogCursor]
		var b strings.Builder
		icon := "✓"
		if entry.isError {
			icon = "✗"
		}
		b.WriteString(th.ModalTitle.Render(fmt.Sprintf("[%s] %s", icon, orchestrator.ToolFinishedPhrase(entry.name))))
		b.WriteString("\n")
		if entry.summary != "" {
			b.WriteString(th.FooterDim.Render("Input: " + entry.summary))
			b.WriteString("\n")
		}
		if entry.elapsed > 0 {
			b.WriteString(th.FooterDim.Render(fmt.Sprintf("Elapsed: %.2fs  Size: %s", entry.elapsed.Seconds(), formatBytes(len(entry.content)))))
		}
		b.WriteString("\n\n")
		content := entry.content
		truncated := len(content) > toolLogContentCap
		if truncated {
			content = text.TruncateUTF8ByBytes(content, toolLogContentCap)
		}
		b.WriteString(th.ModalBody.Render(content))
		if truncated {
			b.WriteString("\n")
			b.WriteString(th.FooterDim.Render("… output truncated (>64 KB)"))
		}
		b.WriteString("\n\n")
		b.WriteString(th.FooterDim.Render("Esc back to list · Ctrl+C quit"))
		m.toolLogText = b.String()
		return
	}

	// List view.
	var b strings.Builder
	total := len(m.toolLog)
	b.WriteString(th.ModalTitle.Render(fmt.Sprintf("Tool history (%d steps)", total)))
	b.WriteString("\n\n")
	for i, entry := range m.toolLog {
		icon := "✓"
		if entry.isError {
			icon = "✗"
		}
		label := orchestrator.ToolFinishedPhrase(entry.name)
		detail := ""
		if entry.summary != "" {
			sum := text.TruncateRunes(entry.summary, toolLogSummaryMaxRunes)
			detail = "  " + sum
		}
		if entry.outcome != "" {
			out := text.TruncateRunes(entry.outcome, toolLogSummaryMaxRunes)
			if detail != "" {
				detail += th.FooterDim.Render(" · ") + th.FooterDim.Render(out)
			} else {
				detail = "  " + th.FooterDim.Render(out)
			}
		}
		elapsed := ""
		if entry.elapsed > 0 {
			elapsed = fmt.Sprintf("  (%.1fs, %s)", entry.elapsed.Seconds(), formatBytes(len(entry.content)))
		}
		line := fmt.Sprintf("[%s] %s%s%s", icon, label, detail, elapsed)
		if i == m.toolLogCursor {
			b.WriteString(th.SlashPickerName.Render("▸ " + line))
		} else {
			b.WriteString(th.ModalBody.Render("  " + line))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.FooterDim.Render("↑↓ move · Enter view output · Esc/Ctrl+T close · Ctrl+C quit"))
	m.toolLogText = b.String()
}

func formatBytes(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

// appendToToolLog records a finished tool in the session log.
// Called from appendToolDoneLine after the compact card line is added to m.lines.
func (m *Model) appendToToolLog(name, summary, content string, isError bool, outcome string) {
	var elapsed time.Duration
	if !m.toolLogStart.IsZero() {
		elapsed = time.Since(m.toolLogStart)
	}
	if outcome == "" {
		outcome = orchestrator.TranscriptOutcomeSnippet(name, content, isError)
	}
	m.toolLog = append(m.toolLog, toolLogEntry{
		name:    name,
		summary: summary,
		outcome: outcome,
		content: content,
		isError: isError,
		elapsed: elapsed,
	})
}
