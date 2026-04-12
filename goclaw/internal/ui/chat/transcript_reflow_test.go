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
	m.lines = append(m.lines, th.RenderToolCard(label, summary, false, 100))
	m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindToolCard, toolName: "bash", toolSummary: summary, toolError: false})

	m.width = 42
	m.reflowTranscriptForWidth()

	sepPlain := strings.TrimSpace(stripANSI(m.lines[start]))
	require.Contains(t, sepPlain, "─")
	require.Equal(t, 42, strings.Count(sepPlain, "─"), "separator should match terminal width")

	card := stripANSI(m.lines[start+1])
	require.Contains(t, card, "bash")
}
