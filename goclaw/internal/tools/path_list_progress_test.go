package tools

import (
	"context"
	"strings"
	"testing"
)

type sliceReporter struct {
	chunks []string
}

func (s *sliceReporter) OnProgress(_, partial string) {
	s.chunks = append(s.chunks, partial)
}

func TestPathListProgress_batchesByCount(t *testing.T) {
	t.Parallel()
	var rep sliceReporter
	ctx := context.Background()
	p := newPathListProgress(ctx, &rep)
	for i := range pathListProgressBatchLines + 2 {
		if err := p.addLine(strings.Repeat("x", 3) + string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	p.flush() // drain any remainder below batch size
	if len(rep.chunks) < 2 {
		t.Fatalf("expected at least 2 progress chunks, got %d", len(rep.chunks))
	}
}

func TestPathListProgress_respectsCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var rep sliceReporter
	p := newPathListProgress(ctx, &rep)
	err := p.addLine("only")
	if err == nil {
		t.Fatal("expected ctx cancellation")
	}
}

func TestPathListProgress_flushDrainsPending(t *testing.T) {
	t.Parallel()
	var rep sliceReporter
	ctx := context.Background()
	p := newPathListProgress(ctx, &rep)
	_ = p.addLine("one")
	p.flush()
	if len(rep.chunks) != 1 || !strings.Contains(rep.chunks[0], "one") {
		t.Fatalf("unexpected chunks: %#v", rep.chunks)
	}
}
