// Package config loads goclaw configuration from the hierarchy:
// defaults → user/project settings.json → user/project settings.local.json → CLI flags.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/okuzpe/goclaw/internal/ui/icons"
)

// DefaultOllamaModel is the built-in Ollama tag when neither OLLAMA_MODEL nor settings.json
// set ollama_model. qwen2.5-coder:14b is the default coding model; use qwen2.5-coder:7b (or
// OLLAMA_MODEL) if you need a smaller VRAM footprint — see defaultTaskModels for lighter roles.
const DefaultOllamaModel = "qwen2.5-coder:14b"

// DefaultOllamaNumCtx is the default context window (tokens) for Ollama and for compaction estimates
// when settings do not override ollama_num_ctx / model_context_tokens.
// Tuned for DefaultOllamaModel (qwen2.5-coder:14b): Qwen2.5-Coder is trained with native context up to
// 32,768 tokens; higher values need YaRN / Modelfile and more VRAM. Lower in settings on low-VRAM hardware.
const DefaultOllamaNumCtx = 32768

// Config holds all runtime settings for goclaw.
type Config struct {
	// Provider is always "ollama" at runtime. Non-ollama values from settings are rejected in app wiring
	// (legacy "anthropic" / "openai_compatible" configs get a clear migration error).
	Provider string

	// Ollama settings
	OllamaHost  string // default: http://localhost:11434
	OllamaModel string // default: DefaultOllamaModel (qwen2.5-coder:14b)
	// OllamaNumCtx sets the context window size sent to Ollama.
	// 0 means use Ollama's model default (often 2048 — too small for tool schemas).
	// Default: DefaultOllamaNumCtx (32K for qwen2.5-coder:14b). Set via settings.json "ollama_num_ctx".
	OllamaNumCtx int

	// OllamaHTTPTimeoutSec is the net/http Client timeout for each Ollama request, including
	// reading the full streaming response body. 0 uses a built-in default (45 minutes).
	// -1 disables the client timeout (no limit; cancellation still follows context).
	// Values above 86400 are clamped to 24 hours. JSON: ollama_http_timeout_sec; env: GOCLAW_OLLAMA_HTTP_TIMEOUT_SEC.
	OllamaHTTPTimeoutSec int

	// CompactionModel overrides the model id for LLM-driven context compaction only (llm_compaction: true).
	// Empty means use Model(). Set via settings.json "compaction_model" or GOCLAW_COMPACTION_MODEL.
	CompactionModel string

	// TaskModelRouter selects how to pick the LLM model id for each user turn from task_models:
	// "off" (default), "rules" (heuristics + profile bias), or "llm" (cheap router call, then task_models lookup).
	// JSON: task_model_router; env: GOCLAW_TASK_MODEL_ROUTER; CLI: --task-model-router.
	TaskModelRouter string

	// TaskModels maps role names to Ollama model tags (e.g. code, reasoning, fast, default).
	// When TaskModelRouter is not off and this map is non-empty, the orchestrator resolves each turn to one role
	// and uses the mapped model instead of Model() unless the agent profile sets model (ModelOverride).
	// JSON: task_models (object).
	TaskModels map[string]string

	// TaskModelRouterModel is the model used only for the "llm" router classification call (short output).
	// Empty means use ModelForCompaction() so a small compaction model can serve as router.
	// JSON: task_model_router_model; env: GOCLAW_TASK_MODEL_ROUTER_MODEL.
	TaskModelRouterModel string

	// PreferredResponseLanguage steers runtime user-language hints in the orchestrator:
	// "auto" (default when empty), "es", "en", "fr", "de", "pt", or "from_os" (use LANG/LC_* as fallback).
	// JSON: preferred_response_language; env: GOCLAW_PREFERRED_RESPONSE_LANGUAGE.
	PreferredResponseLanguage string

	// Context & compaction
	AutoCompactThreshold float64 // fraction of context used before compacting (e.g. 0.85)

	// Paths
	UserConfigDir    string // ~/.goclaw
	ProjectConfigDir string // .goclaw (relative to cwd)

	// ToolWorkspaceRoot sets the default project directory (see ResolveToolWorkspace): prompt
	// context, default glob/grep tree when no narrower path is given, optional extra skill roots.
	// It does not restrict absolute paths. Empty uses the launch working directory.
	// Resolved with ResolveToolWorkspace(launchCwd, ToolWorkspaceRoot). JSON: tool_workspace_root.
	// CLI --workspace and env GOCLAW_TOOL_WORKSPACE override this when set (see app package).
	ToolWorkspaceRoot string

	// AgentProfile is a key from agents.All() (e.g. coordinator, builder, general-purpose, explore).
	AgentProfile string

	// PermissionModes maps tool name → mode string from JSON ("ask"|"allow"|"deny").
	// Populated by Load; applied to permissions.Policy in cmd.
	PermissionModes map[string]string

	// BashTimeoutSec caps bash tool execution; 0 means use internal/tools.BashTimeoutSec (default 30).
	// Values above 3600 are clamped when BashTimeoutSeconds() is used.
	BashTimeoutSec int

	// MaxResponseTokens caps the number of tokens the LLM may generate per turn.
	// 0 (default) uses the built-in per-provider default (4096 for most providers).
	// Increase for long analysis or generation tasks; set via "max_response_tokens" in settings.json.
	MaxResponseTokens int

	// MaxOrchestratorIterations caps LLM↔tool iterations per user message (0 = built-in 32; values above 256 are clamped).
	// JSON: max_orchestrator_iterations.
	MaxOrchestratorIterations int
	// MaxOrchestratorToolCalls caps tool executions per user message (0 = built-in 64; values above 512 are clamped).
	// JSON: max_orchestrator_tool_calls.
	MaxOrchestratorToolCalls int

	// ModelContextTokens overrides the provider-default context window estimate used for compaction.
	// 0 = use built-in default: OllamaNumCtx when > 0, else DefaultOllamaNumCtx.
	// Set in settings.json as "model_context_tokens" when using a non-standard model or remote endpoint.
	ModelContextTokens int

	// MCPServers lists MCP servers (stdio subprocess and/or streamable HTTP); merged by id from settings.
	MCPServers []MCPServerConfig

	// MCPServersAllowRemote permits mcp_servers entries with non-loopback URLs (Streamable HTTP).
	// Default false: only 127.0.0.1, localhost, ::1.
	MCPServersAllowRemote bool

	// IDEBridgeMCP appends an MCP server from ~/.goclaw/ide/*.json lockfiles when present (D21).
	IDEBridgeMCP bool

	// TrustedWorkspace allows loading `.goclaw/hooks.json` from the project (D18).
	TrustedWorkspace bool

	// AllowScript enables the script tool, which allows multi-line shell scripts with
	// pipes, &&, and redirections. Default true; opt-out via allow_script: false in settings.
	AllowScript bool

	// YoloThreshold auto-approves tool calls with a risk score at or below this value.
	// Default (60): auto-approves reads, bash, spawn_agent, and workspace-relative writes.
	// Set to -1 to disable (prompt for everything). Set to 0 to approve only pure reads.
	// Range: -1..100. High-risk operations (script=70, absolute-path writes=90) always prompt at 60.
	YoloThreshold int

	// LLMCompaction uses the active LLM to summarize compacted context instead of
	// the heuristic placeholder. config.Default() enables it; set llm_compaction: false to opt out.
	LLMCompaction bool

	// AutoContinueActionRequests when true: if the user message signals code changes and the model
	// replies prose-only (including the first completion with zero tool calls, or after read-only
	// tools only), inject a short synthetic user line and re-call the LLM (capped per turn).
	// JSON: auto_continue_action_requests.
	AutoContinueActionRequests bool

	// AutoContinueActionMaxNudges caps synthetic "continue with tools" user injections per turn when
	// AutoContinueActionRequests is true. 0 means default 2; values are clamped to 1..5.
	// JSON: auto_continue_action_max_nudges.
	AutoContinueActionMaxNudges int

	// TruthFooterNoWorkspaceWrites when true: if the user message signals code changes, tools ran,
	// writes were allowed, but no successful write_file/edit_file/patch, append a [goclaw] bilingual
	// footer to the assistant reply. JSON: truth_footer_no_workspace_writes (default true).
	TruthFooterNoWorkspaceWrites bool

	// AutoProfileIntent selects profile-type detection before each user send: "off" (default) or "rules".
	// JSON: auto_profile_intent; env: GOCLAW_AUTO_PROFILE_INTENT.
	AutoProfileIntent string
	// AutoDirectCodingProfile when not "off" and the session is coordinator hub mode, may switch the
	// orchestrator to general-purpose or builder for obvious single-session coding (see orchestrator profile intent).
	// JSON: auto_direct_coding_profile; env: GOCLAW_AUTO_DIRECT_CODING_PROFILE.
	AutoDirectCodingProfile string
	// ActionRepairEscalation when true: after action-continue nudges are exhausted with no tools, perform
	// one model escalation (code-role model) before ending the turn. JSON: action_repair_escalation.
	ActionRepairEscalation bool

	// ExternalHooks are subprocess or HTTP hooks from settings (see hooks package).
	ExternalHooks []ExternalHookEntry

	// WebSearchBackend selects the web_search tool HTTP backend: "ddg", "brave", or "serpapi".
	WebSearchBackend string

	// BraveSearchAPIKey is the Brave Search API subscription token (optional; env BRAVE_SEARCH_API_KEY).
	BraveSearchAPIKey string

	// SerpAPIKey is the SerpAPI key (optional; env SERPAPI_API_KEY).
	SerpAPIKey string

	// WebSearchFallbackDDG when true (default) retries via DuckDuckGo if the primary backend fails or returns no results.
	WebSearchFallbackDDG bool

	// PluginDirs lists absolute or relative plugin roots (manifest + optional hooks). Merged from settings; CLI can append.
	PluginDirs []string
	// PluginAllow if non-empty: only manifests with Name in this list load (after deny).
	PluginAllow []string
	// PluginDeny: manifests with Name in this list never load (deny wins).
	PluginDeny []string

	// EnableReflectionNudge when true: after reflectionTriggerRounds (3) read-only tool rounds with no
	// workspace write on a code/fix task, inject a self-assessment prompt asking the model to stop
	// reading and start editing. Lightweight (no extra LLM call). Default false; JSON: enable_reflection_nudge.
	EnableReflectionNudge bool

	// NormalizeInputLanguage when true: before each LLM turn, if the user message is detected as
	// non-English, a lightweight Ollama call translates it to English using the compaction model.
	// The session stores the English version; the language-reply hint still fires so the model
	// answers in the user's language. Default false. JSON: normalize_input_language.
	NormalizeInputLanguage bool

	// MemoryAutoExtract when true: after successful write_file/edit_file, append a short project memory line (path only).
	MemoryAutoExtract bool

	// MemoryLLMSilentExtract when true: after a user turn with no tool calls, run a background LLM pass
	// to optionally append one structured memory entry (see docs/reference/memory-system.md §6).
	// JSON: memory_llm_silent_extract.
	MemoryLLMSilentExtract bool

	// TUIMouseScroll enables mouse-wheel scrolling on the fullscreen chat transcript (Bubble Tea cell motion).
	// Default off so the terminal keeps normal selection / host scroll; set JSON tui_mouse_scroll true or
	// env GOCLAW_TUI_MOUSE_SCROLL=1|true|yes|on to enable wheel on the transcript.
	TUIMouseScroll bool

	// TUIIcons selects footer/workspace glyphs: emoji (default), unicode (legacy ▣), ascii, or nerd (Nerd Fonts PUA).
	// JSON: tui_icons; env: GOCLAW_TUI_ICONS (merged settings override env default from Default()).
	TUIIcons string

	// UIAppearance selects TUI colors and markdown style: auto, dark, light, dark_colorblind, light_colorblind, dark_ansi, light_ansi.
	// JSON key: ui_appearance. Empty or "auto" uses terminal-adaptive styling.
	UIAppearance string

	// PlanRequireApplyApproval when true requires /plan approve (matching plan file hash) before /apply-plan or /plan run execute.
	// JSON: plan_require_apply_approval (default false).
	PlanRequireApplyApproval bool
	// PlanApplyUseCoordinator when true makes /apply-plan and /plan run switch to coordinator and use hub-style handoff (spawn_agent).
	// JSON: plan_apply_use_coordinator (default false).
	PlanApplyUseCoordinator bool
	// AgentPickerHiddenProfiles lists profile names hidden from the TUI Ctrl+P /agents picker (built-in names, lower-case).
	// JSON: agent_picker_hidden_profiles (array of strings).
	AgentPickerHiddenProfiles []string

	// TelegramBotToken is the Bot API token from BotFather (prefer settings.local.json). Env GOCLAW_TELEGRAM_BOT_TOKEN overrides when set.
	TelegramBotToken string
	// TelegramBotTokenFile is a path to a file containing the bot token (single line). Relative paths resolve from launch cwd.
	// JSON: telegram_bot_token_file.
	TelegramBotTokenFile string
	// TelegramAllowedUserIDs is a non-empty allowlist of Telegram user ids that may trigger the bridge (fail-closed when empty).
	// JSON: telegram_allowed_user_ids (array of integers).
	TelegramAllowedUserIDs []int64
	// TelegramSessionID optionally resumes this session id when running goclaw telegram bridge without --session.
	// JSON: telegram_session_id.
	TelegramSessionID string
}

