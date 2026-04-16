package chat

import (
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// Batching reduces tea.Send / viewport refresh churn during LLM streaming.
// Slightly longer window + larger bursts feel smoother than one tiny Msg per token.
const (
	deltaBatchInterval = 42 * time.Millisecond
	deltaBatchMaxBytes = 640
)

// batchedProgramSink coalesces OnTextDelta calls before forwarding to the TUI.
type batchedProgramSink struct {
	p               *tea.Program
	mu              sync.Mutex
	buf             strings.Builder
	timer           *time.Timer
	thinkingStarted bool // true after the first OnThinkingStart — first call is handled by assistantPlaceholderMsg
}

func newBatchedProgramSink(p *tea.Program) *batchedProgramSink {
	return &batchedProgramSink{p: p}
}

func (s *batchedProgramSink) OnTextDelta(text string) {
	if text == "" {
		return
	}
	s.mu.Lock()
	s.buf.WriteString(text)
	flushNow := s.buf.Len() >= deltaBatchMaxBytes
	s.mu.Unlock()
	if flushNow {
		s.flush()
		return
	}
	s.scheduleFlush()
}

func (s *batchedProgramSink) scheduleFlush() {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(deltaBatchInterval, func() { s.flush() })
	s.mu.Unlock()
}

func (s *batchedProgramSink) flush() {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	chunk := s.buf.String()
	s.buf.Reset()
	s.mu.Unlock()
	if chunk != "" {
		s.p.Send(assistantDeltaMsg(chunk))
	}
}

func (s *batchedProgramSink) OnThinkingStart(phase string) {
	s.mu.Lock()
	first := !s.thinkingStarted
	s.thinkingStarted = true
	s.mu.Unlock()
	s.p.Send(thinkingPhaseMsg{phase: phase})
	if first {
		// First iteration: assistantPlaceholderMsg (sent before submit()) already set up
		// the thinking row; thinkingPhaseMsg updates its label when the orchestrator resolves the phase.
		return
	}
	// Subsequent iterations (between tool calls): re-show the thinking row.
	s.flush()
	s.p.Send(thinkingRestartMsg{phase: phase})
}

func (s *batchedProgramSink) OnToolProgress(name, partial string) {
	s.p.Send(toolProgressMsg{name: name, partial: partial})
}

func (s *batchedProgramSink) OnToolUse(toolUseID, name, rawInput string) {
	s.flush()
	preview := orchestrator.FormatToolUsePreview(name, rawInput)
	s.p.Send(toolUseMsg{toolUseID: toolUseID, name: name, preview: preview})
}

func (s *batchedProgramSink) OnToolResult(toolUseID, name string, content string, isError bool) {
	s.flush()
	s.p.Send(toolResultMsg{toolUseID: toolUseID, name: name, content: content, isError: isError})
}

func (s *batchedProgramSink) OnDone(_ string) {
	s.flush()
	s.p.Send(assistantDoneMsg{aborted: false})
}

func (s *batchedProgramSink) OnCompact(removed int) {
	if removed <= 0 {
		return
	}
	s.p.Send(compactNoticeMsg{removed: removed})
}
