package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadFileHappyPath(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("hello\nworld"), 0o600); err != nil {
		require.NoError(t, err)
	}
	tool := NewReadFile(dir)
	res, err := tool.Execute(ctx, `{"path":"a.txt"}`)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)
	require.Equal(t, "   1\thello\n   2\tworld", res.Content)
}

func TestReadFileListsDirectory(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "a.go"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.go"), []byte("x"), 0o600))

	tool := NewReadFile(dir)
	res, err := tool.Execute(ctx, `{"path":"sub"}`)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)
	require.Contains(t, res.Content, "directory: sub")
	require.Contains(t, res.Content, "a.go")
	require.Contains(t, res.Content, "b.go")
}

func TestReadFileListsDirectorySkipsGit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o600))

	tool := NewReadFile(dir)
	res, err := tool.Execute(ctx, `{"path":"."}`)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)
	require.Contains(t, res.Content, "go.mod")
	require.NotContains(t, res.Content, "HEAD") // .git skipped
}

func TestReadFileOffsetLines(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := filepath.Join(dir, "nums.txt")
	require.NoError(t, os.WriteFile(p, []byte("a\nb\nc\nd\ne"), 0o600))

	tool := NewReadFile(dir)
	// offset 2 → skip first 2 lines; result starts at line 3
	res, err := tool.Execute(ctx, `{"path":"nums.txt","offset_lines":2}`)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "   3\tc\n   4\td\n   5\te", res.Content)
}

func TestReadFileAbsoluteOutsideRoot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	outside := t.TempDir()
	tool := NewReadFile(dir)
	target := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(target, []byte("hello-outside"), 0o600))

	payload, _ := json.Marshal(map[string]string{"path": target})
	res, err := tool.Execute(ctx, string(payload))
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)
	require.Contains(t, res.Content, "hello-outside")
}
