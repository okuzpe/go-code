package app

import (
	"errors"
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

// openOnboardingControllingTTY opens /dev/tty when the process has a controlling terminal.
// writable requests O_RDWR (Bubble Tea) vs O_RDONLY (line prompts). Returns nil on Windows or on failure.
func openOnboardingControllingTTY(writable bool) *os.File {
	if runtime.GOOS == "windows" {
		return nil
	}
	flags := os.O_RDONLY
	if writable {
		flags = os.O_RDWR
	}
	// #nosec G304 -- intentional use of the process controlling terminal.
	f, err := os.OpenFile("/dev/tty", flags, 0)
	if err != nil || !term.IsTerminal(f.Fd()) {
		if f != nil {
			_ = f.Close()
		}
		return nil
	}
	return f
}

// onboardingTeaOptsControllingTTY returns input/output wired to /dev/tty when either stdio
// stream is not a terminal (IDE tasks, pipes). Bubble Tea only reopens stdin by default; when
// stdout is piped but the user still sees a terminal panel, routing both streams fixes missing
// key events.
func onboardingTeaOptsControllingTTY() (opts []tea.ProgramOption, cleanup func()) {
	cleanup = func() {}
	if runtime.GOOS == "windows" {
		return nil, cleanup
	}
	if term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) {
		return nil, cleanup
	}
	tty := openOnboardingControllingTTY(true)
	if tty == nil {
		return nil, cleanup
	}
	cleanup = func() { _ = tty.Close() }
	return []tea.ProgramOption{tea.WithInput(tty), tea.WithOutput(tty)}, cleanup
}

func mapOnboardingTeaRunError(err error) error {
	if err != nil && errors.Is(err, tea.ErrInterrupted) {
		return ErrOnboardingAborted
	}
	return err
}
