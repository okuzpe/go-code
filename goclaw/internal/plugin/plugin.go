// Package plugin loads local plugin manifests (V3): hooks and future surfaces (MCP, agents).
// A plugin is a directory containing goclaw-plugin.json. Supply-chain risk matches trusted_workspace hooks.
package plugin

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/hooks"
)

// Manifest is goclaw-plugin.json at the plugin root.
type Manifest struct {
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	HooksFile string `json:"hooks_file,omitempty"`
}

func norm(s string) string { return strings.TrimSpace(strings.ToLower(s)) }

// Allowed returns whether a plugin name passes allow/deny lists (deny wins; empty allow = all except denied).
func Allowed(name string, allow, deny []string) bool {
	n := norm(name)
	if n == "" {
		return false
	}
	for _, d := range deny {
		if norm(d) == n {
			return false
		}
	}
	if len(allow) == 0 {
		return true
	}
	for _, a := range allow {
		if norm(a) == n {
			return true
		}
	}
	return false
}

func resolveDir(workdir, d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return ""
	}
	if filepath.IsAbs(d) {
		return filepath.Clean(d)
	}
	return filepath.Clean(filepath.Join(workdir, d))
}

// RegisterHooksFromDirs loads goclaw-plugin.json in each directory entry (not recursive).
// Returns plugin names that registered at least one hook file load attempt (manifest valid + allowed).
func RegisterHooksFromDirs(reg *hooks.Registry, dirs []string, workdir string, allow, deny []string) []string {
	var loaded []string
	for _, d := range dirs {
		abs := resolveDir(workdir, d)
		if abs == "" {
			continue
		}
		manPath := filepath.Join(abs, "goclaw-plugin.json")
		raw, err := os.ReadFile(manPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			slog.Warn("plugin manifest read failed", "path", manPath, "err", err)
			continue
		}
		var man Manifest
		if err := json.Unmarshal(raw, &man); err != nil {
			slog.Warn("plugin manifest parse failed", "path", manPath, "err", err)
			continue
		}
		if strings.TrimSpace(man.Name) == "" {
			slog.Warn("plugin manifest missing name", "path", manPath)
			continue
		}
		if !Allowed(man.Name, allow, deny) {
			slog.Debug("plugin skipped by allow/deny", "name", man.Name)
			continue
		}
		hf := strings.TrimSpace(man.HooksFile)
		if hf == "" {
			hf = "hooks.json"
		}
		hookPath := filepath.Join(abs, hf)
		if err := hooks.LoadHooksFile(reg, hookPath); err != nil {
			slog.Warn("plugin hooks load failed", "plugin", man.Name, "path", hookPath, "err", err)
			continue
		}
		loaded = append(loaded, man.Name)
	}
	return loaded
}

// LoadManifest reads and validates a manifest file (for tests).
func LoadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var man Manifest
	if err := json.Unmarshal(raw, &man); err != nil {
		return Manifest{}, err
	}
	if strings.TrimSpace(man.Name) == "" {
		return Manifest{}, fmt.Errorf("plugin: empty name")
	}
	return man, nil
}
