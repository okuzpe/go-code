package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/stretchr/testify/require"
)

func TestProjectFileUnderRoot_rejectsEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o700))
	_, ok := projectFileUnderRoot(root, filepath.Join("..", filepath.Base(root), "evil"))
	require.False(t, ok)
	_, ok2 := projectFileUnderRoot(root, "/etc/passwd")
	require.False(t, ok2)
	full, ok3 := projectFileUnderRoot(root, filepath.Join("sub", "a.txt"))
	require.True(t, ok3)
	want, err := filepath.Abs(filepath.Join(sub, "a.txt"))
	require.NoError(t, err)
	require.Equal(t, want, full)
}

func TestBuildProjectContext_claudeLineCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "LINE"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(strings.Join(lines, "\n")), 0o600))

	cfg := config.Default()
	cfg.ProjectContextClaudeMdLines = 5
	got := buildProjectContext(dir, cfg)
	require.Contains(t, got, "first 5 lines")
	require.Equal(t, 5, strings.Count(got, "LINE"))
}

func TestBuildProjectContext_standingOrdersDefaultPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gdir := filepath.Join(dir, ".goclaw")
	require.NoError(t, os.MkdirAll(gdir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(gdir, "STANDING_ORDERS.md"), []byte("alpha\nbeta\ngamma\n"), 0o600))

	cfg := config.Default()
	got := buildProjectContext(dir, cfg)
	require.Contains(t, got, "standing orders")
	require.Contains(t, got, "alpha")
	require.Contains(t, got, "beta")
}

func TestBuildProjectContext_standingOrdersCustomPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "my-orders.md"), []byte("custom-order\n"), 0o600))

	cfg := config.Default()
	cfg.ProjectContextStandingOrdersPath = "my-orders.md"
	got := buildProjectContext(dir, cfg)
	require.Contains(t, got, "standing orders")
	require.Contains(t, got, "custom-order")
}

func TestBuildProjectContext_standingOrdersByteTruncation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gdir := filepath.Join(dir, ".goclaw")
	require.NoError(t, os.MkdirAll(gdir, 0o700))
	longLine := strings.Repeat("x", config.StandingOrdersInjectMaxBytes()+500)
	require.NoError(t, os.WriteFile(filepath.Join(gdir, "STANDING_ORDERS.md"), []byte(longLine), 0o600))

	cfg := config.Default()
	cfg.ProjectContextStandingOrdersMaxLines = 5
	got := buildProjectContext(dir, cfg)
	require.Contains(t, got, "truncated")
}
