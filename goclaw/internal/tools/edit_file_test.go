package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		require.NoError(t, err)
	}
	return p
}

func TestEditFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "alpha\nbeta\ngamma\n")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "beta", "new_string": "BETA"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)
	require.Contains(t, res.Content, "replaced 1 occurrence")

	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.Contains(t, string(got), "BETA")
	require.NotContains(t, string(got), "beta")
}

func TestEditFileDeleteOccurrence(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "foo bar baz")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "bar ", "new_string": ""})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)

	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.Equal(t, "foo baz", string(got))
}

func TestEditFileOldStringNotFound(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "hello")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "world", "new_string": "x"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "not found")
	require.Contains(t, res.Content, "0 matches")
}

func TestEditFileMultipleOccurrencesNoReplaceAll(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "aa bb aa")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "aa", "new_string": "XX"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "found 2 times")
	require.Contains(t, res.Content, "replace_all")
}

func TestEditFileReplaceAll(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "aa bb aa")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "aa", "new_string": "XX", "replace_all": true})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)
	require.Contains(t, res.Content, "replaced 2 occurrence")

	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.Equal(t, "XX bb XX", string(got))
}

func TestEditFileMultilineOldString(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "line1\nline2\nline3\n")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "line1\nline2", "new_string": "merged"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)

	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.Equal(t, "merged\nline3\n", string(got))
}

func TestEditFileStripsReadFileLinePrefixesFromOldString(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{
		"path":       "main.go",
		"old_string": "   1\tpackage main\n   2\t\n   3\tfunc main() {}",
		"new_string": "package main\n\nfunc main() {\n\tprintln(\"ok\")\n}",
	})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)

	got, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	require.Contains(t, string(got), "println(\"ok\")")
}

func TestEditFileStripsContextMarkerPrefixesFromOldString(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "alpha\nbeta\ngamma\n")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{
		"path":       "f.txt",
		"old_string": "  1\talpha\n> 2\tbeta\n  3\tgamma",
		"new_string": "alpha\nBETA\ngamma",
	})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)

	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.Contains(t, string(got), "BETA")
	require.NotContains(t, string(got), "\talpha")
}

func TestEditFileAbsoluteOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "out.txt")
	require.NoError(t, os.WriteFile(target, []byte("alpha"), 0o600))
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": target, "old_string": "alpha", "new_string": "beta"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "beta", string(got))
	_ = dir
}

func TestEditFileFileNotFound(t *testing.T) {
	dir := t.TempDir()
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "nonexistent.txt", "old_string": "x", "new_string": "y"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "does not exist")
}

func TestEditFileRequiresOldString(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "x")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "", "new_string": "y"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "old_string is required")
}

func TestEditFileRequiresPath(t *testing.T) {
	dir := t.TempDir()
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"old_string": "x", "new_string": "y"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestEditFileResultExceedsSizeCap(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "x")
	tool := NewEditFile(dir)

	// Replace "x" with a string that exceeds the size cap
	big := strings.Repeat("y", 1*1024*1024+1)
	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "x", "new_string": big})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "too large")

	// File must be unchanged
	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.Equal(t, "x", string(got))
}

func TestEditFilePreservesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits not supported on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(target, []byte("hello world"), 0o640); err != nil {
		require.NoError(t, err)
	}
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "hello", "new_string": "goodbye"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)

	info, err := os.Stat(target)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestEditFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	tool := NewEditFile(dir)

	res, err := tool.Execute(context.Background(), "{bad json}")
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestEditFileWindowsLineEndingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// CRLF throughout — replace must not normalize line endings unexpectedly.
	writeTestFile(t, dir, "f.txt", "first line\r\nsecond line\r\nthird\r\n")
	tool := NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "second line", "new_string": "SECOND"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)

	got, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	want := "first line\r\nSECOND\r\nthird\r\n"
	require.NoError(t, err)
	require.Equal(t, want, string(got))
}

func TestEditFileReadOnlyTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only file behaviour is platform-specific on Windows")
	}
	// A file mode 0444 does not block atomic replace: rename needs write permission on the
	// parent directory, not the file. Lock the directory instead so CreateTemp/rename fails.
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	target := filepath.Join(sub, "f.txt")
	require.NoError(t, os.WriteFile(target, []byte("immutable"), 0o644))
	require.NoError(t, os.Chmod(sub, 0o555))
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	tool := NewEditFile(sub)
	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "immutable", "new_string": "changed"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError, "expected IsError when directory is read-only, got success: %q", res.Content)
	low := strings.ToLower(res.Content)
	require.True(t, strings.Contains(low, "permission denied") || strings.Contains(low, "operation not permitted"), "expected permission error in message, got: %q", res.Content)

	got, _ := os.ReadFile(target)
	require.Equal(t, "immutable", string(got))
}
