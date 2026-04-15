package app

import "github.com/okuzpe/goclaw/internal/orchestrator"

// NopStreamSink implements orchestrator.StreamSink with no-op handlers. Use it for
// automation output paths and tests that do not observe streaming events.
type NopStreamSink struct{}

func (NopStreamSink) OnTextDelta(string) {}

func (NopStreamSink) OnThinkingStart(string) {}

func (NopStreamSink) OnToolProgress(string, string) {}

func (NopStreamSink) OnToolUse(string, string, string) {}

func (NopStreamSink) OnToolResult(string, string, string, bool) {}

func (NopStreamSink) OnDone(string) {}

func (NopStreamSink) OnCompact(int) {}

var _ orchestrator.StreamSink = NopStreamSink{}
