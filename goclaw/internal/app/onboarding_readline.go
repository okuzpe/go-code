package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
)

func runOnboardingReadline(version, workdir string, base config.Config) error {
	if err := runOnboardingSecurityGate(version, base.UIAppearance); err != nil {
		return err
	}
	fmt.Println()
	flushOnboardingStdout()

	absWd, err := filepath.Abs(workdir)
	if err != nil {
		absWd = workdir
	}
	printOnboardingTrustStepReadline(base.UIAppearance, absWd)
	choice, err := readLine()
	if err != nil {
		return err
	}
	switch strings.TrimSpace(choice) {
	case "1":
	case "2":
		fmt.Println()
		fmt.Println(" Exiting. cd to a trusted project directory, then run goclaw again.")
		fmt.Println(" (Advanced: GOCLAW_NO_ONBOARDING=1 skips this wizard — see docs/goclaw/security.md.)")
		return ErrOnboardingAborted
	default:
		return fmt.Errorf("onboarding: invalid choice %q", choice)
	}

	projSettings := config.ProjectSettingsPath(workdir, base.ProjectConfigDir)
	if err := config.MergeWriteSettings(projSettings, map[string]any{"trusted_workspace": true}); err != nil {
		return fmt.Errorf("write project settings: %w", err)
	}

	fmt.Println()
	fmt.Println(" Choose the appearance preset for the fullscreen TUI (readline is unchanged).")
	fmt.Println(" To change later, run /theme in the REPL.")
	for i, name := range config.UIAppearanceChoices {
		fmt.Printf("  %d. %s\n", i+1, name)
	}
	maxAppearance := len(config.UIAppearanceChoices) + 1
	fmt.Printf("  %d. auto (terminal-adaptive)\n", maxAppearance)
	fmt.Printf("\n Choose (1-%d): ", maxAppearance)
	appearance, err := readLine()
	if err != nil {
		return err
	}
	uiApp := parseAppearanceChoice(strings.TrimSpace(appearance))

	fmt.Println()
	fmt.Println(" goclaw uses local Ollama by default. Configure host and model (Enter keeps the default).")

	patch := map[string]any{
		"ui_appearance": uiApp,
		"provider":      "ollama",
	}
	fmt.Printf("\n Ollama host [%s]: ", base.OllamaHost)
	host, err := readLine()
	if err != nil {
		return err
	}
	if strings.TrimSpace(host) != "" {
		patch["ollama_host"] = strings.TrimSpace(host)
	} else {
		patch["ollama_host"] = base.OllamaHost
	}
	fmt.Printf(" Ollama model [%s]: ", base.OllamaModel)
	model, err := readLine()
	if err != nil {
		return err
	}
	if strings.TrimSpace(model) != "" {
		patch["ollama_model"] = strings.TrimSpace(model)
	} else {
		patch["ollama_model"] = base.OllamaModel
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

func readLine() (string, error) {
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
