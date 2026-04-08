package app

import (
	"context"
	"fmt"
	"time"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
)

func shortSessionID(id string) string {
	if len(id) > 12 {
		return id[:12] + "…"
	}
	return id
}

func mockAssistantReplyBody(userText string) string {
	return fmt.Sprintf(
		"[mock] Canned reply from goclaw. Omit --mock and use a running provider (Ollama or Anthropic) for real answers.\n\nYou said:\n%s",
		userText,
	)
}

func streamMockAssistant(ctx context.Context, userText string, sink orchestrator.StreamSink, sess *session.Session) (string, error) {
	select {
	case <-time.After(380 * time.Millisecond):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	reply := mockAssistantReplyBody(userText)
	const tick = 11 * time.Millisecond
	for _, ch := range reply {
		sink.OnTextDelta(string(ch))
		select {
		case <-time.After(tick):
		case <-ctx.Done():
			sink.OnDone("")
			return reply, ctx.Err()
		}
	}
	sink.OnDone("")
	sess.Add("user", userText)
	sess.AddAssistant(reply, nil)
	return reply, nil
}

