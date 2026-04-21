package chat

import (
	"strings"
	"testing"
)

func TestToolCardLineIndices_andCycle(t *testing.T) {
	th := DefaultTheme()
	var m Model
	m.theme = th
	m.width = 80
	m.lines = []string{"a", "b", "c"}
	m.lineMeta = []lineMeta{
		{kind: lineKindPlain},
		{kind: lineKindToolCard, toolName: "bash", toolSummary: "x", toolContent: "out", toolOutcome: "ok", toolError: false},
		{kind: lineKindToolCard, toolName: "glob", toolSummary: "y", toolContent: "z", toolOutcome: "ok", toolError: false},
	}
	m.lines[1] = th.RenderToolCard("bash", "x", "ok", false, m.width)
	m.lines[2] = th.RenderToolCard("glob", "y", "ok", false, m.width)

	ids := m.toolCardLineIndices()
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("indices: %#v", ids)
	}
	m.browseToolCardLine = -1
	m.cycleBrowseToolCardFocus(1)
	if m.browseToolCardLine != 1 {
		t.Fatalf("first focus: got %d want 1", m.browseToolCardLine)
	}
	m.cycleBrowseToolCardFocus(1)
	if m.browseToolCardLine != 2 {
		t.Fatalf("second focus: got %d want 2", m.browseToolCardLine)
	}
	m.cycleBrowseToolCardFocus(1)
	if m.browseToolCardLine != 1 {
		t.Fatalf("wrap forward: got %d want 1", m.browseToolCardLine)
	}
	m.toggleBrowseToolCardExpanded()
	if !m.lineMeta[1].toolExpanded {
		t.Fatal("expected expanded")
	}
	m.reflowTranscriptForWidth()
	if m.lines[1] == "" {
		t.Fatal("expected non-empty reflowed line")
	}
}

func TestExpandedToolSummaryBody_truncatesManyLines(t *testing.T) {
	lines := make([]string, toolCardExpandedMaxLines+5)
	for i := range lines {
		lines[i] = "x"
	}
	body := expandedToolSummaryBody(strings.Join(lines, "\n"), 100)
	if body == "" {
		t.Fatal("expected body")
	}
	if !strings.Contains(body, "more line") {
		t.Fatalf("expected truncation notice: %q", body)
	}
}
