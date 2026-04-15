package app

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
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

// readlineGateResult holds settings from the single-session readline onboarding gate (Tea).
type readlineGateResult struct {
	UIAppearance string
	OllamaHost   string
	OllamaModel  string
}

type secPreflightPhase int

const (
	secPhaseSummary secPreflightPhase = iota
	secPhaseDoc
	secPhaseTrust
	secPhaseAppearance
	secPhaseOllamaHost
	secPhaseOllamaModel
)

// secPreflightModel is one Bubble Tea run: security → doc → trust → appearance → Ollama host/model.
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

	workdir          string
	projectConfigDir string
	absWorkdir       string

	trustAccepted    bool
	trustDeclined    bool
	trustShowInvalid bool

	maxAppearance         int
	appearanceResult      string
	appearanceShowInvalid bool
	writeErr              error

	ti                 textinput.Model
	ollamaHostDefault  string
	ollamaModelDefault string
	ollamaHostResult   string
	ollamaModelResult  string
}

func newSecPreflight(version, workdir string, base config.Config) *secPreflightModel {
	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.MouseWheelEnabled = true
	vp.SoftWrap = true
	absWd, err := filepath.Abs(workdir)
	if err != nil {
		absWd = workdir
	}
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 2048
	ti.SetWidth(72)
	return &secPreflightModel{
		version:            version,
		uiAppearance:       base.UIAppearance,
		vp:                 vp,
		workdir:            workdir,
		projectConfigDir:   base.ProjectConfigDir,
		absWorkdir:         absWd,
		maxAppearance:      len(config.UIAppearanceChoices) + 1,
		ti:                 ti,
		ollamaHostDefault:  base.OllamaHost,
		ollamaModelDefault: base.OllamaModel,
	}
}

func (m *secPreflightModel) Init() tea.Cmd {
	return nil
}

