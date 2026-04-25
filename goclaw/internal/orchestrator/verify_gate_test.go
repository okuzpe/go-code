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

func TestVerifyGateProcessToolResultsGitStatusDoesNotSatisfyVerify(t *testing.T) {
	cfg := config.Default()
	ut := &userTurnState{verifyPending: true, changedPaths: map[string]bool{"internal/a.go": true}}
	verifyGateProcessToolResults(
		cfg,
		ut,
		[]llm.ToolUse{{Name: "bash", Input: `{"command":"git status --short"}`}},
		[]llm.ToolResultRecord{{ToolName: "bash", IsError: false}},
		"please implement the fix",
	)
	if !ut.verifyPending || ut.verifySatisfied {
		t.Fatalf("git status should not satisfy verify gate: %+v", ut)
	}
}

func TestVerifyGateProcessToolResultsGoBuildSatisfiesVerify(t *testing.T) {
	cfg := config.Default()
	ut := &userTurnState{verifyPending: true, changedPaths: map[string]bool{"internal/a.go": true}}
	verifyGateProcessToolResults(
		cfg,
		ut,
		[]llm.ToolUse{{Name: "bash", Input: `{"command":"go build ./..."}`}},
		[]llm.ToolResultRecord{{ToolName: "bash", IsError: false}},
		"please implement the fix",
	)
	if ut.verifyPending || !ut.verifySatisfied {
		t.Fatalf("go build should satisfy verify gate: %+v", ut)
	}
}

func TestVerifyGateFailedBuildRequiresSameBuildToClear(t *testing.T) {
	cfg := config.Default()
	ut := &userTurnState{verifyPending: true, changedPaths: map[string]bool{"internal/a.go": true}}

	verifyGateProcessToolResults(
		cfg,
		ut,
		[]llm.ToolUse{{Name: "bash", Input: `{"command":"go build ./..."}`}},
		[]llm.ToolResultRecord{{ToolName: "bash", IsError: true}},
		"please implement the fix",
	)
	if got := ut.requiredVerifyLabel; got != "go build ./..." {
		t.Fatalf("required verify label = %q, want go build ./...", got)
	}

	verifyGateProcessToolResults(
		cfg,
		ut,
		[]llm.ToolUse{{Name: "bash", Input: `{"command":"go test ./..."}`}},
		[]llm.ToolResultRecord{{ToolName: "bash", IsError: false}},
		"please implement the fix",
	)
	if !ut.verifyPending || ut.verifySatisfied {
		t.Fatalf("go test should not clear pending failed go build: %+v", ut)
	}

	verifyGateProcessToolResults(
		cfg,
		ut,
		[]llm.ToolUse{{Name: "run_command", Input: `{"command":"go build ./..."}`}},
		[]llm.ToolResultRecord{{ToolName: "run_command", IsError: false}},
		"please implement the fix",
	)
	if ut.verifyPending || !ut.verifySatisfied {
		t.Fatalf("matching go build rerun should clear gate: %+v", ut)
	}
}

func TestVerifyGateDoesNotLetEarlierVerifyClearLaterWriteInSameBatch(t *testing.T) {
	cfg := config.Default()
	ut := &userTurnState{changedPaths: make(map[string]bool)}

	verifyGateProcessToolResults(
		cfg,
		ut,
		[]llm.ToolUse{
			{Name: "bash", Input: `{"command":"go build ./..."}`},
			{Name: "edit_file", Input: `{"path":"internal/a.go","old_string":"x","new_string":"y"}`},
		},
		[]llm.ToolResultRecord{
			{ToolName: "bash", IsError: false},
			{ToolName: "edit_file", IsError: false},
		},
		"please implement the fix",
	)

	if !ut.verifyPending || ut.verifySatisfied {
		t.Fatalf("verify before a later write in the same batch must not clear the gate: %+v", ut)
	}
	if !ut.changedPaths["internal/a.go"] {
		t.Fatalf("expected changed path to be tracked: %#v", ut.changedPaths)
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
	block := verifyChangedPathsBlock(ut, "C:/repo", false)
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

func TestVerifyChangedPathsBlockIncludesRequiredVerifyRerun(t *testing.T) {
	ut := &userTurnState{
		verifyPending:       true,
		changedPaths:        map[string]bool{"internal/a.go": true},
		requiredVerifyLabel: "go build ./...",
		requiredVerifyKind:  "command",
		requiredVerifySig:   "go build ./...",
	}
	block := verifyChangedPathsBlock(ut, "C:/repo", false)
	if !strings.Contains(block, "Re-run the last failed verification before ending: `go build ./...`.") {
		t.Fatalf("expected required verify rerun in block: %q", block)
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
