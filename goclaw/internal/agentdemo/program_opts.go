package agentdemo

import (
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

// ProgramTTYOpts returns Bubble Tea program options when stdin/stdout are not both TTYs
// (mirrors onboarding: route via /dev/tty on Unix). cleanup closes the tty file when non-nil.
func ProgramTTYOpts() (opts []tea.ProgramOption, cleanup func()) {
	cleanup = func() {}
	if runtime.GOOS == "windows" {
		return nil, cleanup
	}
	if term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) {
		return nil, cleanup
	}
	// #nosec G304 -- intentional controlling terminal for IDE-embedded runs.
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil || !term.IsTerminal(f.Fd()) {
		if f != nil {
			_ = f.Close()
		}
		return nil, cleanup
	}
	cleanup = func() { _ = f.Close() }
	return []tea.ProgramOption{tea.WithInput(f), tea.WithOutput(f)}, cleanup
}
