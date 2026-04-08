// Package llm provides HTTP clients for Anthropic and Ollama APIs.
package llm

import "context"

// Request is the payload sent to the LLM.
type Request struct {
	Model    string
	System   string
	Messages []Message
	Tools    []ToolSpec
	MaxTokens int
}

// ToolSpec describes a tool exposed to the model.
type ToolSpec struct {
	Name        string
	Description string
	InputSchema any
}

// Event is emitted by the streaming response.
type Event interface{ isEvent() }

type TextDelta struct{ Text string }
type ToolUse   struct{ ID, Name, Input string }
type Usage     struct{ InputTokens, OutputTokens int }
type Done      struct{}

func (TextDelta) isEvent() {}
func (ToolUse) isEvent()   {}
func (Usage) isEvent()     {}
func (Done) isEvent()      {}

// Client sends requests to an LLM provider and streams back events.
type Client interface {
	Stream(ctx context.Context, req Request) (<-chan Event, <-chan error)
}
