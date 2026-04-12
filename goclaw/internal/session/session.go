// Package session manages conversation history and JSONL persistence.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"

	"github.com/okuzpe/goclaw/internal/llm"
)

// Session holds the in-memory conversation history for one user session.
type Session struct {
	ID       string
	Messages []llm.Message
	// streamingAssistChars is UTF-8 byte count of the assistant reply currently streaming
	// (not yet committed to Messages). Used for live token/compact hints in the TUI.
	streamingAssistChars int64
}

// New creates a fresh session with a cryptographically random ID.
func New() *Session {
	return &Session{
		ID:       newID(),
		Messages: make([]llm.Message, 0, 32),
	}
}

// Add appends a plain text message to the history.
func (s *Session) Add(role, content string) {
	s.Messages = append(s.Messages, llm.PlainMessage(role, content))
}

// AddAssistant appends an assistant turn, optionally including tool calls from the model.
func (s *Session) AddAssistant(text string, toolCalls []llm.ToolCallRecord) {
	s.ClearStreamingAssistant()
	s.Messages = append(s.Messages, llm.Message{
		Role:      "assistant",
		Content:   text,
		ToolCalls: toolCalls,
	})
}

// AddToolResults appends a user turn that carries tool_result blocks for the API.
func (s *Session) AddToolResults(results []llm.ToolResultRecord) {
	if len(results) == 0 {
		return
	}
	s.Messages = append(s.Messages, llm.Message{
		Role:        "user",
		ToolResults: results,
	})
}

// Len returns the number of messages in the session.
func (s *Session) Len() int { return len(s.Messages) }

// ReplaceMessages replaces the full message history (used by compaction only).
// Callers should prefer the Add* methods for normal appends.
func (s *Session) ReplaceMessages(msgs []llm.Message) {
	s.ClearStreamingAssistant()
	s.Messages = msgs
}

// PrependMessage inserts a single message at the front of the history.
// Used by compaction to add a summary before the preserved tail.
func (s *Session) PrependMessage(msg llm.Message) {
	s.ClearStreamingAssistant()
	s.Messages = append([]llm.Message{msg}, s.Messages...)
}

// AddStreamingAssistantChars adds n bytes (UTF-8) to the in-flight assistant stream estimate.
func (s *Session) AddStreamingAssistantChars(n int) {
	if s == nil || n <= 0 {
		return
	}
	atomic.AddInt64(&s.streamingAssistChars, int64(n))
}

// ClearStreamingAssistant resets the in-flight assistant byte counter (e.g. after cancel or commit).
func (s *Session) ClearStreamingAssistant() {
	if s == nil {
		return
	}
	atomic.StoreInt64(&s.streamingAssistChars, 0)
}

// StreamingAssistantChars returns in-flight assistant UTF-8 bytes not yet in Messages.
func (s *Session) StreamingAssistantChars() int {
	if s == nil {
		return 0
	}
	n := atomic.LoadInt64(&s.streamingAssistChars)
	if n < 0 {
		return 0
	}
	return int(n)
}

// newID returns a 16-byte hex-encoded random identifier.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is unrecoverable in practice.
		panic("session: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
