package slashcmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlainHelpREPLRequest(t *testing.T) {
	require.True(t, PlainHelpREPLRequest("/help"))
	require.True(t, PlainHelpREPLRequest("  /help  "))
	require.True(t, PlainHelpREPLRequest("help"))
	require.True(t, PlainHelpREPLRequest("?"))
	require.False(t, PlainHelpREPLRequest("/helpme"))
	require.False(t, PlainHelpREPLRequest("/doctor"))
	require.False(t, PlainHelpREPLRequest(""))
	require.False(t, PlainHelpREPLRequest("hello"))
}

func TestTUIHelpShortcutsText_nonEmpty(t *testing.T) {
	s := TUIHelpShortcutsText()
	require.NotEmpty(t, strings.TrimSpace(s))
	require.Contains(t, s, "Tab")
	require.Contains(t, s, "Ctrl+Shift")
	require.Contains(t, s, "Ctrl+M")
	require.Contains(t, s, "Ctrl+E")
	require.Contains(t, s, "/edit")
}
