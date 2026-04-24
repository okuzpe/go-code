package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/ui/icons"
)

// settingsFile is the JSON shape for ~/.goclaw/settings.json and .goclaw/settings.json.
type settingsFile struct {
	Provider                             *string             `json:"provider"`
	OllamaHost                           *string             `json:"ollama_host"`
	OllamaModel                          *string             `json:"ollama_model"`
	OllamaNumCtx                         *int                `json:"ollama_num_ctx,omitempty"`
	OllamaHTTPTimeoutSec                 *int                `json:"ollama_http_timeout_sec,omitempty"`
	CompactionModel                      *string             `json:"compaction_model,omitempty"`
	TaskModelRouter                      *string             `json:"task_model_router,omitempty"`
	TaskModels                           map[string]string   `json:"task_models,omitempty"`
	TaskModelRouterModel                 *string             `json:"task_model_router_model,omitempty"`
	PreferredResponseLanguage            *string             `json:"preferred_response_language,omitempty"`
	AgentProfile                         *string             `json:"agent_profile"`
	AutoProfileIntent                    *string             `json:"auto_profile_intent,omitempty"`
	AutoDirectCodingProfile              *string             `json:"auto_direct_coding_profile,omitempty"`
	ActionRepairEscalation               *bool               `json:"action_repair_escalation,omitempty"`
	AutoCompactThreshold                 *float64            `json:"auto_compact_threshold"`
	MicroCompactThreshold                *float64            `json:"micro_compact_threshold,omitempty"`
	BashTimeoutSec                       *int                `json:"bash_timeout_sec"`
	ModelContextTokens                   *int                `json:"model_context_tokens"`
	MaxResponseTokens                    *int                `json:"max_response_tokens,omitempty"`
	MaxOrchestratorIterations            *int                `json:"max_orchestrator_iterations,omitempty"`
	MaxOrchestratorToolCalls             *int                `json:"max_orchestrator_tool_calls,omitempty"`
	MCPServers                           []MCPServerConfig   `json:"mcp_servers"`
	TrustedWorkspace                     *bool               `json:"trusted_workspace"`
	ExternalHooks                        []ExternalHookEntry `json:"external_hooks"`
	ToolPermissions                      map[string]string   `json:"tool_permissions"`
	AllowScript                          *bool               `json:"allow_script"`
	YoloThreshold                        *int                `json:"yolo_threshold"`
	LLMCompaction                        *bool               `json:"llm_compaction"`
	AutoContinueActionRequests           *bool               `json:"auto_continue_action_requests,omitempty"`
	AutoContinueActionMaxNudges          *int                `json:"auto_continue_action_max_nudges,omitempty"`
	TruthFooterNoWorkspaceWrites         *bool               `json:"truth_footer_no_workspace_writes,omitempty"`
	SkillsMaxRunes               *int `json:"skills_max_runes,omitempty"`
	MaxMemorySnippetEntries      *int `json:"max_memory_snippet_entries,omitempty"`
	ProjectContextClaudeMdLines          *int                `json:"project_context_claude_md_lines,omitempty"`
	ProjectContextStandingOrdersPath     *string             `json:"project_context_standing_orders_path,omitempty"`
	ProjectContextStandingOrdersMaxLines *int                `json:"project_context_standing_orders_max_lines,omitempty"`
	MCPServersAllowRemote                *bool               `json:"mcp_allow_remote_urls,omitempty"`
	IDEBridgeMCP                         *bool               `json:"ide_bridge_mcp,omitempty"`
	WebSearchBackend                     *string             `json:"web_search_backend,omitempty"`
	BraveSearchAPIKey                    *string             `json:"brave_search_api_key,omitempty"`
	SerpAPIKey                           *string             `json:"serpapi_api_key,omitempty"`
	WebSearchFallbackDDG                 *bool               `json:"web_search_fallback_ddg,omitempty"`
	PluginDirs                           []string            `json:"plugin_dirs,omitempty"`
	PluginAllow                          []string            `json:"plugin_allow,omitempty"`
	PluginDeny                           []string            `json:"plugin_deny,omitempty"`
	EnableReflectionNudge                *bool               `json:"enable_reflection_nudge,omitempty"`
	NormalizeInputLanguage               *bool               `json:"normalize_input_language,omitempty"`
	MemoryAutoExtract                    *bool               `json:"memory_auto_extract,omitempty"`
	MemoryLLMSilentExtract               *bool               `json:"memory_llm_silent_extract,omitempty"`
	TUIMouseScroll                       *bool               `json:"tui_mouse_scroll,omitempty"`
	TUIIcons                             *string             `json:"tui_icons,omitempty"`
	TUIFooterDensity                     *string             `json:"tui_footer_density,omitempty"`
	TUIInteractMode                      *string             `json:"tui_interact_mode,omitempty"`
	TUIChatMaxIterations                 *int                `json:"tui_chat_max_iterations,omitempty"`
	TUIProfileCycle                      []string            `json:"tui_profile_cycle,omitempty"`
	UIAppearance                         *string             `json:"ui_appearance,omitempty"`
	ToolWorkspaceRoot                    *string             `json:"tool_workspace_root,omitempty"`
	PlanRequireApplyApproval             *bool               `json:"plan_require_apply_approval,omitempty"`
	PlanApplyUseCoordinator              *bool               `json:"plan_apply_use_coordinator,omitempty"`
	AgentVerifyAfterWrite                *bool               `json:"agent_verify_after_write,omitempty"`
	ParallelToolBatchMax                 *int                `json:"parallel_tool_batch_max,omitempty"`
	OllamaRequireWireTools               *bool               `json:"ollama_require_wire_tools,omitempty"`
	AgentPickerHiddenProfiles            []string            `json:"agent_picker_hidden_profiles,omitempty"`
	TelegramBotToken                     *string             `json:"telegram_bot_token,omitempty"`
	TelegramBotTokenFile                 *string             `json:"telegram_bot_token_file,omitempty"`
	TelegramAllowedUserIDs               []int64             `json:"telegram_allowed_user_ids,omitempty"`
	TelegramSessionID                    *string             `json:"telegram_session_id,omitempty"`
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
	applyTelegramEnvOverrides(&cfg)
	NormalizeCompactionThresholds(&cfg)
	return cfg, nil
}

