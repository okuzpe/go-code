package projectcontext

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/stretchr/testify/require"
)

func TestFileUnderRoot_rejectsEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o700))
	_, ok := FileUnderRoot(root, filepath.Join("..", filepath.Base(root), "evil"))
	require.False(t, ok)
	absOutside := filepath.Join(t.TempDir(), "outside-marker.txt")
	require.NoError(t, os.WriteFile(absOutside, []byte("x"), 0o600))
	absOutsideAbs, err := filepath.Abs(absOutside)
	require.NoError(t, err)
	_, ok2 := FileUnderRoot(root, absOutsideAbs)
	require.False(t, ok2)
	full, ok3 := FileUnderRoot(root, filepath.Join("sub", "a.txt"))
	require.True(t, ok3)
	want, err := filepath.Abs(filepath.Join(sub, "a.txt"))
	require.NoError(t, err)
	require.Equal(t, want, full)
}

func TestBuild_claudeLineCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "LINE"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(strings.Join(lines, "\n")), 0o600))

	cfg := config.Default()
	cfg.ProjectContextClaudeMdLines = 5
	got := Build(dir, cfg, true)
	require.Contains(t, got, "first 5 lines")
	require.Equal(t, 5, strings.Count(got, "LINE"))
}

func TestBuild_standingOrdersDefaultPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gdir := filepath.Join(dir, ".goclaw")
	require.NoError(t, os.MkdirAll(gdir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(gdir, "STANDING_ORDERS.md"), []byte("alpha\nbeta\ngamma\n"), 0o600))

	cfg := config.Default()
	got := Build(dir, cfg, true)
	require.Contains(t, got, "standing orders")
	require.Contains(t, got, "alpha")
	require.Contains(t, got, "beta")
}

func TestBuild_standingOrdersCustomPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "my-orders.md"), []byte("custom-order\n"), 0o600))

	cfg := config.Default()
	cfg.ProjectContextStandingOrdersPath = "my-orders.md"
	got := Build(dir, cfg, true)
	require.Contains(t, got, "standing orders")
	require.Contains(t, got, "custom-order")
}

func TestBuild_standingOrdersByteTruncation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gdir := filepath.Join(dir, ".goclaw")
	require.NoError(t, os.MkdirAll(gdir, 0o700))
	longLine := strings.Repeat("x", config.StandingOrdersInjectMaxBytes()+500)
	require.NoError(t, os.WriteFile(filepath.Join(gdir, "STANDING_ORDERS.md"), []byte(longLine), 0o600))

	cfg := config.Default()
	cfg.ProjectContextStandingOrdersMaxLines = 5
	got := Build(dir, cfg, true)
	require.Contains(t, got, "truncated")
}

func TestBuild_thinOmitsClaudeAndStandingOrders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/thin\n\ngo 1.22\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("SECRET_CONVENTION_BLOCK\n"), 0o600))
	gdir := filepath.Join(dir, ".goclaw")
	require.NoError(t, os.MkdirAll(gdir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(gdir, "STANDING_ORDERS.md"), []byte("SECRET_STANDING\n"), 0o600))

	cfg := config.Default()
	full := Build(dir, cfg, true)
	thin := Build(dir, cfg, false)

	require.Contains(t, full, "SECRET_CONVENTION_BLOCK")
	require.Contains(t, full, "SECRET_STANDING")
	require.Contains(t, thin, "go.mod")
	require.NotContains(t, thin, "SECRET_CONVENTION_BLOCK")
	require.NotContains(t, thin, "SECRET_STANDING")
	require.NotContains(t, thin, "CLAUDE.md")
}

func TestBuild_thinOnlyClaudeYieldsHintNotConventions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("ONLY_CLAUDE\n"), 0o600))

	cfg := config.Default()
	thin := Build(dir, cfg, false)
	require.NotContains(t, thin, "ONLY_CLAUDE")
	require.Contains(t, thin, "project_workspace_hint")
}

func TestBuild_includesProjectDocsSummary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "docs-map.md"), []byte("# Docs Map\nintro\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte("# Architecture\nflow\n"), 0o600))

	cfg := config.Default()
	got := Build(dir, cfg, false)
	require.Contains(t, got, "docs/docs-map.md")
	require.Contains(t, got, "Docs Map")
	require.Contains(t, got, "docs/architecture.md")
	require.Contains(t, got, "Architecture")
	require.NotContains(t, got, "project_workspace_hint")
}

func TestBuild_includesWorkspaceLayout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/layout\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "main.go"), []byte("package main\n"), 0o600))

	cfg := config.Default()
	got := Build(dir, cfg, false)
	require.Contains(t, got, "workspace_layout:")
	require.Contains(t, got, "cmd/")
	require.Contains(t, got, "internal/")
}

func TestBuild_includesGitWorkspaceSummary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("hello\n"), 0o600))
	runGit(t, dir, "add", "tracked.txt")
	runGit(t, dir, "commit", "-m", "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("changed\n"), 0o600))

	cfg := config.Default()
	got := Build(dir, cfg, false)
	require.Contains(t, got, "git_workspace:")
	require.Contains(t, got, "dirty files: 1")
}

func TestBuild_includesGoRepoMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "app"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/repomap\n\ngo 1.22\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte("package main\n\nfunc main() {}\nfunc run() {}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal", "app", "run.go"), []byte("package app\n\ntype Runner struct{}\n\nfunc Run() {}\nfunc (Runner) Start() {}\n"), 0o600))

	cfg := config.Default()
	got := Build(dir, cfg, false)
	require.Contains(t, got, "repo_map:")
	require.Contains(t, got, "cmd/demo/main.go [main]")
	require.Contains(t, got, "func main")
	require.Contains(t, got, "internal/app/run.go [app]")
	require.Contains(t, got, "type Runner")
	require.Contains(t, got, "func Run")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}
