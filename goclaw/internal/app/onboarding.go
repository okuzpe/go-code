package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
)

// ErrOnboardingAborted is returned when the user declines workspace trust or exits onboarding early.
var ErrOnboardingAborted = errors.New("onboarding aborted")

// ShouldRunOnboarding reports whether the interactive first-run wizard should run before PrepareChatRuntime.
func ShouldRunOnboarding(tty, jsonMode, mock bool, userConfigDir string) bool {
	if jsonMode || mock || !tty {
		return false
	}
	if strings.TrimSpace(os.Getenv("GOCLAW_NO_ONBOARDING")) == "1" {
		return false
	}
	if strings.TrimSpace(os.Getenv("GOCLAW_ONBOARDING")) == "1" {
		return true
	}
	userSettings := filepath.Join(userConfigDir, "settings.json")
	_, err := os.Stat(userSettings)
	return os.IsNotExist(err)
}

// RunOnboarding runs the first-run Bubble Tea wizard; it writes user and project settings on success.
func RunOnboarding(version string, workdir string, base config.Config) error {
	return runOnboardingTUI(version, workdir, base)
}

func securityDocURL() string {
	// Relative to monorepo root when browsing the repo; printed as plain text in the terminal.
	return "docs/goclaw/security.md (in the repository)"
}

// onboardingCompletionProfileHint is shown after successful first-run setup.
func onboardingCompletionProfileHint(agentProfile string) string {
	switch strings.ToLower(strings.TrimSpace(agentProfile)) {
	case "build", "general-purpose":
		return "Tip: you picked build mode for direct coding. Use /mode plan when you want review first, or /profile coordinator for advanced hub delegation."
	case "plan":
		return "Tip: you picked plan mode for read-only analysis first. When the plan looks right, use /mode build to implement it in this session."
	default:
		return "Tip: you picked an advanced profile. Use /mode build for direct coding, /mode plan for planning, or /profile coordinator for hub delegation."
	}
}
