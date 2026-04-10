package slashcmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionLocationBanner_absPath(t *testing.T) {
	dir := t.TempDir()
	b := SessionLocationBanner(dir)
	require.Contains(t, b, filepath.Clean(dir))
	require.Contains(t, b, "You are in directory:")
	require.Contains(t, b, "/capabilities")
	require.Contains(t, b, "What project or task can I help with today?")
}

func TestUserCapabilitiesGuide_nonEmpty(t *testing.T) {
	g := UserCapabilitiesGuide()
	require.NotEmpty(t, strings.TrimSpace(g))
	require.Contains(t, g, "/help")
	require.Contains(t, g, "Research & Q&A")
	require.Contains(t, g, "Code & Development")
	require.Contains(t, g, "File & Project Management")
	require.Contains(t, g, "---")
	require.Contains(t, g, "What are you working on?")
}
