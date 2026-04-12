package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
)

func TestLastUserNaturalText(t *testing.T) {
	t.Parallel()
	msgs := []llm.Message{
		llm.PlainMessage("user", "first"),
		llm.PlainMessage("assistant", "ok"),
		{Role: "user", ToolResults: []llm.ToolResultRecord{{Content: "x"}}},
		llm.PlainMessage("user", "hola"),
	}
	if got := lastUserNaturalText(msgs); got != "hola" {
		t.Fatalf("got %q, want hola", got)
	}
}

func TestClassifyUserLanguage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want string
	}{
		{"", ""},
		{"refactor the auth module", ""},
		{"hola", "es"},
		{"hola que tal?", "es"},
		{"¡Hola! ¿Qué tal?", "es"},
		{"Gracias por la ayuda", "es"},
		{"bonjour et merci", "fr"},
		{"hallo können Sie helfen", "de"},
		{"olá obrigado", "pt"},
		{`{"foo":1}`, ""},
	}
	for _, tt := range tests {
		if got := classifyUserLanguage(tt.text); got != tt.want {
			t.Errorf("classifyUserLanguage(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func TestUserLanguageSystemSuffix(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	s := userLanguageSystemSuffix("hola", cfg)
	if s == "" || !strings.Contains(s, "Runtime user-language hint") || !strings.Contains(s, "Spanish") {
		t.Fatalf("unexpected suffix: %q", s)
	}
	if s := userLanguageSystemSuffix("hola que tal?", cfg); s == "" || !strings.Contains(s, "Spanish") {
		t.Fatalf("expected Spanish hint for colloquial greeting: %q", s)
	}
	if userLanguageSystemSuffix("ok", cfg) != "" {
		t.Fatal("expected no hint for ambiguous short message")
	}
}

func TestUserLanguageSystemSuffixPreferredOverrides(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.PreferredResponseLanguage = "es"
	h := userLanguageSystemSuffix("hello friend how are you today", cfg)
	if !strings.Contains(h, "Preferred response language") || !strings.Contains(h, "Spanish") {
		t.Fatalf("want settings-driven Spanish hint: %q", h)
	}
}

func TestUserLanguageSystemSuffixFromOSLocale(t *testing.T) {
	// Do not use t.Parallel: mutates process environment.
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	t.Setenv("LANG", "")
	t.Setenv("LC_MESSAGES", "")
	cfg := config.Default()
	cfg.PreferredResponseLanguage = "from_os"
	h := userLanguageSystemSuffix("z", cfg)
	if !strings.Contains(h, "German") {
		t.Fatalf("want locale fallback hint: %q", h)
	}
}

func TestUserLanguageSystemSuffixWhatlanggoEnglish(t *testing.T) {
	cfg := config.Default()
	text := "The quick brown fox jumps over the lazy dog. This sentence is long enough for statistical detection and is clearly English."
	h := userLanguageSystemSuffix(text, cfg)
	if !strings.Contains(h, "English") || !strings.Contains(h, "latest user-written message") {
		t.Fatalf("want whatlanggo English hint: %q", h)
	}
}

func TestBuildRequestAppendsRuntimeLanguageHint(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeLangHintTool{})

	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.Explore,
	}
	o.session.Add("user", "hola")
	req := o.buildRequest()
	if !strings.Contains(req.System, "Runtime user-language hint") || !strings.Contains(req.System, "Spanish") {
		t.Fatalf("system missing language hint, tail: %q", tail(req.System, 400))
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

type fakeLangHintTool struct{}

func (fakeLangHintTool) Name() string                       { return "read_file" }
func (fakeLangHintTool) Description() string                 { return "fake" }
func (fakeLangHintTool) InputSchema() any                    { return map[string]any{"type": "object"} }
func (fakeLangHintTool) Execute(context.Context, string) (tools.Result, error) {
	return tools.Result{}, nil
}
