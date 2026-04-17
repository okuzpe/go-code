package terminalstyle

import (
	"image/color"

	"github.com/charmbracelet/lipgloss"
	lipv2 "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/okuzpe/goclaw/internal/config"
)

// lipV2Foreground maps a v1 lipgloss terminal color to an image/color value for the Bubbles v2 spinner.
func lipV2Foreground(c lipgloss.TerminalColor) color.Color {
	if c == nil {
		return lipv2.Color("#C4B5FD")
	}
	switch v := c.(type) {
	case lipgloss.Color:
		return lipv2.Color(string(v))
	case lipgloss.AdaptiveColor:
		return compat.AdaptiveColor{
			Light: lipv2.Color(v.Light),
			Dark:  lipv2.Color(v.Dark),
		}
	default:
		return lipv2.Color("#C4B5FD")
	}
}

// SpinnerAccentLipV2 returns the Bubbles v2 spinner foreground style for ui_appearance.
// It tracks Palette.AccentAI so spinner motion stays aligned with the chat AI accent after palette edits.
func SpinnerAccentLipV2(appearance string) lipv2.Style {
	p := PaletteForAppearance(config.NormalizeUIAppearance(appearance))
	return lipv2.NewStyle().Foreground(lipV2Foreground(p.AccentAI))
}
