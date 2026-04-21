package chat

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/okuzpe/goclaw/internal/gitdiff"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/text"
)

func (m *Model) rebuildTranscriptForSession(s *session.Session) {
	if s == nil {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	var head []string
	var metaHead []lineMeta
	if m.preambleEnd > 0 && m.preambleEnd <= len(m.lines) {
		head = append([]string(nil), m.lines[:m.preambleEnd]...)
		if m.preambleEnd <= len(m.lineMeta) {
			metaHead = append([]lineMeta(nil), m.lineMeta[:m.preambleEnd]...)
		} else {
			m.syncLineMetaLen()
			if m.preambleEnd <= len(m.lineMeta) {
				metaHead = append([]lineMeta(nil), m.lineMeta[:m.preambleEnd]...)
			}
		}
	}
	body := strings.TrimSpace(s.PlainTranscript())
	if body == "" {
		body = "(no messages in this session)"
	}
	msg := "Resumed session " + s.ID + " (" + strconv.Itoa(s.Len()) + " messages)\n" + body
	m.sessionID = s.ID
	m.lines = append(head, th.System.Render(msg))
	m.lineMeta = append(metaHead, lineMeta{kind: lineKindPlain})
	m.setLinesContent(true)
}

// rebuildWelcomeForWidth rebuilds the top welcome panel using the real terminal width so layout
// does not rely on a too-wide two-column block before the first WindowSizeMsg.
func (m *Model) rebuildWelcomeForWidth() {
	if m.welcomeBlockEnd <= 0 || m.width <= 0 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	dash := WelcomeDashboardLines(th, m.welcomeOpts, m.width)
	if len(dash) == 0 {
		return
	}
	newHead := append(append([]string(nil), dash...), "")
	oldEnd := m.welcomeBlockEnd
	newPlain := make([]lineMeta, len(newHead))
	for i := range newPlain {
		newPlain[i] = lineMeta{kind: lineKindPlain}
	}
	if len(newHead) != oldEnd {
		tail := append([]string(nil), m.lines[oldEnd:]...)
		m.lines = append(newHead, tail...)
		var tailMeta []lineMeta
		if oldEnd <= len(m.lineMeta) {
			tailMeta = append([]lineMeta(nil), m.lineMeta[oldEnd:]...)
		}
		m.lineMeta = append(newPlain, tailMeta...)
	} else {
		copy(m.lines, newHead)
		for len(m.lineMeta) < len(newHead) {
			m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindPlain})
		}
		for i := 0; i < len(newHead); i++ {
			m.lineMeta[i] = lineMeta{kind: lineKindPlain}
		}
	}
	m.welcomeBlockEnd = len(newHead)
}

// reflowTitleSeparator widens the rule under the banner when the terminal resizes.
func (m *Model) reflowTitleSeparator() {
	if m.width <= 0 || len(m.lines) < 2 {
		return
	}
	// Welcome rows include "goclaw" in subtitles; replacing the next line would corrupt the panel.
	if m.welcomeBlockEnd > 0 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	for idx := 0; idx < len(m.lines)-1; idx++ {
		plain := strings.TrimSpace(stripANSI(m.lines[idx]))
		if strings.HasPrefix(plain, "goclaw") {
			m.syncLineMetaLen()
			m.lines[idx+1] = th.SeparatorLine(m.width)
			m.lineMeta[idx+1] = lineMeta{kind: lineKindSeparator}
			return
		}
	}
}

// ─── Line append helpers ────────────────────────────────────────────────────

func (m *Model) appendSeparator() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.SeparatorLine(m.width))
	m.appendSeparatorMeta()
	m.setLinesContent(true)
}

func (m *Model) appendSystem(s string) {
	m.appendSystemStick(s, true)
}

func (m *Model) appendSystemStick(s string, stickToBottom bool) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.System.Render(s))
	m.appendPlainMeta()
	m.setLinesContent(stickToBottom)
}

func (m *Model) appendError(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.ErrorStyle.Render(s))
	m.appendPlainMeta()
	m.setLinesContent(true)
}

func (m *Model) appendUser(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, fmt.Sprintf("%s %s", th.UserPrefix(), renderUserTranscriptLine(s, th, m.workdir)))
	m.appendPlainMeta()
	m.setLinesContent(true)
}

func (m *Model) appendAssistant(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, fmt.Sprintf("%s %s", th.AssistantPrefix(), s))
	m.appendPlainMeta()
	m.setLinesContent(true)
}