func (m *secPreflightModel) paletteAppearance() string {
	if strings.TrimSpace(m.appearanceResult) != "" {
		return m.appearanceResult
	}
	return m.uiAppearance
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

func (m *secPreflightModel) advanceToTrust() tea.Cmd {
	m.phase = secPhaseTrust
	m.trustShowInvalid = false
	return nil
}

func (m *secPreflightModel) acceptTrust() tea.Cmd {
	proj := config.ProjectSettingsPath(m.workdir, m.projectConfigDir)
	if err := config.MergeWriteSettings(proj, map[string]any{"trusted_workspace": true}); err != nil {
		m.writeErr = err
		return tea.Quit
	}
	m.trustAccepted = true
	m.phase = secPhaseAppearance
	m.appearanceShowInvalid = false
	return nil
}

func (m *secPreflightModel) advanceToOllamaHost() tea.Cmd {
	m.phase = secPhaseOllamaHost
	m.ti.SetValue(m.ollamaHostDefault)
	m.ti.EchoMode = textinput.EchoNormal
	m.ti.Focus()
	if m.width > 0 {
		m.ti.SetWidth(max(20, m.width-4))
	}
	return textinput.Blink
}

func (m *secPreflightModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.phase == secPhaseDoc {
			m.refreshDoc()
		}
		if m.width > 0 {
			m.ti.SetWidth(max(20, m.width-4))
		}
		return m, nil
	case tea.KeyMsg:
		// Key release events are not used for dismiss/shortcuts; forwarding only in doc viewport.
		if _, ok := msg.(tea.KeyReleaseMsg); ok {
			switch m.phase {
			case secPhaseDoc:
				var cmd tea.Cmd
				m.vp, cmd = m.vp.Update(msg)
				return m, cmd
			case secPhaseOllamaHost, secPhaseOllamaModel:
				var cmd tea.Cmd
				m.ti, cmd = m.ti.Update(msg)
				return m, cmd
			}
			return m, nil
		}
		if teaKeyIsCtrlC(msg) {
			m.aborted = true
			return m, tea.Quit
		}
		switch m.phase {
		case secPhaseSummary:
			switch {
			case teaKeyIsEnterOrEsc(msg):
				return m, m.advanceToTrust()
			case teaKeyIsOnboardingSecurityDocKey(msg):
				return m, m.advanceFromSummary()
			}
		case secPhaseDoc:
			if teaKeyIsEnterOrEsc(msg) {
				return m, m.advanceToTrust()
			}
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		case secPhaseTrust:
			if teaKeyIsEnterOrEsc(msg) {
				return m, nil
			}
			ch := strings.TrimSpace(msg.String())
			if ch == "1" {
				return m, m.acceptTrust()
			}
			if ch == "2" {
				m.trustDeclined = true
				return m, tea.Quit
			}
			k := msg.Key()
			if !k.Mod.Contains(tea.ModCtrl) && !k.Mod.Contains(tea.ModAlt) {
				if k.Code == '1' {
					return m, m.acceptTrust()
				}
				if k.Code == '2' {
					m.trustDeclined = true
					return m, tea.Quit
				}
			}
			m.trustShowInvalid = true
			return m, nil
		case secPhaseAppearance:
			if teaKeyIsEsc(msg) {
				return m, nil
			}
			if teaKeyIsEnter(msg) {
				m.appearanceResult = config.UIAppearanceAuto
				return m, m.advanceToOllamaHost()
			}
			digit := -1
			k := msg.Key()
			if !k.Mod.Contains(tea.ModCtrl) && !k.Mod.Contains(tea.ModAlt) && k.Code >= '1' && k.Code <= '9' {
				digit = int(k.Code - '0')
			} else {
				ch := strings.TrimSpace(msg.String())
				if len(ch) == 1 && ch[0] >= '1' && ch[0] <= '9' {
					digit = int(ch[0] - '0')
				}
			}
			if digit >= 1 && digit <= m.maxAppearance {
				m.appearanceResult = parseAppearanceChoice(strconv.Itoa(digit))
				return m, m.advanceToOllamaHost()
			}
			if digit != -1 || strings.TrimSpace(msg.String()) != "" {
				m.appearanceShowInvalid = true
			}
			return m, nil
		case secPhaseOllamaHost:
			if teaKeyIsEnter(msg) {
				v := strings.TrimSpace(m.ti.Value())
				if v != "" {
					m.ollamaHostResult = v
				} else {
					m.ollamaHostResult = m.ollamaHostDefault
				}
				m.ti.SetValue(m.ollamaModelDefault)
				m.phase = secPhaseOllamaModel
				return m, textinput.Blink
			}
			var cmd tea.Cmd
			m.ti, cmd = m.ti.Update(msg)
			return m, cmd
		case secPhaseOllamaModel:
			if teaKeyIsEnter(msg) {
				v := strings.TrimSpace(m.ti.Value())
				if v != "" {
					m.ollamaModelResult = v
				} else {
					m.ollamaModelResult = m.ollamaModelDefault
				}
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.ti, cmd = m.ti.Update(msg)
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
	// Alternate screen breaks some integrated terminals' keyboard delivery; main buffer is enough
	// for this short preflight and keeps scrollback readable.
	v.AltScreen = false
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
	case secPhaseDoc:
		return m.viewDoc(w)
	case secPhaseTrust:
		return m.viewTrust(w)
	case secPhaseAppearance:
		return m.viewAppearance()
	case secPhaseOllamaHost:
		return m.viewOllamaHost()
	case secPhaseOllamaModel:
		return m.viewOllamaModel()
	default:
		return ""
	}
}

func (m *secPreflightModel) viewOllamaHost() string {
	p := terminalstyle.PaletteForAppearance(m.paletteAppearance())
	title := lipgloss.NewStyle().Bold(true).Foreground(p.TrustAccent).Render("Ollama host")
	sub := lipgloss.NewStyle().Foreground(p.Muted).Render("Press Enter to keep the default in the field.")
	body := lipgloss.NewStyle().Foreground(p.ModalBody)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(sub)
	b.WriteString("\n\n")
	b.WriteString(body.Render("Host:"))
	b.WriteString("\n")
	b.WriteString(m.ti.View())
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(p.Muted).Italic(true).Render("Enter · Ctrl+C to exit"))
	return b.String()
}

func (m *secPreflightModel) viewOllamaModel() string {
	p := terminalstyle.PaletteForAppearance(m.paletteAppearance())
	title := lipgloss.NewStyle().Bold(true).Foreground(p.TrustAccent).Render("Ollama model")
	sub := lipgloss.NewStyle().Foreground(p.Muted).Render("Press Enter to keep the default in the field.")
	body := lipgloss.NewStyle().Foreground(p.ModalBody)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(sub)
	b.WriteString("\n\n")
	b.WriteString(body.Render("Model:"))
	b.WriteString("\n")
	b.WriteString(m.ti.View())
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(p.Muted).Italic(true).Render("Enter · Ctrl+C to exit"))
	return b.String()
}

