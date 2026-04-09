// Package config loads goclaw configuration from the hierarchy:
// defaults → user/project settings.json → user/project settings.local.json → CLI flags.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/tools"
)

// Config holds all runtime settings for goclaw.
type Config struct {
	// LLM provider — set Provider to select which client to build.
	// "ollama"    → local Ollama (default)
	// "anthropic" → Anthropic API (requires APIKey)
	Provider string

	// Ollama settings
	OllamaHost  string // default: http://localhost:11434
	OllamaModel string // default: qwen2.5-coder:14b

	// Anthropic settings (only used when Provider == "anthropic")
	APIKey  string // ANTHROPIC_API_KEY env var
	BaseURL string // override for mock server

	// Context & compaction
	AutoCompactThreshold float64 // fraction of context used before compacting (e.g. 0.85)

	// Paths
	UserConfigDir    string // ~/.goclaw
	ProjectConfigDir string // .goclaw (relative to cwd)

	// AgentProfile is a key from agents.All() (e.g. general-purpose, explore).
	AgentProfile string

	// PermissionModes maps tool name → mode string from JSON ("ask"|"allow"|"deny").
	// Populated by Load; applied to permissions.Policy in cmd.
	PermissionModes map[string]string

	// BashTimeoutSec caps bash tool execution; 0 means use internal/tools.BashTimeoutSec (default 30).
	// Values above 3600 are clamped when BashTimeoutSeconds() is used.
	BashTimeoutSec int

	// ModelContextTokens overrides the provider-default context window estimate used for compaction.
	// 0 = use built-in default: anthropic=200_000, ollama=32_000.
	// Set in settings.json as "model_context_tokens" when using a non-standard Ollama model.
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
	// pipes, &&, and redirections. Default false (opt-in via allow_script: true).
	AllowScript bool

	// YoloThreshold auto-approves tool calls with a risk score at or below this value.
	// -1 (default) disables auto-approval; 0 auto-approves only pure reads.
	// Range: -1..100.
	YoloThreshold int

	// LLMCompaction uses the active LLM to summarize compacted context instead of
	// the heuristic placeholder. Default false (opt-in via llm_compaction: true).
	LLMCompaction bool

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

	// TokenCountMode controls session size estimation for compaction when provider is anthropic.
	// "auto" (default) uses the Anthropic count_tokens API once the heuristic estimate crosses 70% of the compact threshold.
	// "heuristic" always uses the character-based estimate (legacy behavior).
	TokenCountMode string

	// PluginDirs lists absolute or relative plugin roots (manifest + optional hooks). Merged from settings; CLI can append.
	PluginDirs []string
	// PluginAllow if non-empty: only manifests with Name in this list load (after deny).
	PluginAllow []string
	// PluginDeny: manifests with Name in this list never load (deny wins).
	PluginDeny []string

	// MemoryAutoExtract when true: after successful write_file/edit_file, append a short project memory line (path only).
	MemoryAutoExtract bool
}

// MCPServerConfig describes one MCP server (stdio subprocess and/or Streamable HTTP URL).
// If both URL and Command are set, URL (HTTP) takes precedence.
type MCPServerConfig struct {
	ID       string            `json:"id"`
	Command  string            `json:"command"`
	Args     []string          `json:"args"`
	Env      map[string]string `json:"env,omitempty"`
	CWD      string            `json:"cwd,omitempty"`
	URL      string            `json:"url,omitempty"` // Streamable HTTP MCP endpoint
	Headers  map[string]string `json:"headers,omitempty"`
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
func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Provider:             "ollama",
		OllamaHost:           envOr("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:          envOr("OLLAMA_MODEL", "qwen2.5-coder:14b"),
		APIKey:               os.Getenv("ANTHROPIC_API_KEY"),
		BaseURL:              envOr("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		AutoCompactThreshold: 0.85,
		UserConfigDir:        filepath.Join(home, ".goclaw"),
		ProjectConfigDir:     ".goclaw",
		AgentProfile:           "general-purpose",
		PermissionModes:        nil,
		YoloThreshold:          -1,
		WebSearchBackend:       "ddg",
		BraveSearchAPIKey:      os.Getenv("BRAVE_SEARCH_API_KEY"),
		SerpAPIKey:             os.Getenv("SERPAPI_API_KEY"),
		WebSearchFallbackDDG:   true,
		TokenCountMode:           "auto",
	}
}

// anthropicModelAliases maps short CLI-style names to full Anthropic model ids
// (aligned with common claw-code style aliases; unknown values pass through unchanged).
var anthropicModelAliases = map[string]string{
	"opus":   "claude-opus-4-6",
	"sonnet": "claude-sonnet-4-6",
	"haiku":  "claude-haiku-4-5-20251213",
}

func resolveAnthropicModelName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if id, ok := anthropicModelAliases[strings.ToLower(raw)]; ok {
		return id
	}
	return raw
}

// Model returns the model name to pass to the active provider.
func (c Config) Model() string {
	if c.Provider == "anthropic" {
		return resolveAnthropicModelName(envOr("GOCLAW_MODEL", "claude-sonnet-4-6"))
	}
	return c.OllamaModel
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
