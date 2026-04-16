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

func TestDefaultOllamaModelMatchesDefaultConfig(t *testing.T) {
	t.Parallel()
	if got := Default().OllamaModel; got != DefaultOllamaModel {
		t.Fatalf("Default().OllamaModel = %q, want DefaultOllamaModel %q", got, DefaultOllamaModel)
	}
}

func TestEffectiveContextTokens(t *testing.T) {
	t.Parallel()
	var unset Config
	if got := unset.EffectiveContextTokens(); got != DefaultOllamaNumCtx {
		t.Fatalf("zero Config EffectiveContextTokens = %d, want DefaultOllamaNumCtx %d", got, DefaultOllamaNumCtx)
	}
	cfg := Default()
	if got := cfg.EffectiveContextTokens(); got != DefaultOllamaNumCtx {
		t.Fatalf("Default() EffectiveContextTokens = %d, want %d", got, DefaultOllamaNumCtx)
	}
	modelFirst := Config{ModelContextTokens: 8_000, OllamaNumCtx: 9_000}
	if got := modelFirst.EffectiveContextTokens(); got != 8_000 {
		t.Fatalf("ModelContextTokens should win: got %d want 8000", got)
	}
	ollamaOnly := Config{ModelContextTokens: 0, OllamaNumCtx: 50_000}
	if got := ollamaOnly.EffectiveContextTokens(); got != 50_000 {
		t.Fatalf("OllamaNumCtx when ModelContextTokens unset: got %d want 50000", got)
	}
}

func TestConfigModel_OllamaIgnoresGOCLAWModelEnv(t *testing.T) {
	t.Setenv("GOCLAW_MODEL", "ignored-for-ollama")
	cfg := Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = DefaultOllamaModel
	if got := cfg.Model(); got != DefaultOllamaModel {
		t.Fatalf("Model() = %q, want ollama model unchanged", got)
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

func TestLoadPreferredResponseLanguageFromProjectSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projectGoclaw := filepath.Join(dir, ".goclaw")
	if err := os.MkdirAll(projectGoclaw, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projectGoclaw, "settings.json")
	if err := os.WriteFile(path, []byte(`{"preferred_response_language":"from_os"}`), 0o644); err != nil {
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
	if got := cfg.PreferredResponseLanguage; got != "from_os" {
		t.Fatalf("PreferredResponseLanguage = %q, want from_os", got)
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

func TestNormalizePreferredResponseLanguage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{"", "auto"},
		{"auto", "auto"},
		{"AUTO", "auto"},
		{"from_os", "from_os"},
		{"es", "es"},
		{"EN", "en"},
		{"invalid-xl", "auto"},
	}
	for _, tt := range tests {
		if got := NormalizePreferredResponseLanguage(tt.in); got != tt.want {
			t.Errorf("NormalizePreferredResponseLanguage(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeTaskModelRouter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{"", "off"},
		{"off", "off"},
		{"rules", "rules"},
		{"on", "rules"},
		{"llm", "llm"},
		{"bogus", "off"},
	}
	for _, tt := range tests {
		if got := NormalizeTaskModelRouter(tt.in); got != tt.want {
			t.Errorf("NormalizeTaskModelRouter(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestConfigTaskModelRoutingActive(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.TaskModelRouter = "rules"
	cfg.TaskModels = nil
	if cfg.TaskModelRoutingActive() {
		t.Fatal("expected inactive without task_models")
	}
	cfg.TaskModels = map[string]string{"default": "m1"}
	if !cfg.TaskModelRoutingActive() {
		t.Fatal("expected active with router rules + map")
	}
	cfg.TaskModelRouter = "off"
	if cfg.TaskModelRoutingActive() {
		t.Fatal("expected inactive when router off")
	}
}

func TestNormalizeModelForProviderTrims(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if got := cfg.NormalizeModelForProvider("  my:tag  "); got != "my:tag" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadTaskModelsFromProjectSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projectGoclaw := filepath.Join(dir, ".goclaw")
	if err := os.MkdirAll(projectGoclaw, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projectGoclaw, "settings.json")
	raw := `{"task_model_router":"rules","task_models":{"default":"m0","code":"m-coder","Reasoning":"OPUS"}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
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
	if cfg.TaskModelRouter != "rules" {
		t.Fatalf("TaskModelRouter = %q", cfg.TaskModelRouter)
	}
	if cfg.TaskModels["default"] != "m0" || cfg.TaskModels["code"] != "m-coder" || cfg.TaskModels["reasoning"] != "OPUS" {
		t.Fatalf("TaskModels = %#v", cfg.TaskModels)
	}
}

func TestRouterModelForLLM(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "big:latest"
	cfg.CompactionModel = "small:latest"
	if got := cfg.RouterModelForLLM(); got != "small:latest" {
		t.Fatalf("RouterModelForLLM = %q", got)
	}
	cfg.TaskModelRouterModel = "router:8b"
	if got := cfg.RouterModelForLLM(); got != "router:8b" {
		t.Fatalf("RouterModelForLLM = %q", got)
	}
}

func TestNormalizeWebSearchBackend(t *testing.T) {
	tests := []struct {
		raw    string
		want   string
		wantOK bool
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
