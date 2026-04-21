package ui

import (
	"context"
	"strings"

	"github.com/okuzpe/goclaw/internal/agentdemo/llm"
)

func (m *Model) runLLMStream(ctx context.Context, userText string) {
	base := strings.TrimSpace(m.cfg.OllamaHost)
	modelName := strings.TrimSpace(m.cfg.Model)
	msgs := []llm.ChatMessage{
		{Role: "system", Content: "You are a concise coding assistant. Keep answers short for a terminal demo."},
		{Role: "user", Content: userText},
	}
	b := llm.NewStreamDeltaBatcher(0, 0, func(chunk string) {
		if strings.TrimSpace(chunk) == "" {
			return
		}
		m.post(streamDeltaMsg{text: chunk})
	})
	err := llm.StreamChat(ctx, base, modelName, msgs, func(delta string) error {
		if delta == "" {
			return nil
		}
		b.Feed(delta)
		return nil
	})
	b.FlushRemaining()
	m.post(streamDoneMsg{err: err})
}
