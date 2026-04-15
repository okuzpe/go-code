package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
)

func runOnboardingReadline(version, workdir string, base config.Config) error {
	gate, err := runOnboardingSecurityGate(version, workdir, base)
	if err != nil {
		return err
	}
	fmt.Println()
	flushOnboardingStdout()

	patch := map[string]any{
		"ui_appearance": gate.UIAppearance,
		"provider":      "ollama",
		"ollama_host":   gate.OllamaHost,
		"ollama_model":  gate.OllamaModel,
	}

	userPath := config.UserSettingsPath(base.UserConfigDir)
	if err := config.MergeWriteSettings(userPath, patch); err != nil {
		return fmt.Errorf("write user settings: %w", err)
	}

	fmt.Println()
	fmt.Println(" Setup complete. Your settings were saved under ~/.goclaw/")
	fmt.Println()
	fmt.Println(" " + onboardingCompletionProfileHint())
	fmt.Println()
	return nil
}

func formatVersionSuffix(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		return ""
	}
	return " " + v
}

// parseAppearanceChoice maps a numeric answer to a preset; out-of-range values become auto.
// Callers that need strict validation should check the index before calling.
func parseAppearanceChoice(s string) string {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return config.UIAppearanceAuto
	}
	choices := config.UIAppearanceChoices
	if n <= len(choices) {
		return choices[n-1]
	}
	if n == len(choices)+1 {
		return config.UIAppearanceAuto
	}
	return config.UIAppearanceAuto
}
