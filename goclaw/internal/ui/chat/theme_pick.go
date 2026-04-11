package chat

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
)

func bareThemeSlashInput(raw string) bool {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) != 1 {
		return false
	}
	return strings.EqualFold(strings.TrimPrefix(fields[0], "/"), "theme")
}

func (m *Model) themePickPresetNames() []string {
	return config.ThemePresetList()
}

func (m *Model) refreshThemePickOverlay() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	items := m.themePickPresetNames()
	if m.themePickCursor < 0 {
		m.themePickCursor = 0
	}
	if m.themePickCursor >= len(items) {
		m.themePickCursor = len(items) - 1
	}
	var b strings.Builder
	b.WriteString(th.ModalTitle.Render("TUI appearance (ui_appearance)"))
	b.WriteString("\n\n")
	for i, name := range items {
		if i == m.themePickCursor {
			b.WriteString(th.SlashPickerName.Render("▸ " + name))
		} else {
			b.WriteString(th.ModalBody.Render("  " + name))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.FooterDim.Render("↑↓ move · Enter apply · Esc cancel"))
	userPath := filepath.Join(strings.TrimSpace(m.userConfigDir), "settings.json")
	b.WriteString("\n")
	b.WriteString(th.FooterDim.Render(fmt.Sprintf("Merges ui_appearance into %s", userPath)))
	b.WriteString("\n")
	b.WriteString(th.FooterDim.Render("Restart the fullscreen TUI to apply colors fully."))
	m.themePickFullText = b.String()
}

func (m *Model) openThemePicker() {
	m.agentPickOpen = false
	m.agentPickFullText = ""
	items := m.themePickPresetNames()
	m.themePickCursor = 0
	if dir := strings.TrimSpace(m.userConfigDir); dir != "" && strings.TrimSpace(m.workdir) != "" {
		cfg := config.Default()
		cfg.UserConfigDir = dir
		if merged, err := config.Load(cfg, m.workdir); err == nil {
			cur := strings.TrimSpace(merged.UIAppearance)
			for i, name := range items {
				if name == cur {
					m.themePickCursor = i
					break
				}
			}
		}
	}
	m.themePickOpen = true
	m.refreshThemePickOverlay()
	m.layout()
	m.viewport.GotoTop()
}

func (m *Model) closeThemePicker() {
	m.themePickOpen = false
	m.themePickFullText = ""
	m.layout()
	m.viewport.GotoBottom()
}

func (m *Model) moveThemePickCursor(delta int) {
	items := m.themePickPresetNames()
	if len(items) == 0 {
		return
	}
	n := len(items)
	m.themePickCursor = (m.themePickCursor + delta%n + n) % n
	m.refreshThemePickOverlay()
	m.layout()
	m.viewport.GotoTop()
}

func (m *Model) applyThemePick() string {
	items := m.themePickPresetNames()
	if m.themePickCursor < 0 || m.themePickCursor >= len(items) {
		m.closeThemePicker()
		return "error: invalid theme selection"
	}
	preset := items[m.themePickCursor]
	dir := strings.TrimSpace(m.userConfigDir)
	if dir == "" {
		m.closeThemePicker()
		return "error: user config directory not configured"
	}
	userPath := filepath.Join(dir, "settings.json")
	if err := config.MergeWriteSettings(userPath, map[string]any{"ui_appearance": preset}); err != nil {
		m.closeThemePicker()
		return fmt.Sprintf("error: write settings: %v", err)
	}
	m.theme = NewThemeForAppearance(preset)
	m.rebuildWelcomeForWidth()
	m.reflowTitleSeparator()
	m.closeThemePicker()
	return fmt.Sprintf("ui_appearance set to %q (merged into %s).\nRestart the TUI to apply the theme fully.", preset, userPath)
}
