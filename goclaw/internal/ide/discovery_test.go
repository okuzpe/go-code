package ide

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverMCPEndpoint_nonExistentDir(t *testing.T) {
	_, _, err := DiscoverMCPEndpoint("/nonexistent/ide/dir")
	require.Error(t, err)
}

func TestDiscoverMCPEndpoint_emptyDir(t *testing.T) {
	dir := t.TempDir()
	_, _, err := DiscoverMCPEndpoint(dir)
	require.Error(t, err)
}

func TestDiscoverMCPEndpoint_allInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`not json`), 0o600))
	_, _, err := DiscoverMCPEndpoint(dir)
	require.Error(t, err)
}

func TestDiscoverMCPEndpoint_validLoopback(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ide.json"),
		[]byte(`{"url":"http://127.0.0.1:4321/mcp","headers":{"Authorization":"Bearer tok"}}`),
		0o600))
	u, hdrs, err := DiscoverMCPEndpoint(dir)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:4321/mcp", u)
	require.Equal(t, "Bearer tok", hdrs["Authorization"])
}

func TestDiscoverMCPEndpoint_skipsRemoteURL(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "remote.json"),
		[]byte(`{"url":"https://example.com/mcp"}`), 0o600))
	_, _, err := DiscoverMCPEndpoint(dir)
	require.Error(t, err, "remote URL must be skipped")
}

func TestDiscoverMCPEndpoint_sortedFirstWins(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "z.json"), []byte(`{"url":"http://127.0.0.1:9999/z"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{"url":"http://127.0.0.1:1111/a"}`), 0o600))
	u, _, err := DiscoverMCPEndpoint(dir)
	require.NoError(t, err)
	require.Contains(t, u, "1111", "a.json (sorted first) must win over z.json")
}

func TestDiscoverMCPEndpoint_emptyURLSkipped(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.json"), []byte(`{"url":""}`), 0o600))
	_, _, err := DiscoverMCPEndpoint(dir)
	require.Error(t, err)
}
