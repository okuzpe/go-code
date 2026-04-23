package orchestrator

import (
	"sort"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/toolpolicy"
	"github.com/okuzpe/goclaw/internal/tools"
)

const (
	verifyAfterWriteNudgeMessage = `[goclaw] You made workspace changes. Before finishing with prose only, run verification using native tool calls: bash, run_command, script, or run_tests. Prefer a quick git-aware check too (for example: git status --short and git diff -- <changed files>) so you confirm the actual edited files match your intent. Paste command output in the tool result. If verification is impossible, state one sentence why, then stop.`

	maxVerifyAfterWriteNudges = 4
	maxVerifyChangedPaths     = 8
)

func toolResultIsSuccessfulWorkspaceWrite(r llm.ToolResultRecord) bool {
	if r.IsError {
		return false
	}
	switch r.ToolName {
	case "write_file", "write_files", "edit_file", "patch":
		return true
	default:
		return false
	}
}

// verifyGateProcessToolResults updates verify state from one tool batch (writes then verify tools).
func verifyGateProcessToolResults(cfg config.Config, ut *userTurnState, toolsUsed []llm.ToolUse, results []llm.ToolResultRecord, intent string) {
	if ut == nil || !cfg.AgentVerifyAfterWrite || !userMessageWantsWorkspaceWrites(intent) {
		return
	}
	for idx, r := range results {
		if toolResultIsSuccessfulWorkspaceWrite(r) {
			ut.verifyPending = true
			ut.verifySatisfied = false
			if idx < len(toolsUsed) {
				for _, path := range changedPathsFromToolUse(toolsUsed[idx]) {
					if ut.changedPaths == nil {
						ut.changedPaths = make(map[string]bool)
					}
					ut.changedPaths[path] = true
				}
			}
		}
	}
	for _, r := range results {
		if toolpolicy.IsVerifyTool(r.ToolName) && !r.IsError {
			ut.verifySatisfied = true
			ut.verifyPending = false
		}
	}
}

// verifyGateShouldInjectNudge reports whether a synthetic user line should force another LLM iteration.
func verifyGateShouldInjectNudge(cfg config.Config, ut *userTurnState, profileReadOnly bool) bool {
	if ut == nil || !cfg.AgentVerifyAfterWrite || profileReadOnly {
		return false
	}
	if !ut.verifyPending || ut.verifySatisfied {
		return false
	}
	if ut.verifyNudges >= maxVerifyAfterWriteNudges {
		return false
	}
	return true
}

func verifyGateApplyNudge(ut *userTurnState) {
	if ut == nil {
		return
	}
	ut.verifyNudges++
}

func verifyChangedPathsBlock(ut *userTurnState, workdir string) string {
	if ut == nil || !ut.verifyPending || len(ut.changedPaths) == 0 {
		return ""
	}
	paths := make([]string, 0, len(ut.changedPaths))
	for path := range ut.changedPaths {
		if strings.TrimSpace(path) != "" {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return ""
	}
	sort.Strings(paths)
	if len(paths) > maxVerifyChangedPaths {
		paths = paths[:maxVerifyChangedPaths]
	}
	var b strings.Builder
	b.WriteString("\n\n## Verify changed files\n")
	b.WriteString("Changed paths this turn:\n")
	for _, path := range paths {
		b.WriteString("- ")
		b.WriteString(path)
		b.WriteString("\n")
	}
	if strings.TrimSpace(workdir) != "" {
		b.WriteString("Before ending, prefer at least one git-aware check from the workspace, such as `git status --short` and `git diff -- <changed files>`, plus tests/build if relevant.")
	} else {
		b.WriteString("Before ending, prefer at least one git-aware check such as `git status --short` and `git diff -- <changed files>`, plus tests/build if relevant.")
	}
	return strings.TrimRight(b.String(), "\n")
}

func changedPathsFromToolUse(tu llm.ToolUse) []string {
	switch tu.Name {
	case "write_file", "edit_file", "patch":
		var in struct {
			Path string `json:"path"`
		}
		if err := tools.UnmarshalToolInputJSON(tu.Input, &in); err == nil && strings.TrimSpace(in.Path) != "" {
			return []string{strings.TrimSpace(in.Path)}
		}
	case "write_files":
		var in struct {
			Files []struct {
				Path string `json:"path"`
			} `json:"files"`
		}
		if err := tools.UnmarshalToolInputJSON(tu.Input, &in); err == nil {
			out := make([]string, 0, len(in.Files))
			for _, f := range in.Files {
				if p := strings.TrimSpace(f.Path); p != "" {
					out = append(out, p)
				}
			}
			return out
		}
	}
	return nil
}
