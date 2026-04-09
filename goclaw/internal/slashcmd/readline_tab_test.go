package slashcmd

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadlineSlashPrefixes_sortedAndComplete(t *testing.T) {
	p := ReadlineSlashPrefixes()
	require.True(t, sort.StringsAreSorted(p), "keep ReadlineSlashPrefixes sorted for stable Tab UX")
	require.Contains(t, p, "/doctor")
	require.Contains(t, p, "/new")
	require.Contains(t, p, "/sessions")
	require.Len(t, p, 14)
}

func TestReadlinePrefixCompleter_builds(t *testing.T) {
	c := ReadlinePrefixCompleter()
	require.NotNil(t, c)
}
