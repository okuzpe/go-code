package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// lineMetaKind classifies a transcript row for width reflow (see reflowTranscriptForWidth).
type lineMetaKind uint8

const (
	lineKindPlain lineMetaKind = iota
	lineKindSeparator
	lineKindToolCard
	lineKindAssistantMD
	lineKindThinking
	lineKindToolRunning
)

// lineMeta is parallel to Model.lines: same index, same length after mutations.
type lineMeta struct {
	kind lineMetaKind
	// startedAt is wall time when this row began (lineKindThinking, lineKindToolRunning).
	startedAt time.Time
	// thinkingLabel is the orchestrator thinking-phase line (lineKindThinking only); empty defaults to "Thinking".
	thinkingLabel string
	// tool card (lineKindToolCard) + tool running (lineKindToolRunning)
	toolName    string
	toolSummary string // input preview from OnToolUse (pattern, etc.)
	toolContent string // raw tool result body for reflow (glob/grep path lists); empty when not needed
	toolOutcome string // one-line result hint from TranscriptOutcomeSnippet (tool cards only)
	toolError   bool
	// assistant markdown (lineKindAssistantMD)
	assistantRaw string
}

func (m *Model) syncLineMetaLen() {
	for len(m.lineMeta) < len(m.lines) {
		m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindPlain})
	}
	if len(m.lineMeta) > len(m.lines) {
		m.lineMeta = m.lineMeta[:len(m.lines)]
	}
}

func (m *Model) appendPlainMeta() {
	m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindPlain})
}

func (m *Model) appendSeparatorMeta() {
	m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindSeparator})
}

func (m *Model) appendToolCardMeta(toolName, summary, outcome string, isError bool, content string) {
	m.lineMeta = append(m.lineMeta, lineMeta{
		kind:        lineKindToolCard,
		toolName:    toolName,
		toolSummary: summary,
		toolContent: content,
		toolOutcome: outcome,
		toolError:   isError,
	})
}

func (m *Model) appendThinkingMeta(started time.Time, thinkingLabel string) {
	m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindThinking, startedAt: started, thinkingLabel: thinkingLabel})
}

func (m *Model) appendToolRunningMeta(toolName, summary string, started time.Time) {
	m.lineMeta = append(m.lineMeta, lineMeta{
		kind:        lineKindToolRunning,
		toolName:    toolName,
		toolSummary: summary,
		startedAt:   started,
	})
}

func (m *Model) setAssistantMDMetaAt(idx int, raw string) {
	m.syncLineMetaLen()
	if idx < 0 || idx >= len(m.lineMeta) {
		return
	}
	m.lineMeta[idx] = lineMeta{kind: lineKindAssistantMD, assistantRaw: raw}
}

func (m *Model) appendAssistantMDMeta(raw string) {
	m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindAssistantMD, assistantRaw: raw})
}

// reflowTranscriptForWidth re-renders width-sensitive lines after m.width changed.
// Preserves preamble (welcome / static intro); reflows from preambleEnd onward.
func (m *Model) reflowTranscriptForWidth() {
	if m.width <= 0 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.syncLineMetaLen()
	start := m.preambleEnd
	if start < 0 {
		start = 0
	}
	if start > len(m.lines) {
		return
	}
	prefix := th.AssistantPrefix()
	prefixW := lipgloss.Width(prefix)
	for i := start; i < len(m.lines); i++ {
		switch m.lineMeta[i].kind {
		case lineKindPlain:
			continue
		case lineKindSeparator:
			m.lines[i] = th.SeparatorLine(m.width)
		case lineKindToolCard:
			meta := m.lineMeta[i]
			label := orchestrator.ToolFinishedPhrase(meta.toolName)
			summaryBody := orchestrator.ToolCardSummaryBody(meta.toolName, meta.toolSummary, meta.toolContent, meta.toolError)
			m.lines[i] = th.RenderToolCard(label, summaryBody, meta.toolOutcome, meta.toolError, m.width)
		case lineKindThinking:
			meta := m.lineMeta[i]
			elapsed := int(time.Since(meta.startedAt).Seconds())
			m.lines[i] = th.RenderThinkingRow(meta.thinkingLabel, elapsed, m.width)
		case lineKindToolRunning:
			meta := m.lineMeta[i]
			elapsed := int(time.Since(meta.startedAt).Seconds())
			label := orchestrator.ToolWorkingPhrase(meta.toolName)
			sum := meta.toolSummary
			m.lines[i] = th.RenderToolInProgressRow(label, sum, elapsed, m.width)
		case lineKindAssistantMD:
			raw := m.lineMeta[i].assistantRaw
			m.lines[i] = renderAssistantMarkdownSegment(th, raw, m.width, prefix, prefixW)
		}
	}
}

// renderAssistantMarkdownSegment renders one assistant markdown block (multi-line string in the transcript).
func renderAssistantMarkdownSegment(th *Theme, raw string, termWidth int, prefix string, prefixW int) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	rendered := th.RenderMarkdown(raw, termWidth, prefixW)
	mdLines := strings.Split(rendered, "\n")
	mdLines = normalizeAssistantMarkdownLines(mdLines)
	mdLines = dropLeadingBlankLines(mdLines)
	mdLines = dropTrailingBlankLines(mdLines)
	if len(mdLines) == 0 {
		return ""
	}
	padStr := strings.Repeat(" ", prefixW+1)
	finalLines := make([]string, 0, len(mdLines))
	for i, line := range mdLines {
		if i == 0 {
			finalLines = append(finalLines, fmt.Sprintf("%s %s", prefix, line))
		} else {
			finalLines = append(finalLines, padStr+line)
		}
	}
	return strings.Join(finalLines, "\n")
}
