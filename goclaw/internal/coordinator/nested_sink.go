package coordinator

import "github.com/okuzpe/goclaw/internal/orchestrator"

// nestedWorkerStreamSink forwards worker streaming events to the parent UI but drops OnDone.
// The parent orchestrator turn ends when spawn_agent returns; forwarding OnDone would
// prematurely finalize the assistant segment in the TUI while the tool is still running.
type nestedWorkerStreamSink struct {
	inner orchestrator.StreamSink
}

func wrapNestedWorkerSink(inner orchestrator.StreamSink) orchestrator.StreamSink {
	if inner == nil {
		return nil
	}
	return &nestedWorkerStreamSink{inner: inner}
}

func (n *nestedWorkerStreamSink) OnTextDelta(text string) {
	n.inner.OnTextDelta(text)
}

func (n *nestedWorkerStreamSink) OnToolUse(name, inputPreview string) {
	n.inner.OnToolUse(name, inputPreview)
}

func (n *nestedWorkerStreamSink) OnToolResult(name string, content string, isError bool) {
	n.inner.OnToolResult(name, content, isError)
}

func (n *nestedWorkerStreamSink) OnDone(string) {}

func (n *nestedWorkerStreamSink) OnCompact(int) {} // compaction is internal; do not surface to parent

var _ orchestrator.StreamSink = (*nestedWorkerStreamSink)(nil)
