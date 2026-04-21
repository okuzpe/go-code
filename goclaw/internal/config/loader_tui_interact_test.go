package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadTUIInteractMode(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, ".goclaw")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"tui_interact_mode":"agent"}`), 0o600))
	base := Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"
	cfg, err := Load(base, cwd)
	require.NoError(t, err)
	require.Equal(t, TUIInteractModeAgent, cfg.TUIInteractMode)
}
