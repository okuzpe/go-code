package app

import (
	"errors"
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

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
	// #nosec G304 -- intentional use of the process controlling terminal.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil || !term.IsTerminal(tty.Fd()) {
		if tty != nil {
			_ = tty.Close()
		}
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
