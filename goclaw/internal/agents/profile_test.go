package agents

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortedProfileNames_coversAll(t *testing.T) {
	names := SortedProfileNames()
	require.GreaterOrEqual(t, len(names), 7)
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		seen[n] = struct{}{}
	}
	for _, want := range []string{"general-purpose", "explore", "coordinator"} {
		_, ok := seen[want]
		require.True(t, ok, "missing profile %q", want)
	}
}

func TestProfileListHint(t *testing.T) {
	h := ProfileListHint()
	require.Contains(t, h, "coordinator")
	require.Contains(t, h, ", ")
	require.Equal(t, strings.Join(SortedProfileNames(), ", "), h)
}
