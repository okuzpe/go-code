package slashcmd

import (
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/stretchr/testify/require"
)

func TestPopularSlashHint_withWorkdir(t *testing.T) {
	hint := PopularSlashHint("/some/workspace")
	require.Contains(t, hint, "/help")
	require.Contains(t, hint, "/quit")
	require.Contains(t, hint, "Plan:")
}

func TestPopularSlashHint_noWorkdir(t *testing.T) {
	hint := PopularSlashHint("")
	require.Contains(t, hint, "/help")
	require.NotContains(t, hint, "Plan:")
}

func TestPreChatHelpSummary_withWorkdir(t *testing.T) {
	summary := PreChatHelpSummary("/workspace")
	require.Contains(t, summary, "Slash commands")
	require.Contains(t, summary, "/apply-plan")
	require.Contains(t, summary, "Plan file:")
}

func TestPreChatHelpSummary_noWorkdir(t *testing.T) {
	summary := PreChatHelpSummary("")
	require.Contains(t, summary, "Slash commands")
	require.NotContains(t, summary, "Plan file:")
}

func TestSortedProfileNames_sorted(t *testing.T) {
	profs := map[string]agents.Profile{
		"zebra": {Name: "zebra"},
		"alpha": {Name: "alpha"},
		"mango": {Name: "mango"},
	}
	result := sortedProfileNames(profs)
	parts := strings.Split(result, ", ")
	require.Equal(t, []string{"alpha", "mango", "zebra"}, parts)
}

func TestSortedProfileNames_empty(t *testing.T) {
	require.Equal(t, "", sortedProfileNames(nil))
}

func TestPreviewRunes_shorterThanMax(t *testing.T) {
	require.Equal(t, "hello", previewRunes("hello", 10))
}

func TestPreviewRunes_longerThanMax(t *testing.T) {
	result := previewRunes("hello world", 5)
	require.Equal(t, "hello…", result)
}

func TestPreviewRunes_newlinesReplaced(t *testing.T) {
	result := previewRunes("line1\nline2", 20)
	require.NotContains(t, result, "\n")
}

func TestPreviewRunes_emptyOrZeroMax(t *testing.T) {
	require.Equal(t, "", previewRunes("", 10))
	require.Equal(t, "", previewRunes("text", 0))
}
