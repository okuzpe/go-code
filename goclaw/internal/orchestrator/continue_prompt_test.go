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
