package tools

import (
	"context"
	"strings"
	"time"
)

// Throttling for streaming path/list lines to the UI (orchestrator → StreamSink.OnToolProgress).
const (
	pathListProgressMinInterval = 75 * time.Millisecond
	pathListProgressBatchLines  = 14
)

// pathListProgress batches newline-separated snippets for ProgressReporter (glob paths, grep hits).
type pathListProgress struct {
	ctx      context.Context
	reporter ProgressReporter
	pending  []string
	lastEmit time.Time
}

func newPathListProgress(ctx context.Context, r ProgressReporter) *pathListProgress {
	if r == nil {
		return nil
	}
	return &pathListProgress{
		ctx:      ctx,
		reporter: r,
		lastEmit: time.Now().Add(-pathListProgressMinInterval),
		pending:  nil,
	}
}

// addLine queues one display line (single path or grep hit). Respects ctx cancellation.
func (p *pathListProgress) addLine(line string) error {
	if p == nil {
		return nil
	}
	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	default:
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	p.pending = append(p.pending, line)
	p.maybeFlush(false)
	return nil
}

// flush sends any remaining lines (call when the tool finishes).
func (p *pathListProgress) flush() {
	if p == nil {
		return
	}
	p.flushPending()
}

func (p *pathListProgress) flushPending() {
	if p == nil || len(p.pending) == 0 {
		return
	}
	p.reporter.OnProgress("", strings.Join(p.pending, "\n"))
	p.pending = p.pending[:0]
	p.lastEmit = time.Now()
}

func (p *pathListProgress) maybeFlush(force bool) {
	if p == nil || len(p.pending) == 0 {
		return
	}
	if !force && len(p.pending) < pathListProgressBatchLines && time.Since(p.lastEmit) < pathListProgressMinInterval {
		return
	}
	p.flushPending()
}
