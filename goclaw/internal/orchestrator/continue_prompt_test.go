package orchestrator

import (
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/llm"
)

func TestContinueFollowUpPromptFallsBackWhenNothingPending(t *testing.T) {
	t.Parallel()
	msgs := []llm.Message{
		llm.PlainMessage("user", "fix the bug"),
		llm.PlainMessage("assistant", "done"),
	}
	got := ContinueFollowUpPrompt(msgs)
	if got != genericContinueFollowUpPrompt {
		t.Fatalf("got %q, want generic fallback", got)
	}
}

func TestContinueFollowUpPromptCarriesFailedVerifyState(t *testing.T) {
	t.Parallel()
	msgs := []llm.Message{
		llm.PlainMessage("user", "fix the bug"),
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCallRecord{
				{ID: "w1", Name: "write_file", Input: `{"path":"internal/a.go","content":"package a"}`},
			},
		},
		{
			Role: "user",
			ToolResults: []llm.ToolResultRecord{
				{ToolUseID: "w1", ToolName: "write_file", Content: "ok"},
			},
		},
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCallRecord{
				{ID: "v1", Name: "bash", Input: `{"command":"go build ./..."}`},
			},
		},
		{
			Role: "user",
			ToolResults: []llm.ToolResultRecord{
				{ToolUseID: "v1", ToolName: "bash", Content: "# broken", IsError: true},
			},
		},
	}
	got := ContinueFollowUpPrompt(msgs)
	if !strings.Contains(got, "go build ./...") {
		t.Fatalf("expected failed verify label in prompt: %q", got)
	}
	if !strings.Contains(got, "internal/a.go") {
		t.Fatalf("expected changed path in prompt: %q", got)
	}
}

func TestContinueFollowUpPromptCarriesEditRecoveryState(t *testing.T) {
	t.Parallel()
	msgs := []llm.Message{
		llm.PlainMessage("user", "fix the bug"),
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCallRecord{
				{ID: "e1", Name: "edit_file", Input: `{"path":"README.md","old_string":"bogus","new_string":"x"}`},
			},
		},
		{
			Role: "user",
			ToolResults: []llm.ToolResultRecord{
				{ToolUseID: "e1", ToolName: "edit_file", Content: "old_string not found", IsError: true},
			},
		},
	}
	got := ContinueFollowUpPrompt(msgs)
	if !strings.Contains(got, "edit_file recovery is still pending") {
		t.Fatalf("expected edit recovery instruction in prompt: %q", got)
	}
	if !strings.Contains(got, "read_file the target first") {
		t.Fatalf("expected read_file recovery hint in prompt: %q", got)
	}
}

func TestContinueFollowUpPromptCarriesPathRecoveryState(t *testing.T) {
	t.Parallel()
	msgs := []llm.Message{
		llm.PlainMessage("user", "fix the docs path"),
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCallRecord{
				{ID: "r1", Name: "read_file", Input: `{"path":"cmd/goclaw/docs/README.md"}`},
			},
		},
		{
			Role: "user",
			ToolResults: []llm.ToolResultRecord{
				{ToolUseID: "r1", ToolName: "read_file", Content: "path does not exist", IsError: true},
			},
		},
	}
	got := ContinueFollowUpPrompt(msgs)
	if !strings.Contains(got, "path recovery is still pending") {
		t.Fatalf("expected path recovery instruction in prompt: %q", got)
	}
	if !strings.Contains(got, "cmd/goclaw/docs/README.md") {
		t.Fatalf("expected invalid path in prompt: %q", got)
	}
	if !strings.Contains(got, "glob or grep") {
		t.Fatalf("expected discovery guidance in prompt: %q", got)
	}
}

func TestContinueFollowUpPromptKeepsPathRecoveryAfterEmptyGlob(t *testing.T) {
	t.Parallel()
	msgs := []llm.Message{
		llm.PlainMessage("user", "fix the docs path"),
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCallRecord{
				{ID: "r1", Name: "read_file", Input: `{"path":"cmd/goclaw/docs/README.md"}`},
			},
		},
		{
			Role: "user",
			ToolResults: []llm.ToolResultRecord{
				{ToolUseID: "r1", ToolName: "read_file", Content: "path does not exist", IsError: true},
			},
		},
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCallRecord{
				{ID: "g1", Name: "glob", Input: `{"pattern":"README.md"}`},
			},
		},
		{
			Role: "user",
			ToolResults: []llm.ToolResultRecord{
				{ToolUseID: "g1", ToolName: "glob", Content: "(no matches)", IsError: false},
			},
		},
	}
	got := ContinueFollowUpPrompt(msgs)
	if !strings.Contains(got, "path recovery is still pending") {
		t.Fatalf("expected path recovery to remain pending after empty glob: %q", got)
	}
}

func TestContinueFollowUpPromptClearsPathRecoveryAfterUsefulGlob(t *testing.T) {
	t.Parallel()
	msgs := []llm.Message{
		llm.PlainMessage("user", "fix the docs path"),
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCallRecord{
				{ID: "r1", Name: "read_file", Input: `{"path":"cmd/goclaw/docs/README.md"}`},
			},
		},
		{
			Role: "user",
			ToolResults: []llm.ToolResultRecord{
				{ToolUseID: "r1", ToolName: "read_file", Content: "path does not exist", IsError: true},
			},
		},
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCallRecord{
				{ID: "g1", Name: "glob", Input: `{"pattern":"README.md"}`},
			},
		},
		{
			Role: "user",
			ToolResults: []llm.ToolResultRecord{
				{ToolUseID: "g1", ToolName: "glob", Content: "docs/README.md", IsError: false},
			},
		},
	}
	got := ContinueFollowUpPrompt(msgs)
	if strings.Contains(got, "path recovery is still pending") {
		t.Fatalf("expected useful glob to clear path recovery: %q", got)
	}
}

func TestContinueFollowUpPromptUsesLatestRealUserRequestWindow(t *testing.T) {
	t.Parallel()
	msgs := []llm.Message{
		llm.PlainMessage("user", "first request"),
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCallRecord{
				{ID: "w1", Name: "write_file", Input: `{"path":"internal/old.go","content":"x"}`},
			},
		},
		{
			Role: "user",
			ToolResults: []llm.ToolResultRecord{
				{ToolUseID: "w1", ToolName: "write_file", Content: "ok"},
			},
		},
		llm.PlainMessage("user", "second request"),
	}
	got := ContinueFollowUpPrompt(msgs)
	if got != genericContinueFollowUpPrompt {
		t.Fatalf("latest request has no pending state; got %q", got)
	}
}
