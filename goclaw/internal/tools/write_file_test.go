package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
 
	"github.com/stretchr/testify/require"
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func TestWriteFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"path": "hello.txt", "content": "hello world"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)
	require.Contains(t, res.Content, "wrote 11 bytes")

	got, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello world", string(got))
}

func TestWriteFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(target, []byte("old content"), 0o600); err != nil {
		require.NoError(t, err)
	}

	tool := NewWriteFile(dir)
	input := mustJSON(t, map[string]any{"path": "file.txt", "content": "new content"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)

	got, _ := os.ReadFile(target)
	require.Equal(t, "new content", string(got))
}

func TestWriteFileEmptyContent(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"path": "empty.txt", "content": ""})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)

	got, err := os.ReadFile(filepath.Join(dir, "empty.txt"))
	require.NoError(t, err)
	require.Len(t, got, 0)
}

func TestWriteFileRejectsEscapePath(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"path": "../../outside.txt", "content": "x"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "workspace")
}

func TestWriteFileRejectsAbsoluteOutsidePath(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFile(dir)

	// Use a different temp dir as the "outside" path
	outside := t.TempDir()
	input := mustJSON(t, map[string]any{"path": filepath.Join(outside, "evil.txt"), "content": "x"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestWriteFileExceedsSizeCap(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFile(dir)

	big := strings.Repeat("a", MaxWriteFileBytes+1)
	input := mustJSON(t, map[string]any{"path": "big.txt", "content": big})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "too large")
	wantN := fmt.Sprintf("%d", MaxWriteFileBytes+1)
	require.Contains(t, res.Content, wantN)
}

func TestWriteFileAtExactSizeCap(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFile(dir)

	exact := strings.Repeat("a", MaxWriteFileBytes)
	input := mustJSON(t, map[string]any{"path": "exact.txt", "content": exact})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, res.IsError, "content=%s", res.Content)
	wantWrote := fmt.Sprintf("%d", MaxWriteFileBytes)
	require.Contains(t, res.Content, wantWrote)
	got, err := os.ReadFile(filepath.Join(dir, "exact.txt"))
	require.NoError(t, err)
	require.Len(t, got, MaxWriteFileBytes)
}

func TestWriteFileParentDirectoryMissing(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"path": "nonexistent-dir/file.txt", "content": "x"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "parent directory")
	require.Contains(t, res.Content, "does not exist")
}

func TestWriteFileRequiresPath(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"content": "x"})
	res, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "path is required")
}

func TestWriteFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFile(dir)

	res, err := tool.Execute(context.Background(), "{bad json}")
	require.NoError(t, err)
	require.True(t, res.IsError)
}
