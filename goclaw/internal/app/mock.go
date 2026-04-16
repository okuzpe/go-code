package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
)

// Mock stream pacing (production defaults). Tests set to zero for speed (see app_test.go).
var (
	mockStreamInitialDelay = 380 * time.Millisecond
	mockStreamRuneDelay    = 11 * time.Millisecond
)

func effectiveMockStreamDelays() (initial, perRune time.Duration) {
	if strings.TrimSpace(os.Getenv("GOCLAW_MOCK_FAST")) == "1" {
		return 0, 0
	}
	return mockStreamInitialDelay, mockStreamRuneDelay
}

func shortSessionID(id string) string {
	if len(id) > 12 {
		return id[:12] + "…"
	}
	return id
}

func mockAssistantReplyBody(userText string) string {
	return fmt.Sprintf(
		"[mock] Canned reply from goclaw. Omit --mock and use a running Ollama server for real answers.\n\nYou said:\n%s",
		userText,
	)
}

// StreamMockAssistant streams a canned assistant reply for --mock (TUI or JSON automation).
func StreamMockAssistant(ctx context.Context, userText string, sink orchestrator.StreamSink, sess *session.Session) (string, error) {
	initialDelay, runeDelay := effectiveMockStreamDelays()
	if initialDelay > 0 {
		select {
		case <-time.After(initialDelay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	} else if err := ctx.Err(); err != nil {
		return "", err
	}
	reply := mockAssistantReplyBody(userText)
	for _, ch := range reply {
		sink.OnTextDelta(string(ch))
		if runeDelay > 0 {
			select {
			case <-time.After(runeDelay):
			case <-ctx.Done():
				sink.OnDone("")
				return reply, ctx.Err()
			}
		} else if err := ctx.Err(); err != nil {
			sink.OnDone("")
			return reply, err
		}
	}
	sink.OnDone("")
	sess.Add("user", userText)
	sess.AddAssistant(reply, nil)
	return reply, nil
}
