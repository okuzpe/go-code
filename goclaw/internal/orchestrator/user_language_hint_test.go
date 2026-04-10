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
	s := userLanguageSystemSuffix("hola")
	if s == "" || !strings.Contains(s, "Runtime user-language hint") || !strings.Contains(s, "Spanish") {
		t.Fatalf("unexpected suffix: %q", s)
	}
	if userLanguageSystemSuffix("hello there") != "" {
		t.Fatal("expected no hint for plain English")
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
