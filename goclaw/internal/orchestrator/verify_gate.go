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
	verifyAfterWriteNudgeMessage          = `[goclaw] You made workspace changes. Before finishing with prose only, run verification using native tool calls: bash, run_command, script, or run_tests. Prefer a quick git-aware check too (for example: git status --short and git diff -- <changed files>) so you confirm the actual edited files match your intent. Paste command output in the tool result. If verification is impossible, state one sentence why, then stop.`
	verifyAfterWriteNudgeMessageBuildLite = `[goclaw] You made workspace changes. Before finishing with prose only, run verification using run_tests or run_command. If verification fails, inspect with read_file, repair with edit_file/write_file/patch, then rerun the same verification. If verification is impossible, state one sentence why, then stop.`

	maxVerifyAfterWriteNudges = 4
	maxVerifyChangedPaths     = 8
)

func verifyAfterWriteNudge(buildLite bool) string {
	if buildLite {
		return verifyAfterWriteNudgeMessageBuildLite
	}
	return verifyAfterWriteNudgeMessage
}

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

// verifyGateProcessToolResults updates verify state from one tool batch in execution order.
func verifyGateProcessToolResults(cfg config.Config, ut *userTurnState, toolsUsed []llm.ToolUse, results []llm.ToolResultRecord, intent string) {
	if ut == nil || !cfg.AgentVerifyAfterWrite || !userMessageWantsWorkspaceWrites(intent) {
		return
	}
	for idx, r := range results {
		var tu llm.ToolUse
		if idx < len(toolsUsed) {
			tu = toolsUsed[idx]
		}
		if toolResultIsSuccessfulWorkspaceWrite(r) {
			ut.verifyPending = true
			ut.verifySatisfied = false
			for _, path := range changedPathsFromToolUse(tu) {
				if ut.changedPaths == nil {
					ut.changedPaths = make(map[string]bool)
				}
				ut.changedPaths[path] = true
			}
		}
		if !toolpolicy.IsVerifyTool(r.ToolName) {
			continue
		}
		kind, sig, label, ok := verifyRequirementForToolUse(tu, r.ToolName)
		if !ok {
			continue
		}
		if r.IsError {
			ut.requiredVerifyKind = kind
			ut.requiredVerifySig = sig
			ut.requiredVerifyLabel = label
			continue
		}
		if ut.requiredVerifyKind != "" && !verifyRequirementSatisfied(ut, kind, sig) {
			continue
		}
		if !ut.verifyPending {
			continue
		}
		ut.verifySatisfied = true
		ut.verifyPending = false
		ut.requiredVerifyKind = ""
		ut.requiredVerifySig = ""
		ut.requiredVerifyLabel = ""
	}
}

func verifyRequirementForToolUse(tu llm.ToolUse, toolName string) (kind, sig, label string, ok bool) {
	switch strings.TrimSpace(toolName) {
	case "run_tests":
		return "run_tests", "run_tests", "run_tests", true
	case "bash", "run_command":
		var in struct {
			Command string `json:"command"`
		}
		if err := tools.UnmarshalToolInputJSON(tu.Input, &in); err != nil {
			return "", "", "", false
		}
		norm := normalizeVerifyCommand(in.Command)
		if !commandLooksLikeVerification(norm) {
			return "", "", "", false
		}
		return "command", norm, strings.TrimSpace(in.Command), true
	case "script":
		var in struct {
			Script string `json:"script"`
		}
		if err := tools.UnmarshalToolInputJSON(tu.Input, &in); err != nil {
			return "", "", "", false
		}
		norm := normalizeVerifyCommand(in.Script)
		if !commandLooksLikeVerification(norm) {
			return "", "", "", false
		}
		return "script", norm, strings.TrimSpace(in.Script), true
	default:
		return "", "", "", false
	}
}

func verifyRequirementSatisfied(ut *userTurnState, kind, sig string) bool {
	if ut == nil || ut.requiredVerifyKind == "" {
		return true
	}
	return ut.requiredVerifyKind == kind && ut.requiredVerifySig == sig
}

func normalizeVerifyCommand(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(raw)), " "))
}

func toolUseSatisfiesVerifyGate(tu llm.ToolUse, toolName string) bool {
	_, _, _, ok := verifyRequirementForToolUse(tu, toolName)
	return ok
}

func commandLooksLikeVerification(raw string) bool {
	low := normalizeVerifyCommand(raw)
	if low == "" {
		return false
	}
	verifyMarkers := []string{
		"go build", "go test", "go vet",
		"cargo test", "cargo check", "cargo build",
		"npm test", "npm run test", "npm run build", "npm run lint",
		"pnpm test", "pnpm run test", "pnpm build", "pnpm lint",
		"yarn test", "yarn build", "yarn lint",
		"pytest", "tox", "ruff check", "mypy",
		"make test", "make check", "make build", "make lint", "make verify",
		"cmake --build", "ctest",
		"gradle test", "gradlew test", "mvn test",
		"lint", " build", " test", " verify", " check", " compile", " vet",
	}
	for _, marker := range verifyMarkers {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
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

func verifyChangedPathsBlock(ut *userTurnState, workdir string, buildLite bool) string {
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
	if buildLite {
		b.WriteString("Before ending, run verification with `run_tests` or `run_command`. If a verification command already failed this turn, repair the files and rerun that same verification before finishing.")
	} else if strings.TrimSpace(workdir) != "" {
		b.WriteString("Before ending, prefer at least one git-aware check from the workspace, such as `git status --short` and `git diff -- <changed files>`, plus tests/build if relevant.")
	} else {
		b.WriteString("Before ending, prefer at least one git-aware check such as `git status --short` and `git diff -- <changed files>`, plus tests/build if relevant.")
	}
	if strings.TrimSpace(ut.requiredVerifyLabel) != "" {
		b.WriteString("\nRe-run the last failed verification before ending: `")
		b.WriteString(ut.requiredVerifyLabel)
		b.WriteString("`.")
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
