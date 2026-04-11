package slashcmd

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlashCommandTable_sortedByName(t *testing.T) {
	names := make([]string, len(slashCommandTable))
	for i, e := range slashCommandTable {
		names[i] = e.Name
	}
	require.True(t, sort.StringsAreSorted(names), "slashCommandTable must stay sorted by Name for stable UX")
	require.Len(t, slashCommandTable, 24)
}

func TestTUISlashSuggestions_singleLineOnly(t *testing.T) {
	require.Nil(t, TUISlashSuggestions("/h\nx"))
	require.Nil(t, TUISlashSuggestions(""))
	require.Nil(t, TUISlashSuggestions("hello"))
}

func TestTUISlashSuggestions_filtersByPrefix(t *testing.T) {
	all := TUISlashSuggestions("/")
	require.GreaterOrEqual(t, len(all), 10)

	two := TUISlashSuggestions("/c")
	require.GreaterOrEqual(t, len(two), 2)
	require.Equal(t, "/capabilities", two[0].Name)
	require.Equal(t, "/compact", two[1].Name)

	cap := TUISlashSuggestions("/cap")
	require.Len(t, cap, 1)
	require.Equal(t, "/capabilities", cap[0].Name)

	exact := TUISlashSuggestions("/sessions")
	require.Len(t, exact, 1)
	require.Equal(t, "/sessions", exact[0].Name)
}

func TestSlashTabExpand_longestCommonPrefix(t *testing.T) {
	out, ok := SlashTabExpand("/c")
	require.True(t, ok)
	require.Equal(t, "/capabilities ", out) // first match when /c is not unique

	out, ok = SlashTabExpand("/cap")
	require.True(t, ok)
	require.Equal(t, "/capabilities", out) // unique prefix → full name

	out, ok = SlashTabExpand("/comp")
	require.True(t, ok)
	require.Equal(t, "/compact", out)

	out, ok = SlashTabExpand("/sessions")
	require.True(t, ok)
	require.Equal(t, "/sessions ", out)

	_, ok = SlashTabExpand("/sessions already")
	require.False(t, ok)
}
