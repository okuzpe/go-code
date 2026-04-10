package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeWriteSettingsPreservesKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"agent_profile":"plan","ollama_model":"x"}`), 0o600))

	require.NoError(t, MergeWriteSettings(path, map[string]any{"ui_appearance": "dark"}))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	s := string(raw)
	require.Contains(t, s, "plan")
	require.Contains(t, s, "ollama_model")
	require.Contains(t, s, "dark")
}

func TestMergeWriteSettingsCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "settings.json")
	require.NoError(t, MergeWriteSettings(path, map[string]any{"provider": "ollama"}))
	_, err := os.Stat(path)
	require.NoError(t, err)
}
