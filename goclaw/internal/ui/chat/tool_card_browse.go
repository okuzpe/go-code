package chat

import (
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/text"
)

const (
	toolCardExpandedMaxLines     = 72
	toolCardExpandedMaxRunesLine = 240
)

func (m *Model) toolCardLineIndices() []int {
	var out []int
	for i := range m.lines {
		if i < len(m.lineMeta) && m.lineMeta[i].kind == lineKindToolCard {
			out = append(out, i)
		}
	}
	return out
}

func (m *Model) cycleBrowseToolCardFocus(delta int) {
	ids := m.toolCardLineIndices()
	if len(ids) == 0 {
		m.browseToolCardLine = -1
		return
	}
	if m.browseToolCardLine < 0 {
		if delta >= 0 {
			m.browseToolCardLine = ids[0]
		} else {
			m.browseToolCardLine = ids[len(ids)-1]
		}
		return
	}
	at := -1
	for i, id := range ids {
		if id == m.browseToolCardLine {
			at = i
			break
		}
	}
	if at < 0 {
		m.browseToolCardLine = ids[len(ids)-1]
		return
	}
	next := at + delta
	for next < 0 {
		next += len(ids)
	}
	next %= len(ids)
	m.browseToolCardLine = ids[next]
}

func (m *Model) toggleBrowseToolCardExpanded() {
	if m.browseToolCardLine < 0 || m.browseToolCardLine >= len(m.lineMeta) {
		return
	}
	if m.lineMeta[m.browseToolCardLine].kind != lineKindToolCard {
		return
	}
	meta := m.lineMeta[m.browseToolCardLine]
	meta.toolExpanded = !meta.toolExpanded
	m.lineMeta[m.browseToolCardLine] = meta
}

func expandedToolSummaryBody(raw string, termWidth int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	inner := toolCardInnerWidth(termWidth) - 2
	if inner < 16 {
		inner = 16
	}
	lines := strings.Split(raw, "\n")
	extra := 0
	if len(lines) > toolCardExpandedMaxLines {
		extra = len(lines) - toolCardExpandedMaxLines
		lines = lines[:toolCardExpandedMaxLines]
	}
	var b strings.Builder
	for _, ln := range lines {
		ln = strings.TrimRight(ln, "\r")
		if strings.TrimSpace(ln) == "" && b.Len() == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(text.TruncateRunes(ln, min(inner, toolCardExpandedMaxRunesLine)))
	}
	if extra > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "… %d more line(s) not shown (use Ctrl+T for full log)", extra)
	}
	return b.String()
}

func prefixFirstTranscriptLine(prefix, block string) string {
	if prefix == "" {
		return block
	}
	parts := strings.SplitN(block, "\n", 2)
	if len(parts) == 1 {
		return prefix + parts[0]
	}
	return prefix + parts[0] + "\n" + parts[1]
}
