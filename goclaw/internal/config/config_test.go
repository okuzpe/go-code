package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMCPServerConfig_EnvSlice_empty(t *testing.T) {
	srv := MCPServerConfig{}
	require.Nil(t, srv.EnvSlice())
}

func TestMCPServerConfig_EnvSlice_populated(t *testing.T) {
	srv := MCPServerConfig{
		Env: map[string]string{"FOO": "bar", "BAZ": "qux"},
	}
	pairs := srv.EnvSlice()
	require.Len(t, pairs, 2)
	// Values must be KEY=value format.
	for _, p := range pairs {
		require.Contains(t, p, "=")
	}
}

func TestConfig_Model_anthropic(t *testing.T) {
	t.Setenv("GOCLAW_MODEL", "claude-opus-4")
	cfg := Default()
	cfg.Provider = "anthropic"
	require.Equal(t, "claude-opus-4", cfg.Model())
}

func TestConfig_Model_anthropic_defaultWhenEnvUnset(t *testing.T) {
	t.Setenv("GOCLAW_MODEL", "")
	cfg := Default()
	cfg.Provider = "anthropic"
	// Should fall back to the built-in default, not empty string.
	require.NotEmpty(t, cfg.Model())
}

func TestConfig_Model_ollama(t *testing.T) {
	cfg := Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "qwen2.5-coder:14b"
	require.Equal(t, "qwen2.5-coder:14b", cfg.Model())
}

func TestMergeMCPServersByID_incomingOverridesExisting(t *testing.T) {
	existing := []MCPServerConfig{
		{ID: "server-a", Command: "old-cmd"},
		{ID: "server-b", Command: "b-cmd"},
	}
	incoming := []MCPServerConfig{
		{ID: "server-a", Command: "new-cmd"},
		{ID: "server-c", Command: "c-cmd"},
	}

	result := mergeMCPServersByID(existing, incoming)
	require.Len(t, result, 3)

	byID := make(map[string]MCPServerConfig)
	for _, s := range result {
		byID[s.ID] = s
	}
	require.Equal(t, "new-cmd", byID["server-a"].Command)
	require.Equal(t, "b-cmd", byID["server-b"].Command)
	require.Equal(t, "c-cmd", byID["server-c"].Command)
}

func TestMergeMCPServersByID_sortedByID(t *testing.T) {
	existing := []MCPServerConfig{
		{ID: "zebra", Command: "z"},
		{ID: "alpha", Command: "a"},
	}
	result := mergeMCPServersByID(existing, nil)
	require.Len(t, result, 2)
	require.Equal(t, "alpha", result[0].ID)
	require.Equal(t, "zebra", result[1].ID)
}

func TestMergeMCPServersByID_emptyIDsSkipped(t *testing.T) {
	servers := []MCPServerConfig{
		{ID: "", Command: "no-id"},
		{ID: "valid", Command: "ok"},
	}
	result := mergeMCPServersByID(servers, nil)
	require.Len(t, result, 1)
	require.Equal(t, "valid", result[0].ID)
}