func (m *Model) widthOrDefault() int {
	if m.width > 0 {
		return m.width
	}
	return defaultTerminalWidthFallback
}

// findToolRunLineByName returns the transcript line index for the in-progress tool card
// whose toolName matches name. Falls back to toolRunLineIdx[0] when no exact match is
// found (e.g. the name is empty or the card was not yet registered). Returns -1 when
// there are no in-flight tool cards at all.
func (m *Model) findToolRunLineByName(name string) int {
	for _, idx := range m.toolRunLineIdx {
		if idx >= 0 && idx < len(m.lineMeta) && m.lineMeta[idx].toolName == name {
			return idx
		}
	}
	if len(m.toolRunLineIdx) > 0 {
		return m.toolRunLineIdx[0]
	}
	return -1
}

func (m *Model) appendThinkingRow(phaseLabel string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	now := time.Now()
	idx := len(m.lines)
	m.lines = append(m.lines, th.RenderThinkingRow(phaseLabel, 0, m.widthOrDefault()))
	m.appendThinkingMeta(now, phaseLabel)
	m.thinkingLineIdx = idx
	m.setLinesContent(false)
}

func (m *Model) removeTranscriptLineAt(at int) {
	if at < 0 || at >= len(m.lines) {
		return
	}
	for i := range m.toolRunLineIdx {
		if m.toolRunLineIdx[i] > at {
			m.toolRunLineIdx[i]--
		}
	}
	if m.curAssistantLineIdx > at {
		m.curAssistantLineIdx--
	}
	if m.thinkingLineIdx == at {
		m.thinkingLineIdx = -1
	} else if m.thinkingLineIdx > at {
		m.thinkingLineIdx--
	}
	m.lines = append(m.lines[:at], m.lines[at+1:]...)
	if at < len(m.lineMeta) {
		m.lineMeta = append(m.lineMeta[:at], m.lineMeta[at+1:]...)
	}
}

func (m *Model) clearThinkingLine() {
	if m.thinkingLineIdx < 0 {
		return
	}
	idx := m.thinkingLineIdx
	if idx >= len(m.lines) || idx >= len(m.lineMeta) {
		m.thinkingLineIdx = -1
		return
	}
	if m.lineMeta[idx].kind != lineKindThinking {
		m.thinkingLineIdx = -1
		return
	}
	m.removeTranscriptLineAt(idx)
	m.thinkingLineIdx = -1
	m.setLinesContent(false)
}

func (m *Model) refreshThinkingTranscriptRow() {
	if m.thinkingLineIdx < 0 || !m.assistantPlaceholder {
		return
	}
	idx := m.thinkingLineIdx
	if idx < 0 || idx >= len(m.lines) || idx >= len(m.lineMeta) {
		return
	}
	if m.lineMeta[idx].kind != lineKindThinking {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	elapsed := int(time.Since(m.lineMeta[idx].startedAt).Seconds())
	m.lines[idx] = th.RenderThinkingRow(m.lineMeta[idx].thinkingLabel, elapsed, m.widthOrDefault())
	m.setLinesContent(false)
}

func (m *Model) refreshToolRunningTranscriptRows() {
	if len(m.toolRunLineIdx) == 0 || len(m.toolWaitQueue) == 0 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	changed := false
	for i := range m.toolRunLineIdx {
		if i >= len(m.toolWaitQueue) {
			break
		}
		lineIdx := m.toolRunLineIdx[i]
		if lineIdx < 0 || lineIdx >= len(m.lines) || lineIdx >= len(m.lineMeta) {
			continue
		}
		if m.lineMeta[lineIdx].kind != lineKindToolRunning {
			continue
		}
		job := m.toolWaitQueue[i]
		elapsed := int(time.Since(m.lineMeta[lineIdx].startedAt).Seconds())
		label := orchestrator.ToolWorkingPhrase(job.name)
		m.lines[lineIdx] = th.RenderToolInProgressRow(label, job.summary, elapsed, m.widthOrDefault())
		changed = true
	}
	if changed {
		m.setLinesContent(false)
	}
}

func (m *Model) appendToolRunningTranscriptRow(toolName, preview string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	now := time.Now()
	lineIdx := len(m.lines)
	label := orchestrator.ToolWorkingPhrase(toolName)
	m.lines = append(m.lines, th.RenderToolInProgressRow(label, preview, 0, m.widthOrDefault()))
	m.appendToolRunningMeta(toolName, preview, now)
	m.toolRunLineIdx = append(m.toolRunLineIdx, lineIdx)
	m.setLinesContent(false)
}

func (m *Model) replaceToolRunningWithCard(lineIdx int, toolName, summary, content string, isError bool) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	label := orchestrator.ToolFinishedPhrase(toolName)
	if lineIdx < 0 || lineIdx >= len(m.lines) {
		m.appendToolDoneLine(toolName, summary, content, isError)
		return
	}
	summaryBody := orchestrator.ToolCardSummaryBody(toolName, summary, content, isError)
	outcome := orchestrator.TranscriptOutcomeSnippet(toolName, content, isError)
	card := th.RenderToolCard(label, summaryBody, outcome, isError, m.widthOrDefault())
	m.lines[lineIdx] = card
	m.syncLineMetaLen()
	if lineIdx < len(m.lineMeta) {
		m.lineMeta[lineIdx] = lineMeta{
			kind:        lineKindToolCard,
			toolName:    toolName,
			toolSummary: summary,
			toolContent: content,
			toolOutcome: outcome,
			toolError:   isError,
		}
	}
	m.setLinesContent(false)
	m.appendToToolLog(toolName, summary, content, isError, outcome)
}

