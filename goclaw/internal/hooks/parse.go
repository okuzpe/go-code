package hooks

import (
	"fmt"
	"strings"
)

// ParseEventType maps settings / JSON strings to EventType.
func ParseEventType(s string) (EventType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pre_tool_use":
		return PreToolUse, nil
	case "post_tool_use":
		return PostToolUse, nil
	case "post_tool_use_failure":
		return PostToolUseFailure, nil
	case "session_start":
		return SessionStart, nil
	case "session_end":
		return SessionEnd, nil
	default:
		return "", fmt.Errorf("hooks: unknown event type %q", s)
	}
}
