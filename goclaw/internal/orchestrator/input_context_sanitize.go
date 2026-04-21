package orchestrator

import "strings"

const (
	inlineAtContextStart = "[Files loaded via @ references]"
	inlineAtContextEnd   = "[End of @ context]"
)

// routingUserMessage strips injected inline @ context blocks and returns the original
// user instruction for intent/routing decisions.
func routingUserMessage(userMessage string) string {
	trimmed := strings.TrimSpace(userMessage)
	if !strings.HasPrefix(trimmed, inlineAtContextStart) {
		return userMessage
	}
	endIndex := strings.Index(trimmed, inlineAtContextEnd)
	if endIndex < 0 {
		return userMessage
	}
	after := strings.TrimSpace(trimmed[endIndex+len(inlineAtContextEnd):])
	if after == "" {
		return userMessage
	}
	return after
}
