package orchestrator

import (
	"strings"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/agents"
)

// ProfileIntent classifies what kind of agent session a user message prefers (profile class),
// separate from per-turn task_models roles (code / explore / fast).
type ProfileIntent string

const (
	// ProfileIntentStay means keep the configured session profile without auto-switching.
	ProfileIntentStay ProfileIntent = "stay"
	// ProfileIntentHubDelegate means prefer coordinator / spawn_agent multi-worker flow.
	ProfileIntentHubDelegate ProfileIntent = "hub_delegate"
	// ProfileIntentDirectCode means single-session coding with file and shell tools.
	ProfileIntentDirectCode ProfileIntent = "direct_code"
	// ProfileIntentReadOnlyScan means read-only exploration (explore-style).
	ProfileIntentReadOnlyScan ProfileIntent = "read_only_scan"
	// ProfileIntentPlanOnly means planning output without execution (plan profile).
	ProfileIntentPlanOnly ProfileIntent = "plan_only"
	// ProfileIntentVerifyOnly means checks / tests without workspace writes (verification profile).
	ProfileIntentVerifyOnly ProfileIntent = "verify_only"
)

// ProfileIntentConfidence is how reliable the rules classifier considers the intent.
type ProfileIntentConfidence string

const (
	ProfileIntentConfidenceHigh   ProfileIntentConfidence = "high"
	ProfileIntentConfidenceMedium ProfileIntentConfidence = "medium"
)

// ProfileIntentResult is the outcome of profile-type auto-detection.
type ProfileIntentResult struct {
	Intent     ProfileIntent
	Confidence ProfileIntentConfidence
}

// codingTaskRoleProfile is used only for message scoring so coordinator sessions still pick up
// coding signals that profileFallbackRole would otherwise dampen for the hub profile.
var codingTaskRoleProfile = agents.GeneralPurpose

// FusedDirectCodeIntent reports whether the user message should be treated as direct coding work:
// workspace write intent agrees with a coding or fix task role from the shared rules classifier.
func FusedDirectCodeIntent(userMessage string) bool {
	res := ClassifyProfileIntentRules(userMessage)
	return res.Intent == ProfileIntentDirectCode
}

// ClassifyProfileIntentRules scores the user message with lightweight heuristics (no LLM calls).
// Order matters: hub delegation wins over coding verbs in the same prompt.
func ClassifyProfileIntentRules(userMessage string) ProfileIntentResult {
	msg := strings.TrimSpace(userMessage)
	if msg == "" {
		return ProfileIntentResult{Intent: ProfileIntentStay, Confidence: ProfileIntentConfidenceHigh}
	}
	lower := strings.ToLower(msg)

	if hubDelegateSignals(lower, msg) {
		return ProfileIntentResult{Intent: ProfileIntentHubDelegate, Confidence: ProfileIntentConfidenceHigh}
	}
	if verifyOnlySignals(lower) {
		return ProfileIntentResult{Intent: ProfileIntentVerifyOnly, Confidence: ProfileIntentConfidenceMedium}
	}
	if repoWideAuditSignals(lower) {
		return ProfileIntentResult{Intent: ProfileIntentPlanOnly, Confidence: ProfileIntentConfidenceHigh}
	}
	if planOnlySignals(lower) {
		return ProfileIntentResult{Intent: ProfileIntentPlanOnly, Confidence: ProfileIntentConfidenceMedium}
	}

	role := classifyTaskRoleRules(msg, codingTaskRoleProfile)
	if readOnlyScanSignals(lower, role) {
		return ProfileIntentResult{Intent: ProfileIntentReadOnlyScan, Confidence: ProfileIntentConfidenceHigh}
	}

	if userMessageWantsWorkspaceWrites(msg) {
		if role == "code" || role == "fix" {
			return ProfileIntentResult{Intent: ProfileIntentDirectCode, Confidence: ProfileIntentConfidenceHigh}
		}
	}
	if role == "code" || role == "fix" {
		return ProfileIntentResult{Intent: ProfileIntentDirectCode, Confidence: ProfileIntentConfidenceMedium}
	}
	return ProfileIntentResult{Intent: ProfileIntentStay, Confidence: ProfileIntentConfidenceMedium}
}

func hubDelegateSignals(lower, msg string) bool {
	if containsAny(lower, []string{
		"spawn_agent", "spawn agent", "worker agents", "worker tasks",
		"delegate to", "delegates to", "you coordinate", "as coordinator",
		"hub mode", "multi-agent", "parallel workers", "in parallel",
		"simultaneously", "independent tasks", "separate workers",
		"three workers", "four workers", "multiple agents",
	}) {
		return true
	}
	if strings.Contains(lower, "parallel") && (strings.Contains(lower, "task") || strings.Contains(lower, "worker")) {
		return true
	}
	// Long multi-line briefings often imply split work (conservative threshold).
	if strings.Count(msg, "\n") >= 8 && utf8.RuneCountInString(msg) >= 400 {
		return true
	}
	return false
}

func verifyOnlySignals(lower string) bool {
	return containsAny(lower, []string{
		"only run tests", "just run tests", "run the test suite", "run full test",
		"go test ./...", "cargo test", "npm test", "pytest",
		"ci must pass", "make sure tests pass", "report pass/fail",
	}) && !containsAny(lower, []string{
		"implement", "fix ", "refactor", "edit ", "patch ", "write ",
	})
}

func planOnlySignals(lower string) bool {
	return containsAny(lower, []string{
		"implementation plan", "write a plan", "plan only", "roadmap only",
		"step-by-step plan", "design doc", "architecture plan",
	}) && !userMessageWantsWorkspaceWrites(lower)
}

func repoWideAuditSignals(lower string) bool {
	return containsAny(lower, []string{
		"audit the whole repo",
		"audit the whole repository",
		"audit the repo",
		"find all gaps",
		"continuous improvement",
		"continue improving",
		"keep improving",
		"align the system with its own design",
		"align docs",
		"align the documentation",
		"documentation as source of truth",
		"compare docs with code",
		"compare documentation with code",
		"repo-wide audit",
		"whole codebase",
		"entire repo",
		"entire repository",
	})
}

func readOnlyScanSignals(lower, role string) bool {
	if userMessageWantsWorkspaceWrites(lower) {
		return false
	}
	if role == "explore" || role == "reasoning" {
		return containsAny(lower, []string{
			"where is", "find references", "find usages", "explain how",
			"what does", "read only", "read-only", "no changes", "without modifying",
		})
	}
	return false
}
