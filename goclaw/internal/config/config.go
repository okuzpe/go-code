// Package config loads goclaw configuration from the hierarchy:
// defaults → user/project settings.json → user/project settings.local.json → CLI flags.
package config

import (
	"os"
	"path/filepath"

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
	Disabled bool              `json:"disabled,omitempty"`
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
		AgentProfile:         "general-purpose",
		PermissionModes:      nil,
		YoloThreshold:        -1,
	}
}

// Model returns the model name to pass to the active provider.
func (c Config) Model() string {
	if c.Provider == "anthropic" {
		return envOr("GOCLAW_MODEL", "claude-sonnet-4-6")
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
