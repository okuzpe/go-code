package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldRunOnboarding(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, ".goclaw")
	require.NoError(t, os.MkdirAll(user, 0o700))
	settings := filepath.Join(user, "settings.json")

	t.Setenv("GOCLAW_NO_ONBOARDING", "")
	t.Setenv("GOCLAW_ONBOARDING", "")

	require.True(t, ShouldRunOnboarding(true, false, false, user))
	require.NoError(t, os.WriteFile(settings, []byte(`{}`), 0o600))
	require.False(t, ShouldRunOnboarding(true, false, false, user))

	require.NoError(t, os.Remove(settings))
	require.True(t, ShouldRunOnboarding(true, false, false, user))

	t.Setenv("GOCLAW_NO_ONBOARDING", "1")
	require.False(t, ShouldRunOnboarding(true, false, false, user))
	t.Setenv("GOCLAW_NO_ONBOARDING", "")

	require.False(t, ShouldRunOnboarding(false, false, false, user))
	require.False(t, ShouldRunOnboarding(true, true, false, user))
	require.False(t, ShouldRunOnboarding(true, false, true, user))

	t.Setenv("GOCLAW_ONBOARDING", "1")
	require.NoError(t, os.WriteFile(settings, []byte(`{}`), 0o600))
	require.True(t, ShouldRunOnboarding(true, false, false, user))
}

func TestOnboardingWelcomeMarkdown_replacesTitle(t *testing.T) {
	md := onboardingWelcomeMarkdown("1.2.3")
	require.Contains(t, md, "Welcome to goclaw 1.2.3")
	require.NotContains(t, md, "__H1__")
	require.Contains(t, md, "Before you start")
	require.Contains(t, md, "docs/goclaw/security.md")
}

func TestOnboardingProfileStepInitialCursor(t *testing.T) {
	require.Equal(t, 1, onboardingProfileStepInitialCursor("coordinator"))
	require.Equal(t, 0, onboardingProfileStepInitialCursor("plan"))
	require.Equal(t, 0, onboardingProfileStepInitialCursor("general-purpose"))
	require.Len(t, onboardingProfileStepOptions, 2)
}

func TestOnboardingCompletionProfileHint(t *testing.T) {
	require.Contains(t, onboardingCompletionProfileHint("general-purpose"), "direct coding")
	require.Contains(t, onboardingCompletionProfileHint("coordinator"), "hub mode")
}
