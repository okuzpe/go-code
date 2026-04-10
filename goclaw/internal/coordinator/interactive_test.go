package coordinator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveInteractiveTaskID(t *testing.T) {
	ch := make(chan workerJob, 1)
	w := &interactiveWorker{taskID: "abc123xyz", profile: "explore", inbox: ch}
	storeInteractive(w)
	t.Cleanup(func() { deleteInteractive("abc123xyz") })

	_, ok := ResolveInteractiveTaskID("nomatch")
	require.False(t, ok)

	full, ok := ResolveInteractiveTaskID("abc")
	require.True(t, ok)
	require.Equal(t, "abc123xyz", full)

	_, ok = ResolveInteractiveTaskID("")
	require.False(t, ok)
}
