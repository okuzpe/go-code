package slashcmd

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func commandSummaryByName(name string) string {
	for _, entry := range slashCommandTable {
		if entry.Name == name {
			return entry.Summary
		}
	}
	return ""
}

func TestSlashCommandTable_sortedByName(t *testing.T) {
	names := make([]string, len(slashCommandTable))
	for i, e := range slashCommandTable {
		names[i] = e.Name
	}
	require.True(t, sort.StringsAreSorted(names), "slashCommandTable must stay sorted by Name for stable UX")
	require.Positive(t, len(slashCommandTable))
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
	require.GreaterOrEqual(t, len(two), 3)
	require.Equal(t, "/capabilities", two[0].Name)
	require.Equal(t, "/clear", two[1].Name)
	require.Equal(t, "/compact", two[2].Name)

	cap := TUISlashSuggestions("/cap")
	require.Len(t, cap, 1)
	require.Equal(t, "/capabilities", cap[0].Name)

	exact := TUISlashSuggestions("/sessions")
	require.Len(t, exact, 1)
	require.Equal(t, "/sessions", exact[0].Name)
}

func TestSlashCommandTable_UsesSharedSubcommandCatalogs(t *testing.T) {
	require.Contains(t, commandSummaryByName("/memory"), usageList(memorySubcommandSpecs))
	require.Contains(t, commandSummaryByName("/plan"), usageList(planSubcommandSpecs))
}

func TestSuggestSubcommands_UsesCatalogOrder(t *testing.T) {
	got := suggestSubcommands(memorySubcommandSpecs, "")
	require.Len(t, got, len(memorySubcommandSpecs))
	for i, spec := range memorySubcommandSpecs {
		require.Equal(t, spec.Name, got[i].Name)
		require.Equal(t, spec.Summary, got[i].Summary)
	}

	got = suggestSubcommands(planSubcommandSpecs, "re")
	require.Len(t, got, 2)
	require.Equal(t, "review", got[0].Name)
	require.Equal(t, "revoke", got[1].Name)
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
