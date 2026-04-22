package ui

import (
	"context"
	"strings"

	corellm "github.com/okuzpe/goclaw/internal/llm"
)

// runAgentTurn runs the full agent loop (LLM → tools → LLM → …) via the
// real orchestrator and posts events back to the Bubble Tea model.
func (m *Model) runAgentTurn(ctx context.Context, userText string) {
	sink := &bubbleTeaSink{post: m.post}
	err := m.agentRunner.Run(ctx, userText, sink)
	m.post(streamDoneMsg{err: err})
}

// runLLMStream streams a plain chat response (no tools) using the main
// internal/llm.OllamaClient. Conversation history is kept in m.chatHistory
// so context carries across turns.
func (m *Model) runLLMStream(ctx context.Context, userText string) {
	m.chatHistory = append(m.chatHistory, corellm.PlainMessage("user", userText))

	client := corellm.NewOllama(m.cfg.OllamaHost)
	req := corellm.Request{
		Model:    m.cfg.Model,
		System:   "You are a concise coding assistant. Keep answers short for a terminal.",
		Messages: m.chatHistory,
	}

	batcher := corellm.NewStreamDeltaBatcher(0, 0, func(chunk string) {
		m.post(streamDeltaMsg{text: chunk})
	})

	events, errc := client.Stream(ctx, req)

	var assistantText strings.Builder
	for event := range events {
		if delta, ok := event.(corellm.TextDelta); ok {
			batcher.Feed(delta.Text)
			assistantText.WriteString(delta.Text)
		}
	}
	batcher.FlushRemaining()

	var streamErr error
	select {
	case err := <-errc:
		streamErr = err
	default:
	}

	if streamErr == nil && assistantText.Len() > 0 {
		m.chatHistory = append(m.chatHistory, corellm.PlainMessage("assistant", assistantText.String()))
	}

	m.post(streamDoneMsg{err: streamErr})
}