// applyTelegramEnvOverrides applies GOCLAW_TELEGRAM_BOT_TOKEN and GOCLAW_TELEGRAM_ALLOWED_USER_IDS after JSON merge.
func applyTelegramEnvOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("GOCLAW_TELEGRAM_BOT_TOKEN")); v != "" {
		cfg.TelegramBotToken = v
	}
	if v := strings.TrimSpace(os.Getenv("GOCLAW_TELEGRAM_ALLOWED_USER_IDS")); v != "" {
		ids := parseTelegramAllowedUserIDsFromEnv(v)
		if len(ids) > 0 {
			cfg.TelegramAllowedUserIDs = ids
		}
	}
}

func parseTelegramAllowedUserIDsFromEnv(raw string) []int64 {
	parts := strings.Split(raw, ",")
	var out []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
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
	if sf.OllamaHTTPTimeoutSec != nil {
		cfg.OllamaHTTPTimeoutSec = *sf.OllamaHTTPTimeoutSec
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
		cfg.AgentProfile = agents.CanonicalProfileName(*sf.AgentProfile)
	}
	if sf.AutoProfileIntent != nil && strings.TrimSpace(*sf.AutoProfileIntent) != "" {
		cfg.AutoProfileIntent = NormalizeAutoProfileIntent(*sf.AutoProfileIntent)
	}
	if sf.AutoDirectCodingProfile != nil && strings.TrimSpace(*sf.AutoDirectCodingProfile) != "" {
		cfg.AutoDirectCodingProfile = NormalizeAutoDirectCodingProfile(*sf.AutoDirectCodingProfile)
	}
	if sf.ActionRepairEscalation != nil && *sf.ActionRepairEscalation {
		cfg.ActionRepairEscalation = true
	}
	if sf.AutoCompactThreshold != nil {
		cfg.AutoCompactThreshold = *sf.AutoCompactThreshold
	}
	if sf.MicroCompactThreshold != nil {
		cfg.MicroCompactThreshold = *sf.MicroCompactThreshold
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
	if sf.MaxOrchestratorIterations != nil && *sf.MaxOrchestratorIterations > 0 {
		n := *sf.MaxOrchestratorIterations
		if n > 256 {
			n = 256
		}
		cfg.MaxOrchestratorIterations = n
	}
	if sf.MaxOrchestratorToolCalls != nil && *sf.MaxOrchestratorToolCalls > 0 {
		n := *sf.MaxOrchestratorToolCalls
		if n > 512 {
			n = 512
		}
		cfg.MaxOrchestratorToolCalls = n
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
	if sf.AutoContinueActionRequests != nil {
		cfg.AutoContinueActionRequests = *sf.AutoContinueActionRequests
	}
	if sf.AutoContinueActionMaxNudges != nil && *sf.AutoContinueActionMaxNudges > 0 {
		cfg.AutoContinueActionMaxNudges = *sf.AutoContinueActionMaxNudges
	}
	if sf.TruthFooterNoWorkspaceWrites != nil {
		cfg.TruthFooterNoWorkspaceWrites = *sf.TruthFooterNoWorkspaceWrites
	}
	if sf.SkillsMaxRunes != nil && *sf.SkillsMaxRunes > 0 {
		cfg.SkillsMaxRunes = *sf.SkillsMaxRunes
	}
	if sf.MaxMemorySnippetEntries != nil && *sf.MaxMemorySnippetEntries > 0 {
		cfg.MaxMemorySnippetEntries = *sf.MaxMemorySnippetEntries
	}
	if sf.ProjectContextClaudeMdLines != nil && *sf.ProjectContextClaudeMdLines > 0 {
		cfg.ProjectContextClaudeMdLines = *sf.ProjectContextClaudeMdLines
	}
	if sf.ProjectContextStandingOrdersPath != nil {
		cfg.ProjectContextStandingOrdersPath = strings.TrimSpace(*sf.ProjectContextStandingOrdersPath)
	}
	if sf.ProjectContextStandingOrdersMaxLines != nil && *sf.ProjectContextStandingOrdersMaxLines > 0 {
		cfg.ProjectContextStandingOrdersMaxLines = *sf.ProjectContextStandingOrdersMaxLines
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
	// EnableReflectionNudge and NormalizeInputLanguage default to true.
	// A settings.json entry of false explicitly disables them; omitting the key keeps the default.
	if sf.EnableReflectionNudge != nil {
		cfg.EnableReflectionNudge = *sf.EnableReflectionNudge
	}
	if sf.NormalizeInputLanguage != nil {
		cfg.NormalizeInputLanguage = *sf.NormalizeInputLanguage
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
	if sf.TUIIcons != nil {
		cfg.TUIIcons = icons.CanonicalTUIIcons(*sf.TUIIcons)
	}
	if sf.TUIFooterDensity != nil {
		cfg.TUIFooterDensity = NormalizeTUIFooterDensity(*sf.TUIFooterDensity)
	}
	if sf.TUIInteractMode != nil {
		cfg.TUIInteractMode = NormalizeTUIInteractMode(*sf.TUIInteractMode)
	}
	if sf.TUIChatMaxIterations != nil && *sf.TUIChatMaxIterations > 0 {
		cfg.TUIChatMaxIterations = *sf.TUIChatMaxIterations
	}
	if sf.OllamaRequireWireTools != nil && *sf.OllamaRequireWireTools {
		cfg.OllamaRequireWireTools = true
	}
	if len(sf.TUIProfileCycle) > 0 {
		cfg.TUIProfileCycle = nil
		for _, s := range sf.TUIProfileCycle {
			s = agents.CanonicalProfileName(s)
			if s != "" {
				cfg.TUIProfileCycle = append(cfg.TUIProfileCycle, s)
			}
		}
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
	if sf.PlanRequireApplyApproval != nil && *sf.PlanRequireApplyApproval {
		cfg.PlanRequireApplyApproval = true
	}
	if sf.PlanApplyUseCoordinator != nil && *sf.PlanApplyUseCoordinator {
		cfg.PlanApplyUseCoordinator = true
	}
	if sf.AgentVerifyAfterWrite != nil {
		cfg.AgentVerifyAfterWrite = *sf.AgentVerifyAfterWrite
	}
	if sf.ParallelToolBatchMax != nil && *sf.ParallelToolBatchMax > 0 {
		cfg.ParallelToolBatchMax = *sf.ParallelToolBatchMax
	}
	if len(sf.AgentPickerHiddenProfiles) > 0 {
		for _, s := range sf.AgentPickerHiddenProfiles {
			s = agents.CanonicalProfileName(s)
			if s != "" {
				cfg.AgentPickerHiddenProfiles = append(cfg.AgentPickerHiddenProfiles, s)
			}
		}
	}
	if sf.TelegramBotToken != nil {
		cfg.TelegramBotToken = strings.TrimSpace(*sf.TelegramBotToken)
	}
	if sf.TelegramBotTokenFile != nil {
		cfg.TelegramBotTokenFile = strings.TrimSpace(*sf.TelegramBotTokenFile)
	}
	if len(sf.TelegramAllowedUserIDs) > 0 {
		cfg.TelegramAllowedUserIDs = append([]int64(nil), sf.TelegramAllowedUserIDs...)
	}
	if sf.TelegramSessionID != nil {
		cfg.TelegramSessionID = strings.TrimSpace(*sf.TelegramSessionID)
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
