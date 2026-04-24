package agents

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortedProfileNames_coversAll(t *testing.T) {
	names := SortedProfileNames()
	require.GreaterOrEqual(t, len(names), 9)
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		seen[n] = struct{}{}
	}
	for _, want := range []string{"builder", "general-purpose", "explore", "coordinator", "code-review"} {
		_, ok := seen[want]
		require.True(t, ok, "missing profile %q", want)
	}
}

func TestProfileListHint(t *testing.T) {
	h := ProfileListHint()
	require.Contains(t, h, "build")
	require.Contains(t, h, "coordinator")
	require.Contains(t, h, ", ")
	require.NotContains(t, h, "general-purpose")
}

func TestCanonicalAndDisplayProfileNames(t *testing.T) {
	require.Equal(t, "general-purpose", CanonicalProfileName("build"))
	require.Equal(t, "general-purpose", CanonicalProfileName("general"))
	require.Equal(t, "build", DisplayProfileName("general-purpose"))
	require.Equal(t, "build", DisplayProfileName("build"))
	require.Equal(t, "plan", DisplayProfileName("plan"))
}

func TestUserFacingSortedKeys(t *testing.T) {
	keys := UserFacingSortedKeys(All())
	require.GreaterOrEqual(t, len(keys), 2)
	require.Equal(t, "general-purpose", keys[0])
	require.Equal(t, "plan", keys[1])
}

func TestJoinSortedProfileKeys_UsesUserFacingNames(t *testing.T) {
	hint := JoinSortedProfileKeys(All())
	require.Contains(t, hint, "build")
	require.Contains(t, hint, "plan")
	require.NotContains(t, hint, "general-purpose")
}

func TestProfile_AllowsWorkspaceFileWrites(t *testing.T) {
	require.True(t, GeneralPurpose.AllowsWorkspaceFileWrites())
	require.True(t, Builder.AllowsWorkspaceFileWrites())
	require.False(t, Explore.AllowsWorkspaceFileWrites())
	require.False(t, Plan.AllowsWorkspaceFileWrites())
	require.False(t, CodeReview.AllowsWorkspaceFileWrites())
	require.False(t, Coordinator.AllowsWorkspaceFileWrites())
}

func TestProfile_AllowsSpawnAgentDelegation(t *testing.T) {
	require.True(t, GeneralPurpose.AllowsSpawnAgentDelegation())
	require.True(t, Builder.AllowsSpawnAgentDelegation())
	require.True(t, Coordinator.AllowsSpawnAgentDelegation())
	require.False(t, Explore.AllowsSpawnAgentDelegation())
	require.False(t, CodeReview.AllowsSpawnAgentDelegation())
}
