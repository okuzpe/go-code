package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathDescendsFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o700))

	require.True(t, PathDescendsFrom(dir, dir))
	require.True(t, PathDescendsFrom(dir, filepath.Join(dir, "a")))
	require.True(t, PathDescendsFrom(dir, filepath.Join(dir, "a", "b")))
	require.False(t, PathDescendsFrom(filepath.Join(dir, "a"), filepath.Join(dir, "..")))
}

func TestCandidateWriteTargetPathNoMkdir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	scope := PathScope{Root: base, RelativeBase: base}
	got, err := CandidateWriteTargetPath(scope, "notes/out.txt")
	require.NoError(t, err)
	want := filepath.Clean(filepath.Join(base, "notes", "out.txt"))
	require.Equal(t, want, got)
}
