package config

import (
	"testing"
)

func TestDefaultAgentProfileCoordinator(t *testing.T) {
	t.Parallel()
	if got := Default().AgentProfile; got != "coordinator" {
		t.Fatalf("Default().AgentProfile = %q, want coordinator", got)
	}
}

func TestResolveAnthropicModelName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{"sonnet", "claude-sonnet-4-6"},
		{"SONNET", "claude-sonnet-4-6"},
		{"opus", "claude-opus-4-6"},
		{"haiku", "claude-haiku-4-5-20251213"},
		{"claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"custom-model-id", "custom-model-id"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := resolveAnthropicModelName(tt.in); got != tt.want {
			t.Errorf("resolveAnthropicModelName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestConfigModel_AnthropicAliases(t *testing.T) {
	t.Setenv("GOCLAW_MODEL", "haiku")
	cfg := Default()
	cfg.Provider = "anthropic"
	if got := cfg.Model(); got != "claude-haiku-4-5-20251213" {
		t.Fatalf("Model() = %q, want claude-haiku-4-5-20251213", got)
	}
}

func TestConfigModel_OllamaIgnoresAnthropicAliasEnv(t *testing.T) {
	t.Setenv("GOCLAW_MODEL", "sonnet")
	cfg := Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "qwen2.5-coder:14b"
	if got := cfg.Model(); got != "qwen2.5-coder:14b" {
		t.Fatalf("Model() = %q, want ollama model unchanged", got)
	}
}

func TestNormalizeWebSearchBackend(t *testing.T) {
	tests := []struct {
		raw      string
		want     string
		wantOK   bool
	}{
		{"", "ddg", true},
		{"  ddg ", "ddg", true},
		{"BRAVE", "brave", true},
		{"SerpAPI", "serpapi", true},
		{"unknown-backend", "ddg", false},
	}
	for _, tt := range tests {
		got, ok := NormalizeWebSearchBackend(tt.raw)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("NormalizeWebSearchBackend(%q) = (%q, %v), want (%q, %v)", tt.raw, got, ok, tt.want, tt.wantOK)
		}
	}
}