// MCPServerConfig describes one MCP server (stdio subprocess and/or Streamable HTTP URL).
// If both URL and Command are set, URL (HTTP) takes precedence.
type MCPServerConfig struct {
	ID      string            `json:"id"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	CWD     string            `json:"cwd,omitempty"`
	URL     string            `json:"url,omitempty"` // Streamable HTTP MCP endpoint
	Headers map[string]string `json:"headers,omitempty"`
	// BearerTokenFile is read at MCP dial time; contents (trimmed) are sent as Authorization: Bearer.
	// Use a chmod 600 file; prefer over committing tokens. Full OAuth flows are future work (V3 doc).
	BearerTokenFile string `json:"bearer_token_file,omitempty"`
	Disabled        bool   `json:"disabled,omitempty"`
}

// EnvSlice returns env as KEY=value pairs for exec.
func (c MCPServerConfig) EnvSlice() []string {
	if len(c.Env) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.Env))
	for k, v := range c.Env {
		out = append(out, k+"="+v)
	}
	return out
}

// ExternalHookEntry is one hook from settings.json (command or url).
type ExternalHookEntry struct {
	Event   string   `json:"event"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
}

// Default returns a Config that points to a local Ollama instance.
// Multi-model routing is enabled by default: rules router + task_models map so fast/explore/default
// turns use qwen2.5-coder:7b while code, fix, reasoning, and creative turns use the main coder model
// (DefaultOllamaModel / OLLAMA_MODEL, default qwen2.5-coder:14b).
// Any settings.json file can override individual task_models entries without replacing the whole map.
func Default() Config {
	home, _ := os.UserHomeDir()
	cfg := Config{
		Provider:                     "ollama",
		OllamaHost:                   envOr("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:                  envOr("OLLAMA_MODEL", DefaultOllamaModel),
		OllamaNumCtx:                 DefaultOllamaNumCtx,
		CompactionModel:              envOr("GOCLAW_COMPACTION_MODEL", "qwen2.5-coder:7b"),
		TaskModelRouter:              NormalizeTaskModelRouter(envOr("GOCLAW_TASK_MODEL_ROUTER", "rules")),
		TaskModelRouterModel:         envOr("GOCLAW_TASK_MODEL_ROUTER_MODEL", ""),
		TaskModels:                   defaultTaskModels(),
		PreferredResponseLanguage:    envOr("GOCLAW_PREFERRED_RESPONSE_LANGUAGE", ""),
		AutoCompactThreshold:         0.85,
		UserConfigDir:                filepath.Join(home, ".goclaw"),
		ProjectConfigDir:             ".goclaw",
		AgentProfile:                 "coordinator",
		ModelContextTokens:           DefaultOllamaNumCtx,
		PermissionModes:              nil,
		YoloThreshold:                60,
		WebSearchBackend:             "ddg",
		BraveSearchAPIKey:            os.Getenv("BRAVE_SEARCH_API_KEY"),
		SerpAPIKey:                   os.Getenv("SERPAPI_API_KEY"),
		WebSearchFallbackDDG:         true,
		AllowScript:                  true,
		LLMCompaction:                true,
		AutoContinueActionRequests:   true,
		TruthFooterNoWorkspaceWrites: true,
		EnableReflectionNudge:        true,
		NormalizeInputLanguage:       true,
		AutoProfileIntent:            NormalizeAutoProfileIntent(envOr("GOCLAW_AUTO_PROFILE_INTENT", "off")),
		AutoDirectCodingProfile:      NormalizeAutoDirectCodingProfile(envOr("GOCLAW_AUTO_DIRECT_CODING_PROFILE", "off")),
		ActionRepairEscalation:       envTruthy("GOCLAW_ACTION_REPAIR_ESCALATION"),
		TUIMouseScroll:               envTruthy("GOCLAW_TUI_MOUSE_SCROLL"),
		TUIIcons:                     icons.CanonicalTUIIcons(os.Getenv("GOCLAW_TUI_ICONS")),
		UIAppearance:                 "auto",
	}
	if v := strings.TrimSpace(os.Getenv("GOCLAW_OLLAMA_HTTP_TIMEOUT_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.OllamaHTTPTimeoutSec = n
		}
	}
	return cfg
}

const (
	defaultOllamaHTTPTimeoutSec = 2700 // 45 minutes for slow streaming turns
	maxOllamaHTTPTimeoutSec     = 86400
)

// OllamaHTTPClientTimeout returns the http.Client Timeout for Ollama requests.
// Zero uses defaultOllamaHTTPTimeoutSec. -1 means no client timeout (streaming may run until context cancel).
func (c Config) OllamaHTTPClientTimeout() time.Duration {
	sec := c.OllamaHTTPTimeoutSec
	if sec == -1 {
		return 0
	}
	if sec <= 0 {
		return defaultOllamaHTTPTimeoutSec * time.Second
	}
	if sec > maxOllamaHTTPTimeoutSec {
		return maxOllamaHTTPTimeoutSec * time.Second
	}
	return time.Duration(sec) * time.Second
}

// defaultTaskModels returns the built-in per-role model assignments used when task_model_router
// is active. Settings files merge into this map (later entries override individual keys).
//
// Role → model rationale:
//   - fix, code: same as DefaultOllamaModel / OLLAMA_MODEL unless overridden (default qwen2.5-coder:14b)
//   - reasoning, creative: qwen2.5-coder:14b — stronger analysis and generation on those turns
//   - explore, fast, default: qwen2.5-coder:7b — lighter latency for reads and short prompts
//
// Override any role in ~/.goclaw/settings.json under "task_models" without replacing the whole map.
func defaultTaskModels() map[string]string {
	main := envOr("OLLAMA_MODEL", DefaultOllamaModel)
	const coderSmall = "qwen2.5-coder:7b"
	const coderLarge = "qwen2.5-coder:14b"
	return map[string]string{
		"fix":       main,
		"code":      main,
		"reasoning": coderLarge,
		"creative":  coderLarge,
		"research":  coderSmall, // web synthesis is text-heavy, not code-heavy — small model is fine
		"explore":   coderSmall,
		"fast":      coderSmall,
		"default":   coderSmall,
	}
}

// Model returns the Ollama model name to pass to the API.
func (c Config) Model() string {
	return strings.TrimSpace(c.OllamaModel)
}

// ModelForCompaction returns the model id for LLM compaction summaries. When CompactionModel is set,
// it is used so a smaller/faster model can run compaction while Model() serves the main turn.
func (c Config) ModelForCompaction() string {
	if m := strings.TrimSpace(c.CompactionModel); m != "" {
		return c.NormalizeModelForProvider(m)
	}
	return c.Model()
}

// NormalizeModelForProvider trims a user-supplied model id.
func (c Config) NormalizeModelForProvider(raw string) string {
	return strings.TrimSpace(raw)
}

// NormalizeTaskModelRouter returns off, rules, or llm. Unknown values become off.
func NormalizeTaskModelRouter(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "off", "false", "0", "no":
		return "off"
	case "rules", "on", "true", "1", "yes", "heuristic", "heuristics":
		return "rules"
	case "llm":
		return "llm"
	default:
		return "off"
	}
}

