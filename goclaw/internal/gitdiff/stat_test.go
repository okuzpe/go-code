package gitdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorktreeDiffStat(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	t.Run("empty_workdir", func(t *testing.T) {
		require.Equal(t, "", WorktreeDiffStat(""))
		require.Equal(t, "", WorktreeDiffStat("   "))
	})

	t.Run("not_a_git_repo", func(t *testing.T) {
		dir := t.TempDir()
		require.Equal(t, "", WorktreeDiffStat(dir))
	})

	t.Run("with_uncommitted_change", func(t *testing.T) {
		dir := t.TempDir()
		runGit(t, dir, "init")
		runGit(t, dir, "config", "user.email", "goclaw-test@example.com")
		runGit(t, dir, "config", "user.name", "goclaw test")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("one\n"), 0o644))
		runGit(t, dir, "add", "file.txt")
		runGit(t, dir, "commit", "-m", "init")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("one\ntwo\n"), 0o644))

		out := WorktreeDiffStat(dir)
		require.NotEmpty(t, out)
		require.True(t, strings.HasPrefix(out, "goclaw: git diff --stat\n"), "got prefix: %q", out)
		require.Contains(t, out, "file.txt")
	})
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
