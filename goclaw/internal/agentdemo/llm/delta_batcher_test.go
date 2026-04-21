package llm

import (
	"strings"
	"testing"
	"time"
)

func TestStreamDeltaBatcher_thresholdFlush(t *testing.T) {
	t.Parallel()
	var got []string
	b := NewStreamDeltaBatcher(time.Hour, 10, func(chunk string) {
		got = append(got, chunk)
	})
	// 10 bytes exactly triggers immediate flush
	b.Feed(strings.Repeat("a", 10))
	b.FlushRemaining()
	if len(got) != 1 || got[0] != strings.Repeat("a", 10) {
		t.Fatalf("got %#v", got)
	}
}

func TestStreamDeltaBatcher_flushRemaining(t *testing.T) {
	t.Parallel()
	var got []string
	b := NewStreamDeltaBatcher(time.Hour, 640, func(chunk string) {
		got = append(got, chunk)
	})
	b.Feed("hello")
	b.FlushRemaining()
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("got %#v", got)
	}
}
