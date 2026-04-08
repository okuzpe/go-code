package llm

// ToolCallRecord is an assistant-side tool invocation stored in session history.
type ToolCallRecord struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"` // JSON object as string
}

// ToolResultRecord is a tool execution outcome sent back to the model (user role in Anthropic).
// ToolName is used for Ollama "tool" role messages (function name).
type ToolResultRecord struct {
	ToolUseID string `json:"tool_use_id"`
	ToolName  string `json:"tool_name,omitempty"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Message represents one turn in the conversation.
// Simple text: set Role and Content only.
// Assistant with tools: set ToolCalls (and optional Content for preceding text).
// User with tool results: set ToolResults (Anthropic-style user message with tool_result blocks).
type Message struct {
	Role        string             `json:"role"`
	Content     string             `json:"content,omitempty"`
	ToolCalls   []ToolCallRecord   `json:"tool_calls,omitempty"`
	ToolResults []ToolResultRecord `json:"tool_results,omitempty"`
}

// PlainMessage returns a simple user/assistant text message.
func PlainMessage(role, content string) Message {
	return Message{Role: role, Content: content}
}
