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

// Event carries context about what triggered the hook.
type Event struct {
	Type     EventType
	ToolName string
	Input    string // JSON input passed to the tool
	Output   string // JSON output (PostToolUse only)
}

// Handler processes a hook event. Returning an error blocks execution
// (PreToolUse only); PostToolUse errors are logged but not fatal.
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
// Returns the first blocking error (PreToolUse only).
// PostToolUse / PostToolUseFailure handler errors are logged and ignored.
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
