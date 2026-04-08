package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/tools"
)

func TestLoadProjectOverridesUser(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()

	userDir := filepath.Join(home, ".goclaw")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	userJSON := filepath.Join(userDir, "settings.json")
	if err := os.WriteFile(userJSON, []byte(`{"agent_profile":"plan","tool_permissions":{"bash":"allow"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	projGoclaw := filepath.Join(cwd, ".goclaw")
	if err := os.MkdirAll(projGoclaw, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projGoclaw, "settings.json"), []byte(`{"agent_profile":"explore","tool_permissions":{"bash":"deny"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	base := config.Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"

	cfg, err := config.Load(base, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentProfile != "explore" {
		t.Fatalf("AgentProfile: got %q", cfg.AgentProfile)
	}
	if cfg.PermissionModes["bash"] != "deny" {
		t.Fatalf("project should override user bash permission: got %v", cfg.PermissionModes)
	}
}

func TestLoadSettingsLocalOverridesProject(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()

	userDir := filepath.Join(home, ".goclaw")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"agent_profile":"plan"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "settings.local.json"), []byte(`{"agent_profile":"guide"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	projGoclaw := filepath.Join(cwd, ".goclaw")
	if err := os.MkdirAll(projGoclaw, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projGoclaw, "settings.json"), []byte(`{"agent_profile":"explore"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projGoclaw, "settings.local.json"), []byte(`{"agent_profile":"verification"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	base := config.Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"

	cfg, err := config.Load(base, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentProfile != "verification" {
		t.Fatalf("project settings.local should win: got %q", cfg.AgentProfile)
	}
}

func TestLoadBashTimeoutSec(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, ".goclaw")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"bash_timeout_sec":90}`), 0o600); err != nil {
		t.Fatal(err)
	}
	base := config.Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"
	cfg, err := config.Load(base, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BashTimeoutSec != 90 {
		t.Fatalf("BashTimeoutSec: got %d want 90", cfg.BashTimeoutSec)
	}
	if cfg.BashTimeoutSeconds() != 90 {
		t.Fatalf("BashTimeoutSeconds: got %d want 90", cfg.BashTimeoutSeconds())
	}
}

func TestLoadInvalidJSONReportsFilePath(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, ".goclaw")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(userDir, "settings.json")
	if err := os.WriteFile(badPath, []byte(`{"provider":`), 0o600); err != nil {
		t.Fatal(err)
	}
	base := config.Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"
	_, err := config.Load(base, cwd)
	if err == nil {
		t.Fatal("expected parse error for truncated JSON")
	}
	msg := err.Error()
	if !strings.Contains(msg, badPath) {
		t.Fatalf("error should include settings path: %v", err)
	}
	if !strings.Contains(msg, "parse settings") {
		t.Fatalf("error should mention parse: %v", err)
	}
}

func TestLoadBashTimeoutSecZeroFallsBackToDefault(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, ".goclaw")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"bash_timeout_sec":0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	base := config.Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"
	cfg, err := config.Load(base, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BashTimeoutSec != 0 {
		t.Fatalf("zero in JSON should not override (loader ignores non-positive): got %d", cfg.BashTimeoutSec)
	}
	if cfg.BashTimeoutSeconds() != tools.BashTimeoutSec {
		t.Fatalf("BashTimeoutSeconds: got %d want default %d", cfg.BashTimeoutSeconds(), tools.BashTimeoutSec)
	}
}

func TestLoadBashTimeoutSecAboveMaxClampedInBashTimeoutSeconds(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, ".goclaw")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(`{"bash_timeout_sec":3601}`), 0o600); err != nil {
		t.Fatal(err)
	}
	base := config.Default()
	base.UserConfigDir = userDir
	base.ProjectConfigDir = ".goclaw"
	cfg, err := config.Load(base, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BashTimeoutSec != 3601 {
		t.Fatalf("stored value should be raw from JSON: got %d", cfg.BashTimeoutSec)
	}
	if cfg.BashTimeoutSeconds() != 3600 {
		t.Fatalf("BashTimeoutSeconds should clamp to 3600: got %d", cfg.BashTimeoutSeconds())
	}
}
