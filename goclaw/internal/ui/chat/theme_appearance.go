package chat

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/config"
	lipv2 "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// NewThemeForAppearance returns a Theme for the given ui_appearance preset (see config package).
// Unknown or empty values use the same styling as DefaultTheme (auto).
func NewThemeForAppearance(raw string) *Theme {
	app := config.NormalizeUIAppearance(raw)
	switch app {
	case config.UIAppearanceAuto:
		return DefaultTheme()
	case config.UIAppearanceDark:
		return fixedDarkTheme(app, "dark")
	case config.UIAppearanceLight:
		return fixedLightTheme(app, "light")
	case config.UIAppearanceDarkColorblind:
		// Deuteranopia-friendly: blue + amber accents on dark base.
		return buildFixedTheme(app, "dark",
			lipgloss.Color("#60A5FA"), lipgloss.Color("#FBBF24"),
			lipgloss.Color("#9CA3AF"), lipgloss.Color("#6B7280"),
			lipgloss.Color("#FCD34D"), lipgloss.Color("#D1D5DB"),
			lipgloss.Color("#F87171"), lipgloss.Color("#374151"),
			lipgloss.Color("#60A5FA"), lipgloss.Color("#93C5FD"),
		)
	case config.UIAppearanceLightColorblind:
		return buildFixedTheme(app, "light",
			lipgloss.Color("#2563EB"), lipgloss.Color("#D97706"),
			lipgloss.Color("#6B7280"), lipgloss.Color("#9CA3AF"),
			lipgloss.Color("#B45309"), lipgloss.Color("#374151"),
			lipgloss.Color("#DC2626"), lipgloss.Color("#E5E7EB"),
			lipgloss.Color("#2563EB"), lipgloss.Color("#93C5FD"),
		)
	case config.UIAppearanceDarkANSI:
		return ansiDarkTheme(app)
	case config.UIAppearanceLightANSI:
		return ansiLightTheme(app)
	default:
		return DefaultTheme()
	}
}

func fixedDarkTheme(app, glam string) *Theme {
	return buildFixedTheme(app, glam,
		lipgloss.Color("#34D399"), lipgloss.Color("#C4B5FD"),
		lipgloss.Color("#9CA3AF"), lipgloss.Color("#6B7280"),
		lipgloss.Color("#FBBF24"), lipgloss.Color("#D1D5DB"),
		lipgloss.Color("#F87171"), lipgloss.Color("#374151"),
		lipgloss.Color("#C4B5FD"), lipgloss.Color("#4B5563"),
	)
}

func fixedLightTheme(app, glam string) *Theme {
	return buildFixedTheme(app, glam,
		lipgloss.Color("#059669"), lipgloss.Color("#7C3AED"),
		lipgloss.Color("#6B7280"), lipgloss.Color("#9CA3AF"),
		lipgloss.Color("#B45309"), lipgloss.Color("#374151"),
		lipgloss.Color("#DC2626"), lipgloss.Color("#E5E7EB"),
		lipgloss.Color("#7C3AED"), lipgloss.Color("#D1D5DB"),
	)
}

func buildFixedTheme(app, glamStyle string, accentUser, accentAI, muted, dimFG, toolFG, modalBody, errorFG, sepFG, modalBorder, inputBorder lipgloss.Color) *Theme {
	return &Theme{
		AssistantName:  "goclaw",
		UserLabel:      "You",
		UserEmoji:      "❯",
		AssistantEmoji: "✦",
		InputPrompt:    "› ",
		mdGlamourStyle: glamStyle,
		appearance:     app,

		System:    lipgloss.NewStyle().Foreground(muted).Italic(true),
		UserTag:   lipgloss.NewStyle().Bold(true).Foreground(accentUser),
		Assistant: lipgloss.NewStyle().Bold(true).Foreground(accentAI),
		Dim:       lipgloss.NewStyle().Foreground(dimFG),
		Tool:      lipgloss.NewStyle().Foreground(toolFG).Bold(true),
		ToolTag:   lipgloss.NewStyle().Foreground(toolFG),
		FooterDim: lipgloss.NewStyle().Foreground(muted),
		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(modalBorder),
		ModalTitle: lipgloss.NewStyle().Bold(true).Foreground(modalBorder),
		ModalBody:  lipgloss.NewStyle().Foreground(modalBody),

		ErrorStyle:    lipgloss.NewStyle().Foreground(errorFG).Bold(true),
		Separator:     lipgloss.NewStyle().Foreground(sepFG),
		ToolSpinner:   lipgloss.NewStyle().Foreground(toolFG).Italic(true),
		ToolResultOk:  lipgloss.NewStyle().Foreground(accentUser),
		ToolResultErr: lipgloss.NewStyle().Foreground(errorFG),
		InputBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(inputBorder),
		SlashPickerName: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}),
		SlashPickerDesc: lipgloss.NewStyle().Foreground(dimFG),
	}
}

func ansiDarkTheme(app string) *Theme {
	t := buildFixedTheme(app, "ascii",
		lipgloss.Color("2"), lipgloss.Color("5"),
		lipgloss.Color("7"), lipgloss.Color("8"),
		lipgloss.Color("3"), lipgloss.Color("7"),
		lipgloss.Color("1"), lipgloss.Color("8"),
		lipgloss.Color("5"), lipgloss.Color("8"),
	)
	t.SlashPickerName = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	t.SlashPickerDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	return t
}

func ansiLightTheme(app string) *Theme {
	t := buildFixedTheme(app, "ascii",
		lipgloss.Color("2"), lipgloss.Color("5"),
		lipgloss.Color("8"), lipgloss.Color("7"),
		lipgloss.Color("3"), lipgloss.Color("0"),
		lipgloss.Color("1"), lipgloss.Color("7"),
		lipgloss.Color("5"), lipgloss.Color("7"),
	)
	t.SlashPickerName = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	t.SlashPickerDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	return t
}

// SpinnerAccentV2 returns the spinner style for bubbles/v2 for this theme preset.
func (t *Theme) SpinnerAccentV2() lipv2.Style {
	if t == nil {
		return SpinnerAccentStyle()
	}
	switch t.appearance {
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
		return SpinnerAccentStyle()
	}
}
