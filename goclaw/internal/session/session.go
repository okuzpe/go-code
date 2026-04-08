// Package session manages conversation history and JSONL persistence.
package session

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/okuzpe/goclaw/internal/llm"
)

// Session holds the in-memory conversation history for one user session.
type Session struct {
	ID       string
	Messages []llm.Message
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

// newID returns a 16-byte hex-encoded random identifier.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is unrecoverable in practice.
		panic("session: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
