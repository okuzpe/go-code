package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/ui/icons"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type doctorPagerModel struct {
	content      string
	uiAppearance string
	iconSet      icons.Set
	termWidth    int
	vp           viewport.Model
	ready        bool
}

func newDoctorPagerModel(report, uiAppearance string, iconSet icons.Set) *doctorPagerModel {
	return &doctorPagerModel{
		content:      strings.TrimRight(report, "\n"),
		uiAppearance: uiAppearance,
		iconSet:      iconSet,
	}
}

func (m *doctorPagerModel) Init() tea.Cmd { return nil }

func (m *doctorPagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		headerLines := 3
		vh := msg.Height - headerLines
		if vh < 1 {
			vh = 1
		}
		if !m.ready {
			m.vp = viewport.New(viewport.WithWidth(msg.Width), viewport.WithHeight(vh))
			m.vp.SetContent(m.content)
			m.ready = true
		} else {
			m.vp.SetWidth(msg.Width)
			m.vp.SetHeight(vh)
		}
		return m, nil
	case tea.KeyMsg:
		s := msg.String()
		if s == "ctrl+c" || s == "q" || s == "esc" {
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *doctorPagerModel) View() tea.View {
	var body string
	if !m.ready {
		body = "…"
	} else {
		p := terminalstyle.PaletteForAppearance(m.uiAppearance)
		title := lipgloss.NewStyle().Bold(true).Foreground(p.AccentAI).Render(m.iconSet.DoctorBadge()+"goclaw doctor")
		hint := lipgloss.NewStyle().Foreground(p.Muted).Render("q / Esc / Ctrl+C quit · arrows · PgUp / PgDn")
		ruleW := m.termWidth
		if ruleW < 12 {
			ruleW = 40
		}
		if ruleW > 160 {
			ruleW = 160
		}
		rule := lipgloss.NewStyle().Foreground(p.SepFG).Render(strings.Repeat(m.iconSet.ToolCardH(), ruleW))
		var b strings.Builder
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, title, "  ", hint))
		b.WriteString("\n")
		b.WriteString(rule)
		b.WriteString("\n\n")
		b.WriteString(m.vp.View())
		body = b.String()
	}
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

// runDoctorBubbleTea runs a minimal fullscreen pager for the doctor report.
func runDoctorBubbleTea(report, uiAppearance string, iconSet icons.Set) error {
	opts, cleanup := onboardingTeaOptsControllingTTY()
	defer cleanup()
	m := newDoctorPagerModel(report, uiAppearance, iconSet)
	p := tea.NewProgram(m, opts...)
	_, err := p.Run()
	if err != nil && errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}

// RunDoctor shows the health report: Bubble Tea pager on a TTY, plain print otherwise.
func RunDoctor(cmd *cobra.Command, _ []string) error {
	rt, err := PrepareChatRuntime(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	report := DoctorReportFromRuntime(ctx, rt)
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	stdoutTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if stdinTTY && stdoutTTY {
		iconSet := icons.SetFromCanonical(rt.Cfg.TUIIcons)
		if err := runDoctorBubbleTea(report, rt.Cfg.UIAppearance, iconSet); err != nil {
			return err
		}
		return nil
	}
	fmt.Println(report)
	return nil
}