func (m *Model) stripAssistantPlaceholderLine() {
	if len(m.lines) == 0 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	last := m.lines[len(m.lines)-1]
	plain := stripANSI(last)
	pfx := th.AssistantPlainPrefix()
	if isAssistantDimPlaceholderLine(plain, pfx) {
		m.lines = m.lines[:len(m.lines)-1]
		if len(m.lineMeta) == len(m.lines)+1 {
			m.lineMeta = m.lineMeta[:len(m.lineMeta)-1]
		}
		m.setLinesContent(false)
	}
}

// isAssistantDimPlaceholderLine reports whether plain text is our dim "…" (or ASCII "...") assistant row.
func isAssistantDimPlaceholderLine(plain, assistantPlainPrefix string) bool {
	if !strings.HasPrefix(plain, assistantPlainPrefix) {
		return false
	}
	rest := strings.TrimSpace(plain[len(assistantPlainPrefix):])
	if strings.Contains(rest, "…") && strings.TrimSpace(strings.ReplaceAll(rest, "…", "")) == "" {
		return true
	}
	// ASCII ellipsis only
	if strings.Trim(rest, ".") == "" && len(rest) > 0 {
		return true
	}
	return false
}

func (m *Model) toolQueueStatusLine() string {
	if len(m.toolWaitQueue) == 0 {
		return ""
	}
	job := m.toolWaitQueue[0]
	base := orchestrator.ToolWorkingPhrase(job.name)
	if cat := strings.TrimSpace(orchestrator.ToolPhaseHeadline(job.name)); cat != "" {
		base = cat + " · " + base
	}
	if summary := strings.TrimSpace(job.summary); summary != "" {
		summary = text.TruncateRunes(summary, toolQueueSummaryMaxRunes)
		base = base + " · " + summary
	}
	if !m.toolWaitStartedAt.IsZero() {
		secs := int(time.Since(m.toolWaitStartedAt).Seconds())
		if secs >= 1 {
			base = fmt.Sprintf("%s (%ds)", base, secs)
		}
	}
	return base + "…"
}

// dequeueToolResult removes the matching in-flight tool (by toolUseID when set, else FIFO)
// and returns the transcript line index for its running row (-1 when none).
func (m *Model) dequeueToolResult(toolUseID, name string) (lineIdx int, job pendingTool) {
	lineIdx = -1
	if len(m.toolWaitQueue) == 0 {
		return -1, pendingTool{name: name, summary: ""}
	}
	idx := 0
	found := false
	if strings.TrimSpace(toolUseID) != "" {
		for i := range m.toolWaitQueue {
			if m.toolWaitQueue[i].toolUseID == toolUseID {
				idx = i
				found = true
				break
			}
		}
	}
	if !found {
		idx = 0
	}
	job = m.toolWaitQueue[idx]
	if idx < len(m.toolRunLineIdx) {
		lineIdx = m.toolRunLineIdx[idx]
	}
	m.toolWaitQueue = append(m.toolWaitQueue[:idx], m.toolWaitQueue[idx+1:]...)
	m.toolRunLineIdx = append(m.toolRunLineIdx[:idx], m.toolRunLineIdx[idx+1:]...)
	return lineIdx, job
}

