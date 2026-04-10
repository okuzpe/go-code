package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

func TestWelcomeDashboardLines_fitsNarrowTerminal(t *testing.T) {
	th := DefaultTheme()
	opt := WelcomeOptions{
		Version:  "dev",
		Subtitle: "goclaw · ollama · qwen2.5-coder:14b · coordinator",
		Workdir:  "C:\\Users\\Example\\Desktop\\go-code\\goclaw",
	}
	termW := 52
	lines := WelcomeDashboardLines(th, opt, termW)
	require.NotEmpty(t, lines)
	m := maxLineWidth(lines)
	// Left rule + inner padding add up to two cells beyond wrapped content.
	require.LessOrEqual(t, m, termW+2, "no line much wider than terminal: %d", m)
	joined := strings.Join(lines, "\n")
	require.Contains(t, joined, "/capabilities", "slash command should stay readable")
}

func maxLineWidth(lines []string) int {
	m := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > m {
			m = w
		}
	}
	return m
}

func TestWelcomeDashboardLines_emptyVersion(t *testing.T) {
	require.Nil(t, WelcomeDashboardLines(DefaultTheme(), WelcomeOptions{}, 80))
}

func TestWelcomeDashboardLines_wrapsLongWorkdir(t *testing.T) {
	th := DefaultTheme()
	longPath := strings.Repeat("/very/long/segment", 6)
	opt := WelcomeOptions{Version: "1", Workdir: longPath}
	lines := WelcomeDashboardLines(th, opt, 72)
	require.NotEmpty(t, lines)
	m := maxLineWidth(lines)
	require.LessOrEqual(t, m, 72)
}

func TestWrapSubtitle_prefersBulletBreaks(t *testing.T) {
	sub := "goclaw · ollama · qwen2.5-coder:14b · coordinator"
	lines := wrapSubtitle(sub, 36)
	require.NotEmpty(t, lines)
	for _, ln := range lines {
		require.LessOrEqual(t, lipgloss.Width(ln), 36, "line %q", ln)
	}
}
