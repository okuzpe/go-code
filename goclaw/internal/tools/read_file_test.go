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
	require.Equal(t, "hello\nworld", res.Content)
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

func TestReadFileRejectsEscape(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	outside := t.TempDir()
	tool := NewReadFile(dir)
	target := filepath.Join(outside, "secret.txt")
	_ = os.WriteFile(target, []byte("x"), 0o600)

	payload, _ := json.Marshal(map[string]string{"path": filepath.Join(outside, "secret.txt")})
	res, err := tool.Execute(ctx, string(payload))
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.NotEmpty(t, res.Content)
}
