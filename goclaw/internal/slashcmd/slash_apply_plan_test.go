package slashcmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseApplyPlanRest(t *testing.T) {
	path, preview, hub, steps := parseApplyPlanRest("")
	require.False(t, preview)
	require.False(t, hub)
	require.False(t, steps)
	require.Equal(t, "", path)

	path, preview, hub, steps = parseApplyPlanRest("--preview")
	require.True(t, preview)
	require.False(t, hub)
	require.False(t, steps)
	require.Equal(t, "", path)

	_, preview, hub, steps = parseApplyPlanRest("-preview")
	require.True(t, preview)
	require.False(t, hub)
	require.False(t, steps)

	path, preview, hub, steps = parseApplyPlanRest("--preview notes/plan.md")
	require.True(t, preview)
	require.False(t, hub)
	require.False(t, steps)
	require.Equal(t, "notes/plan.md", path)

	path, preview, hub, steps = parseApplyPlanRest("notes/plan.md --preview")
	require.True(t, preview)
	require.False(t, hub)
	require.False(t, steps)
	require.Equal(t, "notes/plan.md", path)

	path, preview, hub, steps = parseApplyPlanRest("--yes")
	require.False(t, preview)
	require.False(t, hub)
	require.False(t, steps)
	require.Equal(t, "", path)

	path, preview, hub, steps = parseApplyPlanRest("--yes sub/plan.md")
	require.False(t, preview)
	require.False(t, hub)
	require.False(t, steps)
	require.Equal(t, "sub/plan.md", path)

	path, preview, hub, steps = parseApplyPlanRest("--hub")
	require.False(t, preview)
	require.True(t, hub)
	require.False(t, steps)
	require.Equal(t, "", path)

	path, preview, hub, steps = parseApplyPlanRest("sub/plan.md --hub")
	require.False(t, preview)
	require.True(t, hub)
	require.False(t, steps)
	require.Equal(t, "sub/plan.md", path)

	path, preview, hub, steps = parseApplyPlanRest("--steps a/b.md --hub")
	require.False(t, preview)
	require.True(t, hub)
	require.True(t, steps)
	require.Equal(t, "a/b.md", path)
}

func TestTruncateRunesPlanPreview(t *testing.T) {
	s := strings.Repeat("あ", 10)
	out := truncateRunesPlanPreview(s, 5)
	require.Contains(t, out, "truncated for preview")
	require.NotEqual(t, s, out)
}
