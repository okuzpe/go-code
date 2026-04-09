package permissions

import (
	"strings"
	"testing"
)

func TestApplyConfigModesValid(t *testing.T) {
	p := NewPolicy()
	err := p.ApplyConfigModes(map[string]string{
		"bash":      "ask",
		"read_file": "allow",
		"web_fetch": "deny",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Evaluate("bash") != DecisionAsk {
		t.Fatalf("bash: got %v", p.Evaluate("bash"))
	}
	if p.Evaluate("read_file") != DecisionAllow {
		t.Fatalf("read_file: got %v", p.Evaluate("read_file"))
	}
	if p.Evaluate("web_fetch") != DecisionDeny {
		t.Fatalf("web_fetch: got %v", p.Evaluate("web_fetch"))
	}
}

func TestApplyConfigModesInvalidMode(t *testing.T) {
	p := NewPolicy()
	err := p.ApplyConfigModes(map[string]string{"bash": "nope"})
	if err == nil {
		t.Fatal("expected error for invalid mode string")
	}
	if !strings.Contains(err.Error(), "tool_permissions") || !strings.Contains(err.Error(), "bash") {
		t.Fatalf("error should name key and settings origin: %v", err)
	}
}