func (m *secPreflightModel) viewAppearance() string {
	p := terminalstyle.PaletteForAppearance(m.paletteAppearance())
	title := lipgloss.NewStyle().Bold(true).Foreground(p.TrustAccent).
		Render("Choose the appearance preset for the fullscreen TUI")
	sub := lipgloss.NewStyle().Foreground(p.Muted).
		Render("To change later, run /theme in the REPL.")
	body := lipgloss.NewStyle().Foreground(p.ModalBody)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(sub)
	b.WriteString("\n\n")
	for i, name := range config.UIAppearanceChoices {
		b.WriteString(body.Render(fmt.Sprintf("  %d. %s", i+1, name)))
		b.WriteString("\n")
	}
	b.WriteString(body.Render(fmt.Sprintf("  %d. auto (terminal-adaptive)", m.maxAppearance)))
	b.WriteString("\n")
	if m.appearanceShowInvalid {
		h := lipgloss.NewStyle().Foreground(p.Muted).
			Render("Invalid choice. Press 1–" + strconv.Itoa(m.maxAppearance) + ", or Enter for auto.")
		b.WriteString("\n")
		b.WriteString(h)
	}
	foot := lipgloss.NewStyle().Foreground(p.Muted).Italic(true).
		Render("Press 1–" + strconv.Itoa(m.maxAppearance) + " or Enter for auto · Ctrl+C to exit")
	b.WriteString("\n\n")
	b.WriteString(foot)
	return b.String()
}

func (m *secPreflightModel) viewTrust(w int) string {
	var b strings.Builder
	b.WriteString(renderOnboardingTrustStepReadlineTTY(m.uiAppearance, m.absWorkdir, w))
	p := terminalstyle.PaletteForAppearance(m.uiAppearance)
	if m.trustShowInvalid {
		hint := lipgloss.NewStyle().Foreground(p.Muted).
			Render("Invalid choice. Press 1 to trust this folder or 2 to exit.")
		b.WriteString("\n\n")
		b.WriteString(hint)
	}
	foot := lipgloss.NewStyle().Foreground(p.Muted).Italic(true).
		Render("Press 1 or 2 · Ctrl+C to exit")
	b.WriteString("\n\n")
	b.WriteString(foot)
	return b.String()
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

// runOnboardingSecurityGate runs readline onboarding in one Bubble Tea session through Ollama defaults.
// On success, trusted_workspace is already written; returns UI appearance and Ollama host/model.
func runOnboardingSecurityGate(version, workdir string, base config.Config) (readlineGateResult, error) {
	out := readlineGateResult{
		OllamaHost:   base.OllamaHost,
		OllamaModel:  base.OllamaModel,
		UIAppearance: config.UIAppearanceAuto,
	}
	m := newSecPreflight(version, workdir, base)
	opts, cleanup := onboardingTeaOptsControllingTTY()
	defer cleanup()
	final, err := tea.NewProgram(m, opts...).Run()
	if err != nil {
		return out, mapOnboardingTeaRunError(err)
	}
	fm, ok := final.(*secPreflightModel)
	if !ok {
		return out, nil
	}
	if fm.writeErr != nil {
		return out, fmt.Errorf("write project settings: %w", fm.writeErr)
	}
	if fm.aborted {
		return out, ErrOnboardingAborted
	}
	if fm.trustDeclined {
		fmt.Println()
		fmt.Println(" Exiting. cd to a trusted project directory, then run goclaw again.")
		fmt.Println(" (Advanced: GOCLAW_NO_ONBOARDING=1 skips this wizard — see docs/goclaw/security.md.)")
		return out, ErrOnboardingAborted
	}
	if !fm.trustAccepted {
		return out, fmt.Errorf("onboarding: trust step not completed")
	}
	if strings.TrimSpace(fm.appearanceResult) == "" {
		return out, fmt.Errorf("onboarding: appearance not chosen")
	}
	if strings.TrimSpace(fm.ollamaHostResult) == "" || strings.TrimSpace(fm.ollamaModelResult) == "" {
		return out, fmt.Errorf("onboarding: Ollama host or model not set")
	}
	out.UIAppearance = fm.appearanceResult
	out.OllamaHost = fm.ollamaHostResult
	out.OllamaModel = fm.ollamaModelResult
	return out, nil
}
