package tools

import "context"

// ProgressReporter receives incremental partial output from a running tool.
// It is stored in the execution context to avoid an import cycle between
// internal/tools and internal/orchestrator.
type ProgressReporter interface {
	// OnProgress is called with each partial chunk (e.g. a line of bash stdout).
	// name is the tool name; partial is the raw text chunk.
	OnProgress(name, partial string)
}

type progressKey struct{}

// WithProgressReporter returns a copy of ctx carrying the given ProgressReporter.
func WithProgressReporter(ctx context.Context, r ProgressReporter) context.Context {
	return context.WithValue(ctx, progressKey{}, r)
}

// ProgressReporterFromContext retrieves the ProgressReporter from ctx.
// Returns nil when none was stored.
func ProgressReporterFromContext(ctx context.Context) ProgressReporter {
	r, _ := ctx.Value(progressKey{}).(ProgressReporter)
	return r
}
