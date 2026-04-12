package slashcmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseApplyPlanRest(t *testing.T) {
	path, preview := parseApplyPlanRest("")
	require.False(t, preview)
	require.Equal(t, "", path)

	path, preview = parseApplyPlanRest("--preview")
	require.True(t, preview)
	require.Equal(t, "", path)

	path, preview = parseApplyPlanRest("-preview")
	require.True(t, preview)

	path, preview = parseApplyPlanRest("--preview notes/plan.md")
	require.True(t, preview)
	require.Equal(t, "notes/plan.md", path)

	path, preview = parseApplyPlanRest("notes/plan.md --preview")
	require.True(t, preview)
	require.Equal(t, "notes/plan.md", path)

	path, preview = parseApplyPlanRest("--yes")
	require.False(t, preview)
	require.Equal(t, "", path)

	path, preview = parseApplyPlanRest("--yes sub/plan.md")
	require.False(t, preview)
	require.Equal(t, "sub/plan.md", path)
}

func TestTruncateRunesPlanPreview(t *testing.T) {
	s := strings.Repeat("あ", 10)
	out := truncateRunesPlanPreview(s, 5)
	require.Contains(t, out, "truncated for preview")
	require.NotEqual(t, s, out)
}
