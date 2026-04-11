package chat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDropTrailingBlankLines(t *testing.T) {
	t.Parallel()
	in := []string{"a", "", "  ", ""}
	got := dropTrailingBlankLines(in)
	require.Equal(t, []string{"a"}, got)
	require.Equal(t, []string{"x"}, dropTrailingBlankLines([]string{"x"}))
	require.Nil(t, dropTrailingBlankLines(nil))
}

func TestIsAssistantDimPlaceholderLine(t *testing.T) {
	t.Parallel()
	pfx := "●"
	require.True(t, isAssistantDimPlaceholderLine("● …", pfx))
	require.True(t, isAssistantDimPlaceholderLine("●  …", pfx))
	require.True(t, isAssistantDimPlaceholderLine("● ...", pfx))
	require.False(t, isAssistantDimPlaceholderLine("● Hola", pfx))
	require.False(t, isAssistantDimPlaceholderLine("> …", pfx))

	pfxNamed := "● Claude"
	require.True(t, isAssistantDimPlaceholderLine("● Claude …", pfxNamed))
	require.False(t, isAssistantDimPlaceholderLine("● Claude está", pfxNamed))
}
