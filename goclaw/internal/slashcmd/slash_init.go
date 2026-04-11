package slashcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
)

// handleSlashProjectInit writes a starter .goclaw/settings.json when missing (coding-oriented defaults).
// Merged config and permissions load at process start; tell the user to restart for tool_permissions,
// and /profile general-purpose for an immediate profile switch in this session.
func handleSlashProjectInit(env SlashEnv) (string, error) {
	wd := strings.TrimSpace(env.Workdir)
	if wd == "" {
		return "", fmt.Errorf("/init: workspace directory not set")
	}
	path := config.ProjectSettingsPath(wd, ".goclaw")
	_, statErr := os.Stat(path)
	if statErr == nil {
		return fmt.Sprintf("already exists: %s\nedit that file or remove it to regenerate.", path), nil
	}
	if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("/init: stat settings: %w", statErr)
	}
	// tool_permissions: allow pure reads; ask for shell, network, and writes (safer default for local agents).
	patch := map[string]any{
		"agent_profile": "general-purpose",
		"provider":      "ollama",
		"ollama_model":  "qwen2.5-coder:14b",
		"tool_permissions": map[string]any{
			"read_file":  "allow",
			"glob":       "allow",
			"grep":       "allow",
			"bash":       "ask",
			"web_fetch":  "ask",
			"web_search": "ask",
			"write_file": "ask",
			"edit_file":  "ask",
			"patch":      "ask",
		},
	}
	if err := config.MergeWriteSettings(path, patch); err != nil {
		return "", fmt.Errorf("/init: write settings: %w", err)
	}
	return fmt.Sprintf("created %s\nfor profile general-purpose in this session, run: /profile general-purpose\ntool permissions from the new file apply after you restart goclaw.", path), nil
}
