package chat

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
	rows := make([]listPickerItem, 0, len(items))
	for i, name := range items {
		label := name
		if i == m.themePickCursor {
			label += "  selected"
		}
		rows = append(rows, listPickerItem{label: label, selected: i == m.themePickCursor})
	}
	userPath := filepath.Join(strings.TrimSpace(m.userConfigDir), "settings.json")
	m.themePickFullText = renderListPicker(
		th,
		"Appearance",
		rows,
		"↑↓ move · Enter apply · Esc cancel",
		fmt.Sprintf("Applies now and saves ui_appearance to %s", userPath),
	)
}

func (m *Model) openThemePicker() {
	m.exitTranscriptBrowse()
	m.exitConfirmDeadline = time.Time{}
	m.docOverlayOpen = false
	m.docOverlayTitle = ""
	m.docOverlaySourceMD = ""
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
	m.syncViewportKeyMapForOverlay()
	m.layout()
	m.viewport.GotoTop()
}

func (m *Model) closeThemePicker() {
	m.themePickOpen = false
	m.themePickFullText = ""
	m.syncViewportKeyMapForCompose()
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
	return fmt.Sprintf("appearance set to %q and saved to %s", preset, userPath)
}
