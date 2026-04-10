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
	if n.inner != nil && text != "" {
		n.inner.OnTextDelta(text)
	}
}

func (n *nestedWorkerStreamSink) OnToolUse(name, inputPreview string) {
	if n.inner != nil {
		n.inner.OnToolUse(name, inputPreview)
	}
}

func (n *nestedWorkerStreamSink) OnToolResult(name string, resultBytes int, isError bool) {
	if n.inner != nil {
		n.inner.OnToolResult(name, resultBytes, isError)
	}
}

func (n *nestedWorkerStreamSink) OnDone(string) {}

var _ orchestrator.StreamSink = (*nestedWorkerStreamSink)(nil)
