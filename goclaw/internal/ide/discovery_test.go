package ide

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverMCPEndpoint(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ide")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	_, _, err := DiscoverMCPEndpoint(dir)
	require.Error(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "z.json"), []byte(`not json`), 0o600))
	_, _, err = DiscoverMCPEndpoint(dir)
	require.Error(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{"url":"http://127.0.0.1:1234/mcp"}`), 0o600))
	u, hdrs, err := DiscoverMCPEndpoint(dir)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:1234/mcp", u)
	require.Nil(t, hdrs)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.json"), []byte(`{"url":"https://example.com/mcp"}`), 0o600))
	u2, _, err := DiscoverMCPEndpoint(dir)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:1234/mcp", u2, "first sorted valid lockfile wins")
}
