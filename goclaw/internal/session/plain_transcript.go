package session

import (
	"fmt"
	"strings"
)

// PlainTranscript renders the in-memory session as plain UTF-8 text (roles, text,
// tool calls, and tool results). Suitable for /export and /copy.
func (s *Session) PlainTranscript() string {
	if s == nil || len(s.Messages) == 0 {
		return ""
	}
	var b strings.Builder
	for _, msg := range s.Messages {
		switch msg.Role {
		case "user", "assistant":
			body := strings.TrimSpace(msg.Content)
			if body != "" {
				fmt.Fprintf(&b, "[%s]\n%s\n\n", msg.Role, body)
			}
			for _, tc := range msg.ToolCalls {
				fmt.Fprintf(&b, "[%s tool_use %s]\n%s\n\n", msg.Role, tc.Name, strings.TrimSpace(tc.Input))
			}
			if len(msg.ToolResults) > 0 {
				b.WriteString("[user tool_results]\n")
				for _, tr := range msg.ToolResults {
					flag := ""
					if tr.IsError {
						flag = " (error)"
					}
					fmt.Fprintf(&b, "  %s%s:\n%s\n", tr.ToolName, flag, strings.TrimSpace(tr.Content))
				}
				b.WriteString("\n")
			}
		default:
			body := strings.TrimSpace(msg.Content)
			if body != "" {
				fmt.Fprintf(&b, "[%s]\n%s\n\n", msg.Role, body)
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
