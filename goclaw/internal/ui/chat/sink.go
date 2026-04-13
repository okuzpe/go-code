package chat

import (
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// Batching reduces tea.Send / viewport refresh churn during LLM streaming.
// Tune interval and byte threshold for terminal feel vs latency.
const (
	deltaBatchInterval = 28 * time.Millisecond
	deltaBatchMaxBytes = 384
)

// batchedProgramSink coalesces OnTextDelta calls before forwarding to the TUI.
type batchedProgramSink struct {
	p     *tea.Program
	mu    sync.Mutex
	buf   strings.Builder
	timer *time.Timer
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

func (s *batchedProgramSink) OnToolUse(name, rawInput string) {
	s.flush()
	preview := orchestrator.FormatToolUsePreview(name, rawInput)
	s.p.Send(toolUseMsg{name: name, preview: preview})
}

func (s *batchedProgramSink) OnToolResult(name string, content string, isError bool) {
	s.flush()
	s.p.Send(toolResultMsg{name: name, content: content, isError: isError})
}

func (s *batchedProgramSink) OnDone(_ string) {
	s.flush()
	s.p.Send(assistantDoneMsg{aborted: false})
}

func (s *batchedProgramSink) OnCompact(removed int) {
	s.p.Send(compactNoticeMsg{removed: removed})
}
