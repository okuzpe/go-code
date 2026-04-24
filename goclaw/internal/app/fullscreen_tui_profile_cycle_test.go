package app

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/stretchr/testify/require"
)

func TestTuiProfileCycleOrder_defaultFiltersUnknown(t *testing.T) {
	t.Parallel()
	rt := &ChatRuntime{
		Cfg:              config.Default(),
		UserAgentsDir:    "",
		ProjectAgentsDir: "",
	}
	got := tuiProfileCycleOrder(rt)
	require.Equal(t, "general-purpose", got[0], "default cycle starts with general-purpose")
	require.Contains(t, got, "plan")
	require.Contains(t, got, "general-purpose")
	require.Len(t, got, 2)
	seen := make(map[string]int)
	for _, k := range got {
		seen[k]++
	}
	for _, n := range seen {
		require.Equal(t, 1, n)
	}
}

func TestTuiProfileCycleOrder_customOrder(t *testing.T) {
	t.Parallel()
	rt := &ChatRuntime{
		Cfg: config.Config{
			TUIProfileCycle: []string{"explore", "plan", "explore", "nope-profile"},
		},
		UserAgentsDir:    "",
		ProjectAgentsDir: "",
	}
	got := tuiProfileCycleOrder(rt)
	require.Equal(t, []string{"explore", "plan"}, got)
}
