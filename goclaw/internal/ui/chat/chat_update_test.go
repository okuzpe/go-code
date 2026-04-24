package chat

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestMouseWheelOverShortComposeFallsBackToTranscript(t *testing.T) {
	t.Parallel()

	m := New(context.Background(), Options{
		Theme:          DefaultTheme(),
		TUIMouseScroll: true,
	})
	m.width = 72
	m.height = 16
	for i := 0; i < 40; i++ {
		m.appendSystem("line")
	}
	m.input.SetValue("draft")
	m.resizeInput()
	m.layout()
	m.viewport.GotoTop()

	before := m.viewport.YOffset()
	composeY := m.height - 1
	_, cmd := m.Update(tea.MouseWheelMsg{
		X:      2,
		Y:      composeY,
		Button: tea.MouseWheelDown,
	})
	require.Nil(t, cmd)
	require.Greater(t, m.viewport.YOffset(), before)
}
