package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// settingsFile is the JSON shape for ~/.goclaw/settings.json and .goclaw/settings.json.
type settingsFile struct {
	Provider                  *string             `json:"provider"`
	OllamaHost                *string             `json:"ollama_host"`
	OllamaModel               *string             `json:"ollama_model"`
	OllamaNumCtx              *int                `json:"ollama_num_ctx,omitempty"`
	CompactionModel           *string             `json:"compaction_model,omitempty"`
	TaskModelRouter           *string             `json:"task_model_router,omitempty"`
	TaskModels                map[string]string   `json:"task_models,omitempty"`
	TaskModelRouterModel      *string             `json:"task_model_router_model,omitempty"`
	PreferredResponseLanguage *string             `json:"preferred_response_language,omitempty"`
	AgentProfile              *string             `json:"agent_profile"`
	AutoCompactThreshold      *float64            `json:"auto_compact_threshold"`
	BashTimeoutSec            *int                `json:"bash_timeout_sec"`
	ModelContextTokens        *int                `json:"model_context_tokens"`
	MaxResponseTokens         *int                `json:"max_response_tokens,omitempty"`
	MCPServers                []MCPServerConfig   `json:"mcp_servers"`
	TrustedWorkspace          *bool               `json:"trusted_workspace"`
	ExternalHooks             []ExternalHookEntry `json:"external_hooks"`
	ToolPermissions           map[string]string   `json:"tool_permissions"`
	AllowScript               *bool               `json:"allow_script"`
	YoloThreshold             *int                `json:"yolo_threshold"`
	LLMCompaction             *bool               `json:"llm_compaction"`
	MCPServersAllowRemote     *bool               `json:"mcp_allow_remote_urls,omitempty"`
	IDEBridgeMCP              *bool               `json:"ide_bridge_mcp,omitempty"`
	WebSearchBackend          *string             `json:"web_search_backend,omitempty"`
	BraveSearchAPIKey         *string             `json:"brave_search_api_key,omitempty"`
	SerpAPIKey                *string             `json:"serpapi_api_key,omitempty"`
	WebSearchFallbackDDG      *bool               `json:"web_search_fallback_ddg,omitempty"`
	PluginDirs                []string            `json:"plugin_dirs,omitempty"`
	PluginAllow               []string            `json:"plugin_allow,omitempty"`
	PluginDeny                []string            `json:"plugin_deny,omitempty"`
	MemoryAutoExtract         *bool               `json:"memory_auto_extract,omitempty"`
	MemoryLLMSilentExtract    *bool               `json:"memory_llm_silent_extract,omitempty"`
	TUIMouseScroll            *bool               `json:"tui_mouse_scroll,omitempty"`
	UIAppearance              *string             `json:"ui_appearance,omitempty"`
	ToolWorkspaceRoot         *string             `json:"tool_workspace_root,omitempty"`
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
	if sf.OllamaNumCtx != nil && *sf.OllamaNumCtx >= 0 {
		cfg.OllamaNumCtx = *sf.OllamaNumCtx
	}
	if sf.CompactionModel != nil && strings.TrimSpace(*sf.CompactionModel) != "" {
		cfg.CompactionModel = strings.TrimSpace(*sf.CompactionModel)
	}
	if sf.TaskModelRouter != nil && strings.TrimSpace(*sf.TaskModelRouter) != "" {
		cfg.TaskModelRouter = NormalizeTaskModelRouter(*sf.TaskModelRouter)
	}
	if len(sf.TaskModels) > 0 {
		if cfg.TaskModels == nil {
			cfg.TaskModels = make(map[string]string)
		}
		for k, v := range sf.TaskModels {
			kk := strings.ToLower(strings.TrimSpace(k))
			if kk == "" {
				continue
			}
			if vv := strings.TrimSpace(v); vv != "" {
				cfg.TaskModels[kk] = vv
			}
		}
	}
	if sf.TaskModelRouterModel != nil && strings.TrimSpace(*sf.TaskModelRouterModel) != "" {
		cfg.TaskModelRouterModel = strings.TrimSpace(*sf.TaskModelRouterModel)
	}
	if sf.PreferredResponseLanguage != nil && strings.TrimSpace(*sf.PreferredResponseLanguage) != "" {
		cfg.PreferredResponseLanguage = strings.TrimSpace(*sf.PreferredResponseLanguage)
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
	if sf.MaxResponseTokens != nil && *sf.MaxResponseTokens > 0 {
		cfg.MaxResponseTokens = *sf.MaxResponseTokens
	}
	if len(sf.MCPServers) > 0 {
		cfg.MCPServers = mergeMCPServersByID(cfg.MCPServers, sf.MCPServers)
	}
	if sf.TrustedWorkspace != nil && *sf.TrustedWorkspace {
		cfg.TrustedWorkspace = true
	}
	if sf.AllowScript != nil {
		cfg.AllowScript = *sf.AllowScript
	}
	if sf.YoloThreshold != nil {
		cfg.YoloThreshold = *sf.YoloThreshold
	}
	if sf.LLMCompaction != nil && *sf.LLMCompaction {
		cfg.LLMCompaction = true
	}
	if sf.MCPServersAllowRemote != nil && *sf.MCPServersAllowRemote {
		cfg.MCPServersAllowRemote = true
	}
	if sf.IDEBridgeMCP != nil && *sf.IDEBridgeMCP {
		cfg.IDEBridgeMCP = true
	}
	if sf.WebSearchBackend != nil {
		cfg.WebSearchBackend = strings.TrimSpace(*sf.WebSearchBackend)
	}
	if sf.BraveSearchAPIKey != nil && strings.TrimSpace(*sf.BraveSearchAPIKey) != "" {
		cfg.BraveSearchAPIKey = strings.TrimSpace(*sf.BraveSearchAPIKey)
	}
	if sf.SerpAPIKey != nil && strings.TrimSpace(*sf.SerpAPIKey) != "" {
		cfg.SerpAPIKey = strings.TrimSpace(*sf.SerpAPIKey)
	}
	if sf.WebSearchFallbackDDG != nil {
		cfg.WebSearchFallbackDDG = *sf.WebSearchFallbackDDG
	}
	if len(sf.PluginDirs) > 0 {
		cfg.PluginDirs = append(cfg.PluginDirs, sf.PluginDirs...)
	}
	if len(sf.PluginAllow) > 0 {
		cfg.PluginAllow = append(cfg.PluginAllow, sf.PluginAllow...)
	}
	if len(sf.PluginDeny) > 0 {
		cfg.PluginDeny = append(cfg.PluginDeny, sf.PluginDeny...)
	}
	if sf.MemoryAutoExtract != nil && *sf.MemoryAutoExtract {
		cfg.MemoryAutoExtract = true
	}
	if sf.MemoryLLMSilentExtract != nil && *sf.MemoryLLMSilentExtract {
		cfg.MemoryLLMSilentExtract = true
	}
	if sf.TUIMouseScroll != nil {
		cfg.TUIMouseScroll = *sf.TUIMouseScroll
	}
	if len(sf.ExternalHooks) > 0 {
		cfg.ExternalHooks = append(cfg.ExternalHooks, sf.ExternalHooks...)
	}
	if sf.UIAppearance != nil && strings.TrimSpace(*sf.UIAppearance) != "" {
		cfg.UIAppearance = NormalizeUIAppearance(*sf.UIAppearance)
	}
	if sf.ToolWorkspaceRoot != nil {
		cfg.ToolWorkspaceRoot = strings.TrimSpace(*sf.ToolWorkspaceRoot)
	}
	maps.Copy(perms, sf.ToolPermissions)
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
