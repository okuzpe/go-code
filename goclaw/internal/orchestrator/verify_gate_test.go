package orchestrator

import (
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
)

func TestVerifyGateProcessToolResultsTracksChangedPaths(t *testing.T) {
	cfg := config.Default()
	ut := &userTurnState{changedPaths: make(map[string]bool)}
	toolsUsed := []llm.ToolUse{
		{Name: "write_file", Input: `{"path":"internal/a.go","content":"x"}`},
		{Name: "write_files", Input: `{"files":[{"path":"internal/b.go","content":"y"},{"path":"README.md","content":"z"}]}`},
	}
	results := []llm.ToolResultRecord{
		{ToolName: "write_file", IsError: false},
		{ToolName: "write_files", IsError: false},
	}
	verifyGateProcessToolResults(cfg, ut, toolsUsed, results, "please implement the fix")
	if !ut.verifyPending || ut.verifySatisfied {
		t.Fatalf("verify gate should be pending after successful writes: %+v", ut)
	}
	for _, path := range []string{"internal/a.go", "internal/b.go", "README.md"} {
		if !ut.changedPaths[path] {
			t.Fatalf("expected changed path %q to be tracked: %#v", path, ut.changedPaths)
		}
	}
}

func TestVerifyGateProcessToolResultsSatisfiedByVerifyTool(t *testing.T) {
	cfg := config.Default()
	ut := &userTurnState{verifyPending: true, changedPaths: map[string]bool{"internal/a.go": true}}
	verifyGateProcessToolResults(cfg, ut, []llm.ToolUse{{Name: "run_tests", Input: `{}`}}, []llm.ToolResultRecord{{ToolName: "run_tests", IsError: false}}, "please implement the fix")
	if ut.verifyPending || !ut.verifySatisfied {
		t.Fatalf("verify gate should be satisfied after successful verify tool: %+v", ut)
	}
}

func TestVerifyChangedPathsBlock(t *testing.T) {
	ut := &userTurnState{
		verifyPending: true,
		changedPaths: map[string]bool{
			"internal/b.go": true,
			"internal/a.go": true,
		},
	}
	block := verifyChangedPathsBlock(ut, "C:/repo")
	if !strings.Contains(block, "Verify changed files") {
		t.Fatalf("expected verify header: %q", block)
	}
	if !strings.Contains(block, "internal/a.go") || !strings.Contains(block, "internal/b.go") {
		t.Fatalf("expected changed paths in block: %q", block)
	}
	if !strings.Contains(block, "git status --short") || !strings.Contains(block, "git diff -- <changed files>") {
		t.Fatalf("expected git-aware verify guidance: %q", block)
	}
}

func TestChangedPathsFromToolUse(t *testing.T) {
	tests := []struct {
		name string
		tool llm.ToolUse
		want []string
	}{
		{
			name: "write_file",
			tool: llm.ToolUse{Name: "write_file", Input: `{"path":"a.go","content":"x"}`},
			want: []string{"a.go"},
		},
		{
			name: "write_files",
			tool: llm.ToolUse{Name: "write_files", Input: `{"files":[{"path":"a.go","content":"x"},{"path":"b.go","content":"y"}]}`},
			want: []string{"a.go", "b.go"},
		},
		{
			name: "unknown",
			tool: llm.ToolUse{Name: "glob", Input: `{"pattern":"*.go"}`},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := changedPathsFromToolUse(tt.tool)
			if len(got) != len(tt.want) {
				t.Fatalf("len(got)=%d want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d]=%q want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
