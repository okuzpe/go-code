package chat

import (
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/stretchr/testify/require"
)

func TestReflowTranscriptForWidth_separatorAndToolCard(t *testing.T) {
	th := DefaultTheme()
	m := &Model{
		theme:           th,
		width:           100,
		preambleEnd:     0,
		lastReflowWidth: -1,
	}
	start := len(m.lines)
	m.width = 100
	m.lines = append(m.lines, th.SeparatorLine(100))
	m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindSeparator})
	label := orchestrator.ToolFinishedPhrase("bash")
	summary := "echo hello"
	m.lines = append(m.lines, th.RenderToolCard(label, summary, "hello", false, 100))
	m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindToolCard, toolName: "bash", toolSummary: summary, toolOutcome: "hello", toolError: false})

	m.width = 42
	m.reflowTranscriptForWidth()

	sepPlain := strings.TrimSpace(stripANSI(m.lines[start]))
	require.Contains(t, sepPlain, "─")
	// SeparatorLine uses a compact rule (~40% of width, min 8, max 40), not full terminal width.
	ruleW := m.width * 2 / 5
	if ruleW < 8 {
		ruleW = 8
	}
	if ruleW > 40 {
		ruleW = 40
	}
	require.Equal(t, ruleW, strings.Count(sepPlain, "─"), "separator rule width should match theme.SeparatorLine")

	card := stripANSI(m.lines[start+1])
	require.Contains(t, card, "echo hello")
}
