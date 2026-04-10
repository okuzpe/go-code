package terminalstyle

import (
	lipv2 "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/okuzpe/goclaw/internal/config"
)

// SpinnerAccentLipV2 returns the Bubbles v2 spinner foreground style for ui_appearance.
func SpinnerAccentLipV2(appearance string) lipv2.Style {
	app := config.NormalizeUIAppearance(appearance)
	switch app {
	case config.UIAppearanceDarkANSI, config.UIAppearanceLightANSI:
		return lipv2.NewStyle().Foreground(lipv2.Color("5"))
	case config.UIAppearanceLight, config.UIAppearanceLightColorblind:
		return lipv2.NewStyle().Foreground(compat.AdaptiveColor{
			Light: lipv2.Color("#7C3AED"),
			Dark:  lipv2.Color("#7C3AED"),
		})
	case config.UIAppearanceDarkColorblind:
		return lipv2.NewStyle().Foreground(compat.AdaptiveColor{
			Light: lipv2.Color("#FBBF24"),
			Dark:  lipv2.Color("#FBBF24"),
		})
	case config.UIAppearanceDark:
		return lipv2.NewStyle().Foreground(compat.AdaptiveColor{
			Light: lipv2.Color("#C4B5FD"),
			Dark:  lipv2.Color("#C4B5FD"),
		})
	default:
		return lipv2.NewStyle().Foreground(compat.AdaptiveColor{
			Light: lipv2.Color("#7C3AED"),
			Dark:  lipv2.Color("#C4B5FD"),
		})
	}
}
