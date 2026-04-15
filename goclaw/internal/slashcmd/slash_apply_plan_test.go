package slashcmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseApplyPlanRest(t *testing.T) {
	path, preview, hub := parseApplyPlanRest("")
	require.False(t, preview)
	require.False(t, hub)
	require.Equal(t, "", path)

	path, preview, hub = parseApplyPlanRest("--preview")
	require.True(t, preview)
	require.False(t, hub)
	require.Equal(t, "", path)

	_, preview, hub = parseApplyPlanRest("-preview")
	require.True(t, preview)
	require.False(t, hub)

	path, preview, hub = parseApplyPlanRest("--preview notes/plan.md")
	require.True(t, preview)
	require.False(t, hub)
	require.Equal(t, "notes/plan.md", path)

	path, preview, hub = parseApplyPlanRest("notes/plan.md --preview")
	require.True(t, preview)
	require.False(t, hub)
	require.Equal(t, "notes/plan.md", path)

	path, preview, hub = parseApplyPlanRest("--yes")
	require.False(t, preview)
	require.False(t, hub)
	require.Equal(t, "", path)

	path, preview, hub = parseApplyPlanRest("--yes sub/plan.md")
	require.False(t, preview)
	require.False(t, hub)
	require.Equal(t, "sub/plan.md", path)

	path, preview, hub = parseApplyPlanRest("--hub")
	require.False(t, preview)
	require.True(t, hub)
	require.Equal(t, "", path)

	path, preview, hub = parseApplyPlanRest("sub/plan.md --hub")
	require.False(t, preview)
	require.True(t, hub)
	require.Equal(t, "sub/plan.md", path)
}

func TestTruncateRunesPlanPreview(t *testing.T) {
	s := strings.Repeat("あ", 10)
	out := truncateRunesPlanPreview(s, 5)
	require.Contains(t, out, "truncated for preview")
	require.NotEqual(t, s, out)
}
