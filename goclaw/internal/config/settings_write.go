package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

// MergeWriteSettings merges patch into the JSON object at path (creating the file and parent dirs if needed).
// Unknown top-level keys in an existing file are preserved. Atomic replace via temp file in the same directory.
func MergeWriteSettings(path string, patch map[string]any) error {
	if path == "" {
		return fmt.Errorf("merge write settings: empty path")
	}
	raw := make(map[string]json.RawMessage)
	if existing, err := os.ReadFile(path); err == nil {
		existing = trimBOMAndSpace(existing)
		if len(existing) > 0 {
			if err := json.Unmarshal(existing, &raw); err != nil {
				return fmt.Errorf("parse settings %s: %w", path, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read settings %s: %w", path, err)
	}
	for key, value := range patch {
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshal settings key %q: %w", key, err)
		}
		raw[key] = json.RawMessage(encoded)
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	out = append(out, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return writeFileAtomic(path, out, 0o600)
}

func trimBOMAndSpace(b []byte) []byte {
	b = trimSpaceBytes(b)
	if len(b) >= len(utf8BOM) && bytes.HasPrefix(b, utf8BOM) {
		b = b[len(utf8BOM):]
	}
	return trimSpaceBytes(b)
}

func trimSpaceBytes(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t' || b[0] == '\n' || b[0] == '\r') {
		b = b[1:]
	}
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '\t' || b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".goclaw-settings-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp settings: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp settings: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp settings: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(tmpPath, mode)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace settings %s: %w", path, err)
	}
	return nil
}

// UserSettingsPath returns ~/.goclaw/settings.json under the given user config dir.
func UserSettingsPath(userConfigDir string) string {
	return filepath.Join(userConfigDir, "settings.json")
}

// UserSettingsLocalPath returns ~/.goclaw/settings.local.json.
func UserSettingsLocalPath(userConfigDir string) string {
	return filepath.Join(userConfigDir, "settings.local.json")
}

// ProjectSettingsPath returns cwd/.goclaw/settings.json.
func ProjectSettingsPath(cwd, projectConfigDir string) string {
	return filepath.Join(cwd, projectConfigDir, "settings.json")
}
