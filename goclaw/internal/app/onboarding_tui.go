package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/okuzpe/goclaw/internal/config"
)

type obStep int

const (
	obSecurity obStep = iota
	obTrust
	obTheme
	obOllamaHost
	obOllamaModel
	obDone
)

type obModel struct {
	step   obStep
	cursor int
	width  int
	height int

	version string
	workdir string
	base    config.Config

	themeChoices []string

	ti textinput.Model

	appearance  string
	ollamaHost  string
	ollamaModel string

	// Security step: s = full docs/goclaw/security.md in viewport (same as readline preflight gate).
	secDoc bool
	secVP  viewport.Model

	err error
}

func runOnboardingTUI(version, workdir string, base config.Config) error {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 2048
	ti.SetWidth(72)

	themeLabels := append([]string{}, config.UIAppearanceChoices...)
	themeLabels = append(themeLabels, config.UIAppearanceAuto+" (terminal-adaptive)")

	secVP := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	secVP.MouseWheelEnabled = true
	secVP.SoftWrap = true

	m := obModel{
		step:         obSecurity,
		version:      version,
		workdir:      workdir,
		base:         base,
		themeChoices: themeLabels,
		ti:           ti,
		secVP:        secVP,
		appearance:   config.UIAppearanceAuto,
		ollamaHost:   base.OllamaHost,
		ollamaModel:  base.OllamaModel,
	}
	p := tea.NewProgram(&m)
	_, err := p.Run()
	if err != nil {
		return err
	}
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *obModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *obModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ti.SetWidth(max(20, msg.Width-4))
		if m.step == obSecurity && m.secDoc {
			m.refreshSecurityDocViewport()
		}
		if m.step == obOllamaHost || m.step == obOllamaModel {
			var cmd tea.Cmd
			m.ti, cmd = m.ti.Update(msg)
			return m, cmd
		}
		return m, nil
	case tea.KeyMsg:
		if teaKeyIsCtrlC(msg) {
			m.err = ErrOnboardingAborted
			return m, tea.Quit
		}

		if m.step == obSecurity {
			if m.secDoc {
				if teaKeyIsEnterOrEsc(msg) {
					m.secDoc = false
					m.step = obTrust
					m.cursor = 0
					return m, nil
				}
				var cmd tea.Cmd
				m.secVP, cmd = m.secVP.Update(msg)
				return m, cmd
			}
			switch {
			case teaKeyIsEnterOrEsc(msg):
				m.step = obTrust
				m.cursor = 0
				return m, nil
			case teaKeyIsOnboardingSecurityDocKey(msg):
				m.secDoc = true
				m.refreshSecurityDocViewport()
				return m, nil
			}
		}

		switch {
		case teaKeyIsEsc(msg):
			if m.step == obTrust && m.cursor == 1 {
				m.err = ErrOnboardingAborted
				return m, tea.Quit
			}
			if m.step == obDone {
				return m, tea.Quit
			}
			if m.step == obTrust {
				m.cursor = 1
				m.err = ErrOnboardingAborted
				return m, tea.Quit
			}
			m.err = ErrOnboardingAborted
			return m, tea.Quit
		}

		switch m.step {
		case obTrust:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < 1 {
					m.cursor++
				}
			default:
				if !teaKeyIsEnter(msg) {
					break
				}
				if m.cursor == 1 {
					m.err = ErrOnboardingAborted
					return m, tea.Quit
				}
				proj := config.ProjectSettingsPath(m.workdir, m.base.ProjectConfigDir)
				if err := config.MergeWriteSettings(proj, map[string]any{"trusted_workspace": true}); err != nil {
					m.err = fmt.Errorf("write project settings: %w", err)
					return m, tea.Quit
				}
				m.step = obTheme
				m.cursor = 0
				return m, nil
			}
		case obTheme:
			n := len(m.themeChoices)
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < n-1 {
					m.cursor++
				}
			default:
				if !teaKeyIsEnter(msg) {
					break
				}
				m.appearance = themeIndexToAppearance(m.cursor)
				m.step = obOllamaHost
				m.ti.SetValue(m.ollamaHost)
				m.ti.EchoMode = textinput.EchoNormal
				m.ti.Focus()
				return m, textinput.Blink
			}
		case obOllamaHost:
			if teaKeyIsEnter(msg) {
				v := strings.TrimSpace(m.ti.Value())
				if v != "" {
					m.ollamaHost = v
				}
				m.step = obOllamaModel
				m.ti.SetValue(m.ollamaModel)
				m.ti.EchoMode = textinput.EchoNormal
				m.ti.Focus()
				return m, textinput.Blink
			}
			var cmd tea.Cmd
			m.ti, cmd = m.ti.Update(msg)
			return m, cmd
		case obOllamaModel:
			if teaKeyIsEnter(msg) {
				v := strings.TrimSpace(m.ti.Value())
				if v != "" {
					m.ollamaModel = v
				}
				cmd := m.finish()
				return m, cmd
			}
			var cmd tea.Cmd
			m.ti, cmd = m.ti.Update(msg)
			return m, cmd
		case obDone:
			if teaKeyIsEnter(msg) {
				return m, tea.Quit
			}
		}
	}

	if m.step == obOllamaHost || m.step == obOllamaModel {
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *obModel) finish() tea.Cmd {
	patch := map[string]any{
		"ui_appearance": m.appearance,
		"provider":      "ollama",
		"ollama_host":   m.ollamaHost,
		"ollama_model":  m.ollamaModel,
	}
	userPath := config.UserSettingsPath(m.base.UserConfigDir)
	if err := config.MergeWriteSettings(userPath, patch); err != nil {
		m.err = fmt.Errorf("write user settings: %w", err)
		return tea.Quit
	}
	m.step = obDone
	return nil
}

