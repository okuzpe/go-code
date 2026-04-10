package config

import (
	"os"
	"path/filepath"
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

func TestConfigModel_OpenAICompat(t *testing.T) {
	t.Setenv("OPENAI_MODEL", "")
	cfg := Default()
	cfg.Provider = "openai_compatible"
	cfg.OpenAICompatModel = "openrouter/free"
	if got := cfg.Model(); got != "openrouter/free" {
		t.Fatalf("Model() = %q, want openrouter/free", got)
	}
}

func TestConfigModel_OpenAICompatEnvFallback(t *testing.T) {
	t.Setenv("OPENAI_MODEL", "meta/llama:free")
	cfg := Default()
	cfg.Provider = "openai_compatible"
	cfg.OpenAICompatModel = ""
	if got := cfg.Model(); got != "meta/llama:free" {
		t.Fatalf("Model() = %q, want env fallback", got)
	}
}

func TestConfigModelForCompaction(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "llama3:latest"
	cfg.CompactionModel = "qwen2.5-coder:7b"
	if got := cfg.ModelForCompaction(); got != "qwen2.5-coder:7b" {
		t.Fatalf("ModelForCompaction() = %q, want qwen2.5-coder:7b", got)
	}
	cfg.CompactionModel = ""
	if got := cfg.ModelForCompaction(); got != "llama3:latest" {
		t.Fatalf("ModelForCompaction() with empty CompactionModel = %q, want llama3:latest", got)
	}
}

func TestLoadCompactionModelFromProjectSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projectGoclaw := filepath.Join(dir, ".goclaw")
	if err := os.MkdirAll(projectGoclaw, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projectGoclaw, "settings.json")
	if err := os.WriteFile(path, []byte(`{"compaction_model":"phi3:latest"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	cfg.UserConfigDir = filepath.Join(dir, "user-goclaw-missing")
	cfg.ProjectConfigDir = ".goclaw"
	var err error
	cfg, err = Load(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.CompactionModel; got != "phi3:latest" {
		t.Fatalf("CompactionModel = %q, want phi3:latest", got)
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
