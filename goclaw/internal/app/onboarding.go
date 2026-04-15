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

// RunOnboarding runs the TUI or readline wizard; it writes user and project settings on success.
func RunOnboarding(version string, workdir string, useTUI bool, base config.Config) error {
	if useTUI {
		if err := runOnboardingTUI(version, workdir, base); err != nil {
			return err
		}
	} else {
		if err := runOnboardingReadline(version, workdir, base); err != nil {
			return err
		}
	}
	return nil
}

func securityDocURL() string {
	// Relative to monorepo root when browsing the repo; printed as plain text in the terminal.
	return "docs/goclaw/security.md (in the repository)"
}

// onboardingCompletionProfileHint is shown after successful first-run setup (readline + TUI).
func onboardingCompletionProfileHint() string {
	return "Tip: default profile is general-purpose (full tools on the main session). Prefer /profile builder for shorter, action-first replies. For hub-and-spoke delegation, use /profile coordinator or set agent_profile in settings."
}
