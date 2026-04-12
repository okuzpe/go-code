package app

import (
	_ "embed"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
)

//go:embed onboarding_security_full.md
var onboardingSecurityFullMD string

const (
	glamourSecurityMinWrap       = 40
	glamourSecurityFallbackWrap  = 68
	onboardingSecurityDocRuleMax = 78
)

// glamourRenderSecurityFull renders the full SECURITY-style markdown for terminal display.
func glamourRenderSecurityFull(uiAppearance string, wrap int) string {
	if wrap < glamourSecurityMinWrap {
		wrap = glamourSecurityFallbackWrap
	}
	opts := config.GlamourTermRendererOptions(uiAppearance, wrap)
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return strings.TrimSpace(onboardingSecurityFullMD)
	}
	out, err := r.Render(onboardingSecurityFullMD)
	if err != nil {
		return strings.TrimSpace(onboardingSecurityFullMD)
	}
	return strings.TrimRight(out, "\n")
}

type secPreflightPhase int

const (
	secPhaseSummary secPreflightPhase = iota
	secPhaseDoc
)

// secPreflightModel is a short Bubble Tea flow: summary → optional full doc (viewport) → quit.
type secPreflightModel struct {
	phase        secPreflightPhase
	version      string
	uiAppearance string
	width        int
	height       int
	vp           viewport.Model
	docRendered  string
	docWrap      int
	aborted      bool
}

func newSecPreflight(version, uiAppearance string) *secPreflightModel {
	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.MouseWheelEnabled = true
	vp.SoftWrap = true
	return &secPreflightModel{
		version:      version,
		uiAppearance: uiAppearance,
		vp:           vp,
	}
}

func (m *secPreflightModel) Init() tea.Cmd {
	return nil
}

func (m *secPreflightModel) advanceFromSummary() tea.Cmd {
	m.phase = secPhaseDoc
	m.refreshDoc()
	return nil
}

func (m *secPreflightModel) refreshDoc() {
	if m.width <= 0 {
		m.width = 80
	}
	if m.height <= 0 {
		m.height = 24
	}
	innerW := max(m.width-4, 48)
	if innerW != m.docWrap || m.docRendered == "" {
		m.docWrap = innerW
		m.docRendered = glamourRenderSecurityFull(m.uiAppearance, innerW)
	}
	vpW := max(m.width-2, 40)
	vpH := max(m.height-9, 8)
	m.vp.SetWidth(vpW)
	m.vp.SetHeight(vpH)
	m.vp.SetContent(m.docRendered)
	m.vp.GotoTop()
}

func (m *secPreflightModel) finishOK() tea.Cmd {
	return tea.Quit
}

func (m *secPreflightModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.phase == secPhaseDoc {
			m.refreshDoc()
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.aborted = true
			return m, tea.Quit
		}
		switch m.phase {
		case secPhaseSummary:
			switch msg.String() {
			case "enter", "esc":
				return m, m.finishOK()
			case "s", "S":
				return m, m.advanceFromSummary()
			}
		case secPhaseDoc:
			switch msg.String() {
			case "enter", "esc":
				return m, m.finishOK()
			}
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
	}
	if m.phase == secPhaseDoc {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *secPreflightModel) View() tea.View {
	body := m.viewBody()
	v := tea.NewView(body)
	v.AltScreen = true
	if m.phase == secPhaseDoc {
		v.MouseMode = tea.MouseModeCellMotion
	} else {
		v.MouseMode = tea.MouseModeNone
	}
	return v
}

func (m *secPreflightModel) viewBody() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	switch m.phase {
	case secPhaseSummary:
		return m.viewSummary(w)
	default:
		return m.viewDoc(w)
	}
}

func (m *secPreflightModel) viewSummary(w int) string {
	var block strings.Builder
	s := renderOnboardingSecurityMarkdown(m.version, m.uiAppearance, w)
	if s == "" {
		block.WriteString(onboardingWelcomePlainBlock(m.version, w))
	} else {
		block.WriteString("\n")
		block.WriteString(s)
		block.WriteString("\n")
	}
	return block.String()
}

func (m *secPreflightModel) viewDoc(w int) string {
	return renderOnboardingSecurityDocFrame(m.vp.View(), w, m.uiAppearance)
}

// renderOnboardingSecurityDocFrame draws the path header + rule + viewport body + footer.
func renderOnboardingSecurityDocFrame(viewportText string, w int, uiAppearance string) string {
	p := terminalstyle.PaletteForAppearance(uiAppearance)
	pathStyle := lipgloss.NewStyle().Bold(true).Foreground(p.TrustAccent2)
	rule := lipgloss.NewStyle().Foreground(p.Muted).Render(strings.Repeat("╌", min(max(w-2, 40), onboardingSecurityDocRuleMax)))
	head := pathStyle.Render("docs/goclaw/security.md") + "\n" + rule
	foot := lipgloss.NewStyle().
		Foreground(p.Muted).
		Italic(true).
		Render("↑ ↓ PgUp PgDn · scroll · wheel · Press Enter or Esc to continue")
	return "\n" + head + "\n\n" + viewportText + "\n" + foot + "\n"
}

// runOnboardingSecurityGate runs the preflight UI (readline onboarding path). Ctrl+C aborts onboarding.
func runOnboardingSecurityGate(version, uiAppearance string) error {
	m := newSecPreflight(version, uiAppearance)
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	fm, ok := final.(*secPreflightModel)
	if ok && fm.aborted {
		return ErrOnboardingAborted
	}
	return nil
}
