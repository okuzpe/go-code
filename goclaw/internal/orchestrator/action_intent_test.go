package orchestrator

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/llm"
)

func TestUserMessageWantsWorkspaceWrites(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{"empty", "", false},
		{"refactor es", "Refactoriza el paquete internal/foo", true},
		{"mejorar", "quiero mejorar el código", true},
		{"fix english", "Please fix the nil pointer in bar.go", true},
		{"explain only", "Explain how the orchestrator works", false},
		{"path only", "c:/proj/goclaw", false},
		{"explain plus fix", "Explain how X works and then fix the bug", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userMessageWantsWorkspaceWrites(tt.msg); got != tt.want {
				t.Errorf("userMessageWantsWorkspaceWrites(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestToolUsesIncludeWorkspaceWrite(t *testing.T) {
	if toolUsesIncludeWorkspaceWrite(nil) {
		t.Fatal("expected false for nil")
	}
	if toolUsesIncludeWorkspaceWrite([]llm.ToolUse{{ID: "1", Name: "read_file", Input: "{}"}}) {
		t.Fatal("expected false for read_file only")
	}
	if !toolUsesIncludeWorkspaceWrite([]llm.ToolUse{{ID: "1", Name: "edit_file", Input: "{}"}}) {
		t.Fatal("expected true for edit_file")
	}
}
