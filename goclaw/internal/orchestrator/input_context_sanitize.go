package orchestrator

import "strings"

const (
	InlineAtContextStart = "[Files loaded via @ references]"
	InlineAtContextEnd   = "[End of @ context]"
)

// routingUserMessage strips injected inline @ context blocks and returns the original
// user instruction for intent/routing decisions.
func routingUserMessage(userMessage string) string {
	trimmed := strings.TrimSpace(userMessage)
	if !strings.HasPrefix(trimmed, InlineAtContextStart) {
		return userMessage
	}
	endIndex := strings.Index(trimmed, InlineAtContextEnd)
	if endIndex < 0 {
		return userMessage
	}
	after := strings.TrimSpace(trimmed[endIndex+len(InlineAtContextEnd):])
	if after == "" {
		return userMessage
	}
	return after
}
