package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// settingsFile is the JSON shape for ~/.goclaw/settings.json and .goclaw/settings.json.
type settingsFile struct {
	Provider             *string           `json:"provider"`
	OllamaHost           *string           `json:"ollama_host"`
	OllamaModel          *string           `json:"ollama_model"`
	AnthropicBaseURL     *string           `json:"anthropic_base_url"`
	AgentProfile         *string           `json:"agent_profile"`
	AutoCompactThreshold *float64          `json:"auto_compact_threshold"`
	BashTimeoutSec       *int                `json:"bash_timeout_sec"`
	ModelContextTokens   *int                `json:"model_context_tokens"`
	MCPServers           []MCPServerConfig   `json:"mcp_servers"`
	TrustedWorkspace     *bool               `json:"trusted_workspace"`
	ExternalHooks        []ExternalHookEntry `json:"external_hooks"`
	ToolPermissions      map[string]string   `json:"tool_permissions"`
	AllowScript          *bool               `json:"allow_script"`
	YoloThreshold        *int                `json:"yolo_threshold"`
	LLMCompaction        *bool               `json:"llm_compaction"`
}

// Load merges JSON settings into base in this order (later wins for overlapping keys):
// user settings.json → project settings.json → user settings.local.json → project settings.local.json.
// Missing files are skipped. tool_permissions merge with later files overriding keys.
func Load(base Config, cwd string) (Config, error) {
	cfg := base
	perms := make(map[string]string)

	userPath := filepath.Join(cfg.UserConfigDir, "settings.json")
	if err := mergeFile(userPath, &cfg, perms); err != nil {
		return cfg, err
	}

	projectPath := filepath.Join(cwd, cfg.ProjectConfigDir, "settings.json")
	if err := mergeFile(projectPath, &cfg, perms); err != nil {
		return cfg, err
	}

	userLocal := filepath.Join(cfg.UserConfigDir, "settings.local.json")
	if err := mergeFile(userLocal, &cfg, perms); err != nil {
		return cfg, err
	}

	projectLocal := filepath.Join(cwd, cfg.ProjectConfigDir, "settings.local.json")
	if err := mergeFile(projectLocal, &cfg, perms); err != nil {
		return cfg, err
	}

	if len(perms) > 0 {
		cfg.PermissionModes = perms
	}
	return cfg, nil
}

// mergeFile reads path if it exists and applies fields to cfg and merges tool_permissions into perms (later files win).
func mergeFile(path string, cfg *Config, perms map[string]string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read settings %s: %w", path, err)
	}

	var sf settingsFile
	if err := json.Unmarshal(raw, &sf); err != nil {
		return fmt.Errorf("parse settings %s: %w", path, err)
	}

	if sf.Provider != nil {
		cfg.Provider = *sf.Provider
	}
	if sf.OllamaHost != nil {
		cfg.OllamaHost = *sf.OllamaHost
	}
	if sf.OllamaModel != nil {
		cfg.OllamaModel = *sf.OllamaModel
	}
	if sf.AnthropicBaseURL != nil {
		cfg.BaseURL = *sf.AnthropicBaseURL
	}
	if sf.AgentProfile != nil && *sf.AgentProfile != "" {
		cfg.AgentProfile = *sf.AgentProfile
	}
	if sf.AutoCompactThreshold != nil {
		cfg.AutoCompactThreshold = *sf.AutoCompactThreshold
	}
	if sf.BashTimeoutSec != nil && *sf.BashTimeoutSec > 0 {
		cfg.BashTimeoutSec = *sf.BashTimeoutSec
	}
	if sf.ModelContextTokens != nil && *sf.ModelContextTokens > 0 {
		cfg.ModelContextTokens = *sf.ModelContextTokens
	}
	if len(sf.MCPServers) > 0 {
		cfg.MCPServers = mergeMCPServersByID(cfg.MCPServers, sf.MCPServers)
	}
	if sf.TrustedWorkspace != nil && *sf.TrustedWorkspace {
		cfg.TrustedWorkspace = true
	}
	if sf.AllowScript != nil && *sf.AllowScript {
		cfg.AllowScript = true
	}
	if sf.YoloThreshold != nil {
		cfg.YoloThreshold = *sf.YoloThreshold
	}
	if sf.LLMCompaction != nil && *sf.LLMCompaction {
		cfg.LLMCompaction = true
	}
	if len(sf.ExternalHooks) > 0 {
		cfg.ExternalHooks = append(cfg.ExternalHooks, sf.ExternalHooks...)
	}

	for k, v := range sf.ToolPermissions {
		perms[k] = v
	}
	return nil
}

func mergeMCPServersByID(existing, incoming []MCPServerConfig) []MCPServerConfig {
	byID := make(map[string]MCPServerConfig)
	for _, s := range existing {
		if s.ID != "" {
			byID[s.ID] = s
		}
	}
	for _, s := range incoming {
		if s.ID != "" {
			byID[s.ID] = s
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]MCPServerConfig, 0, len(ids))
	for _, id := range ids {
		out = append(out, byID[id])
	}
	return out
}
