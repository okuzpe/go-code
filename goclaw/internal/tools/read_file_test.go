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
