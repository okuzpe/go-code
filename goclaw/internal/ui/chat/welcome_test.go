package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

func TestWelcomeDashboardLines_fitsVeryNarrowTerminal(t *testing.T) {
	t.Parallel()
	th := DefaultTheme()
	opt := WelcomeOptions{
		Version:  "dev",
		Subtitle: "goclaw · model",
		Workdir:  "C:\\proj",
	}
	termW := 28
	lines := WelcomeDashboardLines(th, opt, termW)
	require.NotEmpty(t, lines)
	m := maxLineWidth(lines)
	require.LessOrEqual(t, m, termW+4, "frame should not exceed terminal by more than border padding: got %d for term %d", m, termW)
}

func TestWelcomeDashboardLines_fitsNarrowTerminal(t *testing.T) {
	th := DefaultTheme()
	opt := WelcomeOptions{
		Version:  "dev",
		Subtitle: "goclaw · ollama · qwen2.5-coder:14b · general-purpose",
		Workdir:  "C:\\Users\\Example\\Desktop\\go-code\\goclaw",
	}
	termW := 52
	lines := WelcomeDashboardLines(th, opt, termW)
	require.NotEmpty(t, lines)
	m := maxLineWidth(lines)
	// Left rule + inner padding add up to two cells beyond wrapped content.
	require.LessOrEqual(t, m, termW+4, "no line much wider than terminal: %d", m)
	joined := strings.Join(lines, "\n")
	require.Contains(t, joined, "/help", "slash command should stay readable")
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
	require.LessOrEqual(t, m, 76)
}

func TestWelcomeDashboardLines_wideTwoColumn(t *testing.T) {
	th := DefaultTheme()
	opt := WelcomeOptions{
		Version:  "dev",
		Subtitle: "goclaw  ollama/llama3:latest  general-purpose",
		Workdir:  "C:\\Users\\Example\\Desktop\\go-code\\goclaw",
	}
	lines := WelcomeDashboardLines(th, opt, 100)
	require.NotEmpty(t, lines)
	joined := strings.Join(lines, "\n")
	require.Contains(t, joined, "Tips")
	require.Contains(t, joined, "Flows")
	require.Contains(t, joined, "/help")
}

func TestWrapSubtitle_prefersBulletBreaks(t *testing.T) {
	sub := "goclaw · ollama · qwen2.5-coder:14b · general-purpose"
	lines := wrapSubtitle(sub, 36)
	require.NotEmpty(t, lines)
	for _, ln := range lines {
		require.LessOrEqual(t, lipgloss.Width(ln), 36, "line %q", ln)
	}
}
