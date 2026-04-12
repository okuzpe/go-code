package chat

import (
	"context"
	"testing"
)

func TestFirstLayoutKeepsWelcomeScrollAtTop(t *testing.T) {
	th := DefaultTheme()
	m := New(context.Background(), Options{
		Theme: th,
		Welcome: WelcomeOptions{
			Version:  "0.0.1",
			Subtitle: "ollama · test · profile",
			Workdir:  "/tmp",
			Profile:  "general",
		},
	})
	if m.welcomeBlockEnd == 0 {
		t.Fatal("expected welcome dashboard lines")
	}
	m.width = 72
	m.height = 18
	m.layout()

	if m.viewport.PastBottom() {
		t.Fatalf("unexpected PastBottom after first layout (y=%d)", m.viewport.YOffset())
	}
	if m.viewport.YOffset() != 0 {
		t.Fatalf("first paint should start at top of transcript (welcome), got YOffset=%d", m.viewport.YOffset())
	}
}
