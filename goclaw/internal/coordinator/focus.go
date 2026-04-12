package coordinator

import (
	"fmt"
	"strings"
	"sync"

	"github.com/okuzpe/goclaw/internal/text"
)

const (
	focusHintFocusedMaxRunes          = 12
	focusHintBackgroundPrefixMaxRunes = 8
	focusHintBackgroundLabelMaxRunes  = 10
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

// Hint returns a short footer line for TUI: worker focus, or parent when background workers exist.
func (f *FocusRouter) Hint() string {
	if f == nil {
		return ""
	}
	f.mu.Lock()
	current := f.taskID
	f.mu.Unlock()

	if current != "" {
		short := text.TruncateRunes(current, focusHintFocusedMaxRunes)
		return "Inside worker " + short + " — /back or /detach returns to coordinator"
	}

	list := ListInteractiveWorkers()
	n := len(list)
	if n == 0 {
		return ""
	}
	if n == 1 {
		w := list[0]
		prefix := text.TruncateRunesHard(strings.TrimSpace(w.TaskID), focusHintBackgroundPrefixMaxRunes)
		short := text.TruncateRunes(w.TaskID, focusHintBackgroundLabelMaxRunes)
		return fmt.Sprintf("Background worker %s — /focus %s · /workers", short, prefix)
	}
	return fmt.Sprintf("%d background workers — /workers · /focus <id prefix>", n)
}