func themeIndexToAppearance(index int) string {
	if index < 0 {
		return config.UIAppearanceAuto
	}
	if index < len(config.UIAppearanceChoices) {
		return config.UIAppearanceChoices[index]
	}
	return config.UIAppearanceAuto
}

func (m *obModel) refreshSecurityDocViewport() {
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}
	innerW := max(w-4, 48)
	doc := glamourRenderSecurityFull(m.base.UIAppearance, innerW)
	vpW := max(w-2, 40)
	vpH := max(h-9, 8)
	m.secVP.SetWidth(vpW)
	m.secVP.SetHeight(vpH)
	m.secVP.SetContent(doc)
	m.secVP.GotoTop()
}

func (m *obModel) View() tea.View {
	v := tea.NewView(m.viewBody())
	v.AltScreen = true
	if m.step == obSecurity && m.secDoc {
		v.MouseMode = tea.MouseModeCellMotion
	} else {
		v.MouseMode = tea.MouseModeNone
	}
	return v
}

func (m *obModel) viewBody() string {
	var b strings.Builder

	switch m.step {
	case obSecurity:
		w := m.width
		if w <= 0 {
			w = 80
		}
		if m.secDoc {
			b.WriteString(renderOnboardingSecurityDocFrame(m.secVP.View(), w, m.base.UIAppearance))
			break
		}
		s := renderOnboardingSecurityMarkdown(m.version, m.base.UIAppearance, m.width)
		if s == "" {
			b.WriteString(onboardingWelcomePlainBlock(m.version, m.width))
		} else {
			b.WriteString("\n")
			b.WriteString(s)
			b.WriteString("\n")
		}
	case obTrust:
		absWd, _ := filepath.Abs(m.workdir)
		if absWd == "" {
			absWd = m.workdir
		}
		b.WriteString(renderOnboardingTrustStepTUI(m.base.UIAppearance, absWd, m.cursor, m.width))
	case obTheme:
		b.WriteString("\n Choose TUI appearance (change later with /theme):\n\n")
		for i, label := range m.themeChoices {
			prefix := "   "
			if i == m.cursor {
				prefix = "> "
			}
			b.WriteString(fmt.Sprintf("%s%d. %s\n", prefix, i+1, label))
		}
		b.WriteString("\n ↑/↓ · Enter")
	case obOllamaHost:
		b.WriteString("\n Ollama host:\n\n")
		b.WriteString(m.ti.View())
		b.WriteString("\n\n Enter to continue")
	case obOllamaModel:
		b.WriteString("\n Ollama model:\n\n")
		b.WriteString(m.ti.View())
		b.WriteString("\n\n Enter to finish setup")
	case obDone:
		b.WriteString("\n Setup complete. Settings saved under ~/.goclaw/\n\n ")
		b.WriteString(onboardingCompletionProfileHint())
		b.WriteString("\n\n Press Enter…")
	}
	if m.err != nil && m.step != obDone {
		b.WriteString(fmt.Sprintf("\n\n error: %v", m.err))
	}
	return b.String()
}
