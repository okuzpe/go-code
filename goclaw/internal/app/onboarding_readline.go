package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
	"golang.org/x/term"
)

func runOnboardingReadline(version, workdir string, base config.Config) error {
	if err := runOnboardingSecurityGate(version, base.UIAppearance); err != nil {
		return err
	}
	fmt.Println()

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
	fmt.Printf("  %d. auto (terminal-adaptive)\n", len(config.UIAppearanceChoices)+1)
	fmt.Print("\n Choose (1-7): ")
	appearance, err := readLine()
	if err != nil {
		return err
	}
	uiApp := parseAppearanceChoice(strings.TrimSpace(appearance))

	fmt.Println()
	fmt.Println(" Select how goclaw connects to the language model:")
	fmt.Println()
	fmt.Println("  1. Local Ollama — models on this machine")
	fmt.Println("  2. Anthropic API — API key (Console / usage billing)")
	fmt.Println("  3. Third-party cloud (Bedrock, Foundry, Vertex) — not available yet")
	fmt.Print("\n Choose (1-3): ")
	conn, err := readLine()
	if err != nil {
		return err
	}
	conn = strings.TrimSpace(conn)
	for conn == "3" || (conn != "1" && conn != "2") {
		if conn != "1" && conn != "2" && conn != "3" {
			fmt.Println(" Invalid choice. Enter 1, 2, or 3.")
		} else {
			fmt.Println(" That option is not implemented yet. Choose 1 (Ollama) or 2 (Anthropic).")
		}
		fmt.Print("\n Choose (1-3): ")
		conn, err = readLine()
		if err != nil {
			return err
		}
		conn = strings.TrimSpace(conn)
	}

	patch := map[string]any{
		"ui_appearance": uiApp,
	}
	var apiKey string

	switch conn {
	case "1":
		patch["provider"] = "ollama"
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
	case "2":
		patch["provider"] = "anthropic"
		fmt.Print("\n Anthropic API key (input hidden): ")
		fd := int(os.Stdin.Fd())
		if !term.IsTerminal(fd) {
			return fmt.Errorf("onboarding: stdin is not a terminal; cannot read API key safely")
		}
		keyBytes, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read API key: %w", err)
		}
		apiKey = strings.TrimSpace(string(keyBytes))
		if apiKey == "" {
			return fmt.Errorf("onboarding: API key is required for Anthropic")
		}
	default:
		return fmt.Errorf("onboarding: invalid connection choice %q", conn)
	}

	userPath := config.UserSettingsPath(base.UserConfigDir)
	if err := config.MergeWriteSettings(userPath, patch); err != nil {
		return fmt.Errorf("write user settings: %w", err)
	}
	if apiKey != "" {
		localPath := config.UserSettingsLocalPath(base.UserConfigDir)
		if err := config.MergeWriteSettings(localPath, map[string]any{"api_key": apiKey}); err != nil {
			return fmt.Errorf("write user local settings: %w", err)
		}
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
	switch s {
	case "1":
		return config.UIAppearanceDark
	case "2":
		return config.UIAppearanceLight
	case "3":
		return config.UIAppearanceDarkColorblind
	case "4":
		return config.UIAppearanceLightColorblind
	case "5":
		return config.UIAppearanceDarkANSI
	case "6":
		return config.UIAppearanceLightANSI
	case "7":
		return config.UIAppearanceAuto
	default:
		return config.UIAppearanceAuto
	}
}

func readLine() (string, error) {
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}
