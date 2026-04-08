package llm

import "encoding/json"

// messageToOllama expands one logical Message into one or more Ollama /api/chat messages.
func messageToOllama(m Message) []ollamaMessage {
	if m.Role == "assistant" && len(m.ToolCalls) > 0 {
		tcalls := make([]ollamaToolCallWire, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			args := json.RawMessage(tc.Input)
			if tc.Input == "" {
				args = json.RawMessage("{}")
			}
			var w ollamaToolCallWire
			w.Type = "function"
			w.Function.Name = tc.Name
			w.Function.Arguments = args
			tcalls = append(tcalls, w)
		}
		return []ollamaMessage{{
			Role:      "assistant",
			Content:   m.Content,
			ToolCalls: tcalls,
		}}
	}
	if m.Role == "user" && len(m.ToolResults) > 0 {
		var out []ollamaMessage
		if m.Content != "" {
			out = append(out, ollamaMessage{Role: "user", Content: m.Content})
		}
		for _, tr := range m.ToolResults {
			out = append(out, ollamaMessage{
				Role:     "tool",
				ToolName: tr.ToolName,
				Content:  tr.Content,
			})
		}
		return out
	}
	return []ollamaMessage{{Role: m.Role, Content: m.Content}}
}
