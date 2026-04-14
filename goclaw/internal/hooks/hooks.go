// Package hooks implements the event system for pre/post tool execution.
package hooks

import (
	"context"
	"log/slog"
)

// EventType identifies when a hook fires.
type EventType string

const (
	PreToolUse         EventType = "pre_tool_use"
	PostToolUse        EventType = "post_tool_use"
	PostToolUseFailure EventType = "post_tool_use_failure"
	SessionStart       EventType = "session_start"
	SessionEnd         EventType = "session_end"
)

// FailureKind values classify post_tool_use_failure (see Event.FailureKind).
const (
	// FailureExecuteError means Tool.Execute returned a non-nil error.
	FailureExecuteError = "execute_error"
	// FailureErrorResult means the tool ran but reported Result.IsError.
	FailureErrorResult = "error_result"
)

// Event carries context about what triggered the hook.
type Event struct {
	Type        EventType
	ToolName    string
	Input       string // JSON input passed to the tool
	// Output is the tool result body on post_tool_use / post_tool_use_failure.
	// When FailureKind is FailureExecuteError, it is whatever the tool returned
	// alongside the non-nil Execute error and may be empty or partial.
	Output      string
	FailureKind string // post_tool_use_failure only: FailureExecuteError or FailureErrorResult
}

// Handler processes a hook event. Returning an error blocks execution
// (PreToolUse only); PostToolUse / PostToolUseFailure errors are logged but not fatal.
type Handler func(ctx context.Context, e Event) error

// Registry holds per-event handler lists.
type Registry struct {
	handlers  map[EventType][]Handler
	cmdHooks  map[EventType][]externalCommand
	httpHooks map[EventType][]externalHTTP
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		handlers:  make(map[EventType][]Handler),
		cmdHooks:  make(map[EventType][]externalCommand),
		httpHooks: make(map[EventType][]externalHTTP),
	}
}

// On registers a handler for an event type.
func (r *Registry) On(event EventType, h Handler) {
	r.handlers[event] = append(r.handlers[event], h)
}

// Fire runs all handlers for the given event sequentially.
// For PreToolUse, it returns the first handler error and does not run external hooks afterward.
// For other event types, handler errors are logged with slog.WarnContext and execution continues;
// the returned error is not used for those handler failures. A non-nil error on post-tool or
// session events usually means setup failed before external hooks (for example JSON marshal
// in fireExternal); external hook failures are also logged for non-PreToolUse events.
func (r *Registry) Fire(ctx context.Context, e Event) error {
	for _, h := range r.handlers[e.Type] {
		err := h(ctx, e)
		if err == nil {
			continue
		}
		if e.Type == PreToolUse {
			return err
		}
		slog.WarnContext(ctx, "hook handler error", "event", string(e.Type), "tool", e.ToolName, "err", err)
	}
	return r.fireExternal(ctx, e)
}
