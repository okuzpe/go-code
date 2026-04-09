package config

import (
	"os"
	"path/filepath"
	"testing"
 
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/stretchr/testify/require"
)

func TestLoadProjectOverridesUser(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()

	userDir := filepath.Join(home, ".goclaw")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	userJSON := filepath.Join(userDir, "settings.json")
	require.NoError(t, os.WriteFile(userJSON, []byte(`{"agent_profile":"plan","tool_permissions":{"bash":"allow"}}`), 0o600))

	projGoclaw := filepath.Join(cwd, ".goclaw")
	require.NoError(t, os.MkdirAll(projGoclaw, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(projGoclaw, "settings.json"), []byte(`{"agent_profile":"explore","tool_permissions":{"bash":"deny"}}`), 0o600))

	base := Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"

	cfg, err := Load(base, cwd)
	require.NoError(t, err)
	require.Equal(t, "explore", cfg.AgentProfile)
	require.Equal(t, "deny", cfg.PermissionModes["bash"])
}

func TestLoadSettingsLocalOverridesProject(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()

	userDir := filepath.Join(home, ".goclaw")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"agent_profile":"plan"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "settings.local.json"), []byte(`{"agent_profile":"guide"}`), 0o600))

	projGoclaw := filepath.Join(cwd, ".goclaw")
	require.NoError(t, os.MkdirAll(projGoclaw, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(projGoclaw, "settings.json"), []byte(`{"agent_profile":"explore"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(projGoclaw, "settings.local.json"), []byte(`{"agent_profile":"verification"}`), 0o600))

	base := Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"

	cfg, err := Load(base, cwd)
	require.NoError(t, err)
	require.Equal(t, "verification", cfg.AgentProfile)
}

func TestLoadBashTimeoutSec(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, ".goclaw")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"bash_timeout_sec":90}`), 0o600))
	base := Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"
	cfg, err := Load(base, cwd)
	require.NoError(t, err)
	require.Equal(t, 90, cfg.BashTimeoutSec)
	require.Equal(t, 90, cfg.BashTimeoutSeconds())
}

func TestLoadInvalidJSONReportsFilePath(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, ".goclaw")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	badPath := filepath.Join(userDir, "settings.json")
	require.NoError(t, os.WriteFile(badPath, []byte(`{"provider":`), 0o600))
	base := Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"
	_, err := Load(base, cwd)
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, badPath)
	require.Contains(t, msg, "parse settings")
}

func TestLoadBashTimeoutSecZeroFallsBackToDefault(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, ".goclaw")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"bash_timeout_sec":0}`), 0o600))
	base := Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"
	cfg, err := Load(base, cwd)
	require.NoError(t, err)
	require.Equal(t, 0, cfg.BashTimeoutSec)
	require.Equal(t, tools.BashTimeoutSec, cfg.BashTimeoutSeconds())
}

func TestLoadBashTimeoutSecAboveMaxClampedInBashTimeoutSeconds(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, ".goclaw")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"bash_timeout_sec":3601}`), 0o600))
	base := Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"
	cfg, err := Load(base, cwd)
	require.NoError(t, err)
	require.Equal(t, 3601, cfg.BashTimeoutSec)
	require.Equal(t, 3600, cfg.BashTimeoutSeconds())
}