// appendToolDoneLine renders a completed tool call as a compact card (claw-code style)
// and records it in the session tool log for Ctrl+T drill-down.
func (m *Model) appendToolDoneLine(toolName, summary, content string, isError bool) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	label := orchestrator.ToolFinishedPhrase(toolName)
	summaryBody := orchestrator.ToolCardSummaryBody(toolName, summary, content, isError)
	outcome := orchestrator.TranscriptOutcomeSnippet(toolName, content, isError)
	card := th.RenderToolCard(label, summaryBody, outcome, isError, m.width)
	m.lines = append(m.lines, card)
	m.appendToolCardMeta(toolName, summary, outcome, isError, content)
	m.setLinesContent(false)
	m.appendToToolLog(toolName, summary, content, isError, outcome)
}

// refreshAssistantLine updates or appends a line with streaming content.
// Uses curAssistantLineIdx to track which line we're updating.
func (m *Model) refreshAssistantLine() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	prefix := th.AssistantPrefix()
	rendered := fmt.Sprintf("%s %s", prefix, m.curAssistant.String())

	if m.curAssistantLineIdx >= 0 && m.curAssistantLineIdx < len(m.lines) {
		// We know exactly which line to update.
		m.lines[m.curAssistantLineIdx] = rendered
	} else {
		// Start a new assistant line.
		m.lines = append(m.lines, rendered)
		m.appendPlainMeta()
		m.curAssistantLineIdx = len(m.lines) - 1
		m.streamPaintAt = time.Time{}
		m.streamPaintSkip = 0
	}

	// While streaming, avoid joining the full transcript on every delta — that stutters on long sessions.
	const (
		streamViewportMin   = 32 * time.Millisecond
		streamViewportBurst = 512
	)
	now := time.Now()
	curLen := m.curAssistant.Len()
	if m.streaming && !m.streamPaintAt.IsZero() &&
		now.Sub(m.streamPaintAt) < streamViewportMin &&
		(curLen-m.streamPaintSkip) < streamViewportBurst {
		return
	}
	m.streamPaintAt = now
	m.streamPaintSkip = curLen
	m.setLinesContent(false)
}

func (m *Model) clearAssistantRevealState() {
	m.assistantRevealRunes = nil
	m.assistantRevealPos = 0
	m.pendingAssistantDoneSet = false
	m.pendingAssistantRaw = ""
}

func humanBytes(n int) string {
	if n <= 0 {
		return "0 B"
	}
	if n < 2048 {
		return fmt.Sprintf("%d B", n)
	}
	kb := float64(n) / 1024
	if kb < 1024 {
		if kb < 10 {
			return fmt.Sprintf("%.1f kB", kb)
		}
		return fmt.Sprintf("%.0f kB", kb)
	}
	return fmt.Sprintf("%.1f MB", kb/1024)
}

// refreshAssistantStreamDisplay paints either live tokens (legacy) or a hold placeholder row.
func (m *Model) refreshAssistantStreamDisplay() {
	if !assistantStreamHold {
		m.refreshAssistantLine()
		return
	}
	m.refreshAssistantHoldLine()
}

// refreshAssistantHoldLine shows a single assistant row while tokens accumulate in curAssistant only.
func (m *Model) refreshAssistantHoldLine() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	prefix := th.AssistantPrefix()
	n := m.curAssistant.Len()
	hint := "…"
	if n > 0 {
		hint = fmt.Sprintf("… %s", humanBytes(n))
	}
	rendered := fmt.Sprintf("%s %s", prefix, th.FooterDim.Render(hint))

	if m.curAssistantLineIdx >= 0 && m.curAssistantLineIdx < len(m.lines) {
		m.lines[m.curAssistantLineIdx] = rendered
	} else {
		m.lines = append(m.lines, rendered)
		m.appendPlainMeta()
		m.curAssistantLineIdx = len(m.lines) - 1
		m.assistantHoldLastPaint = time.Time{}
	}

	now := time.Now()
	if !m.assistantHoldLastPaint.IsZero() && now.Sub(m.assistantHoldLastPaint) < assistantHoldPaintMin && n > 0 {
		return
	}
	m.assistantHoldLastPaint = now
	m.setLinesContent(false)
}

