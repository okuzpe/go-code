package orchestrator

import (
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
)

func TestTuiInteractModePromptBlock_inactive(t *testing.T) {
	if s := tuiInteractModePromptBlock(false, config.TUIInteractModeAgent); s != "" {
		t.Fatalf("expected empty when inactive, got %q", s)
	}
}

func TestTuiInteractModePromptBlock_modes(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want string
	}{
		{config.TUIInteractModeChat, "Terminal interact mode: chat"},
		{config.TUIInteractModeCode, "Terminal interact mode: code"},
		{config.TUIInteractModeAgent, "Terminal interact mode: agent"},
	} {
		got := tuiInteractModePromptBlock(true, tc.mode)
		if !strings.Contains(got, tc.want) {
			t.Fatalf("mode %q: want substring %q in %q", tc.mode, tc.want, got)
		}
	}
}
