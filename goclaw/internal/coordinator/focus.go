package coordinator

import (
	"strings"
	"sync"
)

// FocusRouter tracks whether user input is routed to the coordinator or an interactive worker.
type FocusRouter struct {
	mu     sync.Mutex
	taskID string // empty = parent (coordinator)
}

// NewFocusRouter returns a router defaulting to the parent session.
func NewFocusRouter() *FocusRouter {
	return &FocusRouter{}
}

// Current returns the focused interactive worker task_id, or empty for the coordinator.
func (f *FocusRouter) Current() string {
	if f == nil {
		return ""
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.taskID
}

// FocusTaskID sets focus to a full worker task id (must exist as interactive worker).
func (f *FocusRouter) FocusTaskID(full string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.taskID = strings.TrimSpace(full)
	f.mu.Unlock()
}

// Detach returns input routing to the coordinator.
func (f *FocusRouter) Detach() {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.taskID = ""
	f.mu.Unlock()
}

// Hint returns a short footer line for TUI/readline, or empty when focused on parent.
func (f *FocusRouter) Hint() string {
	if f == nil {
		return ""
	}
	id := f.Current()
	if id == "" {
		return ""
	}
	short := id
	if len(short) > 12 {
		short = short[:12] + "…"
	}
	return "Focus: worker " + short + " — /detach returns to coordinator"
}
