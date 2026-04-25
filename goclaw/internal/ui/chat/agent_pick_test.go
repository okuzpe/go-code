package chat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentPickNamesDefaultToPrimaryModes(t *testing.T) {
	t.Parallel()

	m := &Model{}
	names := m.agentPickNames()

	require.ElementsMatch(t, []string{"general-purpose", "plan"}, names)
	require.NotContains(t, names, "builder")
	require.NotContains(t, names, "coordinator")
}

func TestAgentPickNamesHiddenProfilesDoNotReopenAdvancedProfiles(t *testing.T) {
	t.Parallel()

	m := &Model{agentPickerHidden: []string{"build"}}
	names := m.agentPickNames()

	require.Equal(t, []string{"plan"}, names)
	require.NotContains(t, names, "builder")
	require.NotContains(t, names, "coordinator")
}
