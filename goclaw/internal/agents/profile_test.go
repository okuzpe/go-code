package agents

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortedProfileNames_coversAll(t *testing.T) {
	names := SortedProfileNames()
	require.GreaterOrEqual(t, len(names), 8)
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		seen[n] = struct{}{}
	}
	for _, want := range []string{"general-purpose", "explore", "coordinator", "code-review"} {
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

func TestProfile_AllowsWorkspaceFileWrites(t *testing.T) {
	require.True(t, GeneralPurpose.AllowsWorkspaceFileWrites())
	require.False(t, Explore.AllowsWorkspaceFileWrites())
	require.False(t, Plan.AllowsWorkspaceFileWrites())
	require.False(t, CodeReview.AllowsWorkspaceFileWrites())
	require.False(t, Coordinator.AllowsWorkspaceFileWrites())
}

func TestProfile_AllowsSpawnAgentDelegation(t *testing.T) {
	require.True(t, GeneralPurpose.AllowsSpawnAgentDelegation())
	require.True(t, Coordinator.AllowsSpawnAgentDelegation())
	require.False(t, Explore.AllowsSpawnAgentDelegation())
	require.False(t, CodeReview.AllowsSpawnAgentDelegation())
}
