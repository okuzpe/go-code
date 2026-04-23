package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeaderView_longModelLabelKeepsBrandVisible(t *testing.T) {
	t.Parallel()
	m := &Model{
		theme:      NewThemeForAppearance("cyberpunk"),
		width:      28,
		modelLabel: "ollama/some-very-long-model-name-with-many-segments:latest",
	}

	got := m.headerView()
	plain := stripANSI(got)

	require.Contains(t, plain, "GOCLAW")
	require.LessOrEqual(t, len([]rune(plain)), 32)
}

func TestRenderDocOverlay_includesTitleAndBody(t *testing.T) {
	t.Parallel()
	m := &Model{
		theme:            DefaultTheme(),
		width:            80,
		docOverlayTitle:  "Help",
		docOverlaySourceMD: "## Commands\n\nUse `/help`.",
	}

	got := m.renderDocOverlay()
	plain := stripANSI(got)

	require.Contains(t, plain, "Help")
	require.Contains(t, plain, "Commands")
	require.True(t, strings.Index(plain, "Help") < strings.Index(plain, "Commands"))
}

func TestFooterContextLine_defaultsToSingleInputFirstHint(t *testing.T) {
	t.Parallel()
	m := New(context.Background(), Options{Theme: DefaultTheme()})
	m.width = 80

	got := m.footerContextLine(80)
	plain := stripANSI(got)

	require.Contains(t, plain, "/ commands")
	require.Contains(t, plain, "@ files")
	require.Contains(t, plain, "Ctrl+P profile")
	require.Contains(t, plain, "Ctrl+T tools")
}

func TestSlashSuggestStripView_isCompactSingleLine(t *testing.T) {
	t.Parallel()
	m := New(context.Background(), Options{Theme: DefaultTheme()})
	m.width = 80
	m.input.SetValue("/")

	got := m.slashSuggestStripView()
	plain := stripANSI(got)

	require.NotEmpty(t, plain)
	require.NotContains(t, plain, "\n")
	require.Contains(t, plain, "Commands")
	require.Contains(t, plain, "/agents")
}

func TestMessageQueueStripView_isCompactSingleLine(t *testing.T) {
	t.Parallel()
	m := New(context.Background(), Options{Theme: DefaultTheme()})
	m.width = 80
	m.messageQueue = []string{
		"/profile plan and review the current repo state carefully",
		"second queued message",
	}

	got := m.messageQueueStripView()
	plain := stripANSI(got)

	require.NotEmpty(t, plain)
	require.NotContains(t, plain, "\n")
	require.Contains(t, plain, "Queued")
	require.Contains(t, plain, "2 messages")
}
