package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/okuzpe/goclaw/internal/config"
)

// onboardingReadlineInput returns the reader for line-based prompts. When stdin is not a
// terminal (e.g. IDE wiring) but the security gate used /dev/tty for Bubble Tea, reads must
// use the same controlling terminal or prompts block on the wrong stream.
func onboardingReadlineInput() (r io.Reader, cleanup func()) {
	cleanup = func() {}
	if term.IsTerminal(os.Stdin.Fd()) {
		return os.Stdin, cleanup
	}
	if runtime.GOOS == "windows" {
		return os.Stdin, cleanup
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err != nil || !term.IsTerminal(tty.Fd()) {
		if tty != nil {
			_ = tty.Close()
		}
		return os.Stdin, cleanup
	}
	cleanup = func() { _ = tty.Close() }
	return tty, cleanup
}

func readLineFrom(in io.Reader) (string, error) {
	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err != nil {
		if errors.Is(err, io.EOF) {
			if line == "" {
				return "", ErrOnboardingAborted
			}
			return line, nil
		}
		return "", err
	}
	return line, nil
}

func runOnboardingReadline(version, workdir string, base config.Config) error {
	if err := runOnboardingSecurityGate(version, base.UIAppearance); err != nil {
		return err
	}
	fmt.Println()
	flushOnboardingStdout()

	in, inCleanup := onboardingReadlineInput()
	defer inCleanup()

	absWd, err := filepath.Abs(workdir)
	if err != nil {
		absWd = workdir
	}
	printOnboardingTrustStepReadline(base.UIAppearance, absWd)
	choice, err := readLineFrom(in)
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

	var uiApp string
	for {
		fmt.Printf("\n Choose (1-%d, blank=auto): ", maxAppearance)
		appearance, err := readLineFrom(in)
		if err != nil {
			return err
		}
		s := strings.TrimSpace(appearance)
		if s == "" {
			uiApp = config.UIAppearanceAuto
			break
		}
		n, aerr := strconv.Atoi(s)
		if aerr != nil || n < 1 || n > maxAppearance {
			fmt.Println(" Invalid choice. Enter a listed number or leave blank for auto.")
			continue
		}
		uiApp = parseAppearanceChoice(s)
		break
	}

	fmt.Println()
	fmt.Println(" goclaw uses local Ollama by default. Configure host and model (Enter keeps the default).")

	patch := map[string]any{
		"ui_appearance": uiApp,
		"provider":      "ollama",
	}
	fmt.Printf("\n Ollama host [%s]: ", base.OllamaHost)
	host, err := readLineFrom(in)
	if err != nil {
		return err
	}
	if strings.TrimSpace(host) != "" {
		patch["ollama_host"] = strings.TrimSpace(host)
	} else {
		patch["ollama_host"] = base.OllamaHost
	}
	fmt.Printf(" Ollama model [%s]: ", base.OllamaModel)
	model, err := readLineFrom(in)
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