func (m *Model) paintAssistantRevealPlain() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	prefix := th.AssistantPrefix()
	if m.curAssistantLineIdx < 0 || m.curAssistantLineIdx >= len(m.lines) {
		return
	}
	chunk := string(m.assistantRevealRunes[:m.assistantRevealPos])
	m.lines[m.curAssistantLineIdx] = fmt.Sprintf("%s %s", prefix, chunk)
	m.setLinesContent(false)
}

// applyAssistantDoneFollowup runs footer hints, git diff, and queue drain after the assistant segment is finalized.
func (m *Model) applyAssistantDoneFollowup(msg assistantDoneMsg, raw string, rawLen int) tea.Cmd {
	if rawLen >= idleTranscriptHintMinRunes {
		m.idleTranscriptHint = m.transcriptScrollNavHint()
	}
	if !msg.aborted && (strings.Contains(raw, orchestrator.TruthFooterMarkerEN) ||
		strings.Contains(raw, orchestrator.TruthFooterMarkerES)) {
		m.footerHint = "goclaw: no workspace writes — type continue, /continue, or a short follow-up if you still want edits."
	}
	if !msg.aborted && m.turnHadWorkspaceWrite {
		if block := gitdiff.WorktreeDiffStat(m.workdir); block != "" {
			m.appendSystem(block)
		}
	}
	m.turnHadWorkspaceWrite = false
	if !msg.aborted {
		return m.drainMessageQueue()
	}
	if n := len(m.messageQueue); n > 0 {
		m.messageQueue = nil
		m.appendSystem(fmt.Sprintf("(cleared %d queued message(s) after cancel)", n))
	}
	return nil
}

// normalizeAssistantMarkdownLines removes glamour's heavy left padding on wrapped
// paragraphs so we do not stack it on top of our gutter padding (looked like a staircase).
// Lines inside fenced code blocks are left unchanged.
func normalizeAssistantMarkdownLines(lines []string) []string {
	out := make([]string, len(lines))
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			out[i] = line
			continue
		}
		if inFence {
			out[i] = line
			continue
		}
		if i == 0 {
			out[i] = strings.TrimLeft(line, " \t")
			continue
		}
		// Continuation lines: strip runaway indentation from word-wrap, keep shallow list indents.
		n := countLeadingASCIISpaces(line)
		if n >= mdLineRunawayIndentMin {
			out[i] = strings.TrimLeft(line, " \t")
		} else {
			out[i] = line
		}
	}
	return out
}

func countLeadingASCIISpaces(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += mdTabIndentSpaces
		default:
			return n
		}
	}
	return n
}

// dropLeadingBlankLines removes empty / whitespace-only lines glamour often emits
// before the first paragraph (otherwise the prefix sits alone on its own row).
func dropLeadingBlankLines(lines []string) []string {
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i == 0 {
		return lines
	}
	return lines[i:]
}

// dropTrailingBlankLines removes empty lines Glamour often emits after the last paragraph.
func dropTrailingBlankLines(lines []string) []string {
	j := len(lines)
	for j > 0 && strings.TrimSpace(lines[j-1]) == "" {
		j--
	}
	if j == len(lines) {
		return lines
	}
	return lines[:j]
}

// finalizeCurrentSegment renders the current curAssistant buffer as markdown
// and replaces the streaming line with the formatted version. Called both
// before a tool use (to finalize pre-tool text) and on assistantDone.
func (m *Model) finalizeCurrentSegment() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	raw := m.curAssistant.String()
	if strings.TrimSpace(raw) == "" {
		return
	}

	prefix := th.AssistantPrefix()
	prefixW := lipgloss.Width(prefix)
	finalRendered := renderAssistantMarkdownSegment(th, raw, m.width, prefix, prefixW)
	if strings.TrimSpace(finalRendered) == "" {
		return
	}

	// Replace the tracked streaming line with the markdown version.
	if m.curAssistantLineIdx >= 0 && m.curAssistantLineIdx < len(m.lines) {
		m.lines[m.curAssistantLineIdx] = finalRendered
		m.setAssistantMDMetaAt(m.curAssistantLineIdx, raw)
	} else {
		m.lines = append(m.lines, finalRendered)
		m.appendAssistantMDMeta(raw)
	}
	m.curAssistantLineIdx = -1
	m.setLinesContent(false)
}

func stripANSI(s string) string {
	return ansi.Strip(s)
}
