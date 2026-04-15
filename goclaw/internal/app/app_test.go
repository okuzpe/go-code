package app

import (
	"context"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/stretchr/testify/require"
)

func TestFormatChatWindowTitle(t *testing.T) {
	longModel := strings.Repeat("m", 50)
	got := FormatChatWindowTitle("ollama", longModel, "explore")
	require.True(t, strings.HasPrefix(got, "goclaw  ollama/"), "unexpected prefix: %q", got)
	require.True(t, strings.Contains(got, "…"), "expected truncated model in compact title: %q (len=%d)", got, len(got))
	require.LessOrEqual(t, len(got), 130, "expected compact title length cap: %q (len=%d)", got, len(got))
}

func TestReplHistoryFile(t *testing.T) {
	dir := t.TempDir()
	got := replHistoryFile(dir)
	require.True(t, strings.HasSuffix(got, "history"), got)
	require.Contains(t, got, dir)
}

func TestShortSessionID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"empty", "", ""},
		{"short", "abc", "abc"},
		{"twelve", "123456789012", "123456789012"},
		{"thirteen", "1234567890123", "123456789012…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shortSessionID(tt.id))
		})
	}
}

func TestMockAssistantReplyBody(t *testing.T) {
	body := mockAssistantReplyBody("ping")
	require.Contains(t, body, "ping")
	require.Contains(t, body, "[mock]")
}

func TestReplPrompt(t *testing.T) {
	require.Equal(t, "> ", replPrompt(nil, nil))

	s := &session.Session{ID: "abcd"}
	require.Equal(t, "abcd> ", replPrompt(s, nil))

	s.ID = "123456789"
	require.Equal(t, "12345678> ", replPrompt(s, nil))

	f := coordinator.NewFocusRouter()
	f.FocusTaskID("deadbeefcafe")
	require.Equal(t, "12345678@wdeadbeef> ", replPrompt(s, f))
}

func TestStreamMockAssistant_cancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sess := session.New()
	_, err := StreamMockAssistant(ctx, "x", NopStreamSink{}, sess)
	require.Error(t, err)
	require.Contains(t, err.Error(), context.Canceled.Error())
	require.Equal(t, 0, sess.Len())
}

func withZeroMockStreamDelays(t *testing.T) {
	t.Helper()
	origI, origR := mockStreamInitialDelay, mockStreamRuneDelay
	mockStreamInitialDelay, mockStreamRuneDelay = 0, 0
	t.Cleanup(func() {
		mockStreamInitialDelay, mockStreamRuneDelay = origI, origR
	})
}

func TestStreamMockAssistant_recordsTurns(t *testing.T) {
	withZeroMockStreamDelays(t)
	ctx := context.Background()
	sess := session.New()
	reply, err := StreamMockAssistant(ctx, "hi", NopStreamSink{}, sess)
	require.NoError(t, err)
	require.Contains(t, reply, "hi")
	require.Equal(t, 2, sess.Len(), "want 2 messages (user + assistant)")
}

type cancelAfterDeltas struct {
	NopStreamSink
	n      int
	limit  int
	cancel context.CancelFunc
}

func (c *cancelAfterDeltas) OnTextDelta(s string) {
	c.n += len(s)
	if c.n >= c.limit {
		c.cancel()
	}
}

func TestStreamMockAssistant_cancelMidStream(t *testing.T) {
	withZeroMockStreamDelays(t)
	ctx, cancel := context.WithCancel(context.Background())
	sess := session.New()
	sink := &cancelAfterDeltas{cancel: cancel, limit: 8}
	_, err := StreamMockAssistant(ctx, strings.Repeat("x", 400), sink, sess)
	require.Error(t, err)
	require.Contains(t, err.Error(), context.Canceled.Error())
}
