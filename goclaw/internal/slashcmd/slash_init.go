package slashcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
)

const claudeStub = `# Project rules for AI agents

Keep this file in **English**. Add conventions, test commands, and anything agents should follow in this repository.

`

const gitignoreLocalSettingsBlock = "\n\n# goclaw — machine-local settings (do not commit)\n.goclaw/settings.local.json\n"

// handleSlashProjectInit writes a starter .goclaw/settings.json when missing (coordinator-hub-oriented defaults).
// Merged config and permissions load at process start; tell the user to restart for tool_permissions,
// and /profile coordinator for an immediate profile switch in this session if needed.
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
	// tool_permissions: allow pure reads; ask for shell, network, writes, and hub tools (safer default for local agents).
	patch := map[string]any{
		"agent_profile":          "coordinator",
		"provider":               "ollama",
		"ollama_model":           config.DefaultOllamaModel,
		"ollama_num_ctx":         config.DefaultOllamaNumCtx,
		"model_context_tokens":   config.DefaultOllamaNumCtx,
		"llm_compaction":         true,
		"compaction_model":       "qwen2.5-coder:7b",
		"task_model_router":      "rules",
		"task_models": map[string]any{
			"default":   "qwen2.5-coder:7b",
			"code":      "qwen2.5-coder:14b",
			"reasoning": "qwen2.5-coder:14b",
			"explore":   "qwen2.5-coder:7b",
			"fast":      "qwen2.5-coder:7b",
			"creative":  "qwen2.5-coder:14b",
		},
		"auto_profile_intent":        "off",
		"auto_direct_coding_profile": "off",
		"action_repair_escalation":   false,
		"tool_permissions": map[string]any{
			"read_file":    "allow",
			"glob":         "allow",
			"grep":         "allow",
			"bash":         "ask",
			"web_fetch":    "ask",
			"web_search":   "ask",
			"write_file":   "ask",
			"edit_file":    "ask",
			"patch":        "ask",
			"spawn_agent":  "ask",
			"stop_task":    "ask",
			"todo_write":   "ask",
		},
	}
	if err := config.MergeWriteSettings(path, patch); err != nil {
		return "", fmt.Errorf("/init: write settings: %w", err)
	}
	var notes []string
	claudePath := filepath.Join(wd, "CLAUDE.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		if werr := os.WriteFile(claudePath, []byte(claudeStub), 0o600); werr != nil {
			notes = append(notes, fmt.Sprintf("CLAUDE.md not created: %v", werr))
		} else {
			notes = append(notes, "created "+claudePath+" (starter template)")
		}
	} else if err != nil {
		notes = append(notes, fmt.Sprintf("CLAUDE.md skipped: stat: %v", err))
	}
	gi := filepath.Join(wd, ".gitignore")
	if raw, err := os.ReadFile(gi); err == nil {
		body := string(raw)
		if !strings.Contains(body, ".goclaw/settings.local.json") {
			if err := os.WriteFile(gi, append(raw, []byte(gitignoreLocalSettingsBlock)...), 0o600); err != nil {
				notes = append(notes, fmt.Sprintf(".gitignore not updated: %v", err))
			} else {
				notes = append(notes, "appended .goclaw/settings.local.json to .gitignore")
			}
		}
	}
	msg := fmt.Sprintf("created %s\nfor profile coordinator in this session, run: /profile coordinator\ntool permissions from the new file apply after you restart goclaw.", path)
	if len(notes) > 0 {
		msg += "\n" + strings.Join(notes, "\n")
	}
	return msg, nil
}