// NormalizeAutoProfileIntent returns off or rules. Unknown values become off.
func NormalizeAutoProfileIntent(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "rules", "on", "true", "1", "yes", "heuristic", "heuristics":
		return "rules"
	default:
		return "off"
	}
}

// NormalizeAutoDirectCodingProfile returns off, general-purpose, or builder. Unknown values become off.
func NormalizeAutoDirectCodingProfile(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "general-purpose", "gp", "general":
		return "general-purpose"
	case "builder", "b":
		return "builder"
	default:
		return "off"
	}
}

// TaskModelRoutingActive reports whether per-turn model selection from TaskModels should run.
func (c Config) TaskModelRoutingActive() bool {
	if NormalizeTaskModelRouter(c.TaskModelRouter) == "off" {
		return false
	}
	return len(c.TaskModels) > 0
}

// RouterModelForLLM returns the model id for the optional LLM-based task classifier.
func (c Config) RouterModelForLLM() string {
	if m := strings.TrimSpace(c.TaskModelRouterModel); m != "" {
		return c.NormalizeModelForProvider(m)
	}
	return c.ModelForCompaction()
}

// EffectiveContextTokens returns the context window size (in tokens) used for compaction decisions.
// Priority: explicit ModelContextTokens > provider-aware default (Ollama uses OllamaNumCtx).
// This ensures the compaction threshold matches the context actually sent to the model.
func (c Config) EffectiveContextTokens() int {
	if c.ModelContextTokens > 0 {
		return c.ModelContextTokens
	}
	if c.OllamaNumCtx > 0 {
		return c.OllamaNumCtx
	}
	return DefaultOllamaNumCtx
}

// BashTimeoutSeconds returns the bash tool timeout in seconds (clamped to 1..3600).
func (c Config) BashTimeoutSeconds() int {
	const maxBash = 3600
	d := tools.BashTimeoutSec
	if c.BashTimeoutSec > 0 {
		d = c.BashTimeoutSec
		if d > maxBash {
			d = maxBash
		}
	}
	return d
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envTruthy reports whether key is set to a common affirmative (1, true, yes, on).
func envTruthy(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// NormalizePreferredResponseLanguage returns a canonical mode: auto, from_os, or a supported tag (es|en|fr|de|pt).
// Unknown values are treated as auto.
func NormalizePreferredResponseLanguage(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "", "auto":
		return "auto"
	case "from_os":
		return "from_os"
	case "es", "en", "fr", "de", "pt":
		return s
	default:
		return "auto"
	}
}

// NormalizeWebSearchBackend returns a canonical backend name ("ddg", "brave", "serpapi").
// If raw is unknown, it returns ("ddg", false) so callers can log a warning.
func NormalizeWebSearchBackend(raw string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", "ddg":
		return "ddg", true
	case "brave", "serpapi":
		return v, true
	default:
		return "ddg", false
	}
}
