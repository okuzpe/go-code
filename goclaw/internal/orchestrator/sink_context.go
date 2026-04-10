package orchestrator

import "context"

// streamSinkCtxKey carries an optional StreamSink for nested runs (e.g. spawn_agent worker).
type streamSinkCtxKey struct{}

// ContextWithStreamSink returns ctx that exposes sink via StreamSinkFromContext.
// A nil sink is a no-op: the returned context equals ctx.
func ContextWithStreamSink(ctx context.Context, sink StreamSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, streamSinkCtxKey{}, sink)
}

// StreamSinkFromContext returns the sink stored by ContextWithStreamSink, or nil.
func StreamSinkFromContext(ctx context.Context) StreamSink {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(streamSinkCtxKey{}).(StreamSink)
	return v
}
