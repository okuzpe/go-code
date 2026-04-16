package chat

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
)

// NewThemeForAppearance returns a Theme for the given ui_appearance preset (see config package).
// Unknown or empty values use the same styling as DefaultTheme (auto).
func NewThemeForAppearance(raw string) *Theme {
	p := terminalstyle.PaletteForAppearance(raw)
	return newThemeFromPalette(p)
}

func footerWorkspaceChipStyle(p terminalstyle.Palette) lipgloss.Style {
	ascii := p.GlamourStyle == "ascii" ||
		p.Appearance == config.UIAppearanceDarkANSI ||
		p.Appearance == config.UIAppearanceLightANSI
	if ascii {
		return lipgloss.NewStyle().Bold(true).Foreground(p.SlashPickName)
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#1E3A8A", Dark: "#F8FAFC"}).
		Background(lipgloss.AdaptiveColor{Light: "#BFDBFE", Dark: "#2563EB"}).
		Padding(0, 1).
		Bold(true)
}

func newThemeFromPalette(p terminalstyle.Palette) *Theme {
	return &Theme{
		AssistantName:  "",
		UserLabel:      "",
		UserEmoji:      ">",
		AssistantEmoji: "●",
		// Empty: compose uses a line-number gutter instead of a per-line ">" prompt.
		InputPrompt: "",
		mdGlamourStyle: p.GlamourStyle,
		appearance:     p.Appearance,

		System:    lipgloss.NewStyle().Foreground(p.Muted).Italic(true),
		UserTag:   lipgloss.NewStyle().Bold(true).Foreground(p.AccentUser),
		Assistant: lipgloss.NewStyle().Bold(true).Foreground(p.AccentAI),
		Dim:       lipgloss.NewStyle().Foreground(p.DimFG),
		Tool:      lipgloss.NewStyle().Foreground(p.ToolFG).Bold(true),
		ToolTag:   lipgloss.NewStyle().Foreground(p.ToolFG),
		FooterDim: lipgloss.NewStyle().Foreground(p.Muted),
		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, true, true, true).
			BorderForeground(p.ModalBorder),
		ModalTitle: lipgloss.NewStyle().Bold(true).Foreground(p.ModalBorder),
		ModalBody:  lipgloss.NewStyle().Foreground(p.ModalBody),

		ErrorStyle:    lipgloss.NewStyle().Foreground(p.ErrorFG).Bold(true),
		Separator:     lipgloss.NewStyle().Foreground(p.SepFG),
		ToolSpinner:   lipgloss.NewStyle().Foreground(p.ToolFG).Italic(true),
		ToolResultOk:  lipgloss.NewStyle().Foreground(p.AccentUser),
		ToolResultErr: lipgloss.NewStyle().Foreground(p.ErrorFG),
		// Rounded border on all four sides gives the input area the feel of a modern
		// terminal input widget (vs. a plain top-rule). Width must be reduced by 4
		// cells in syncInputComposeWidth (1 left-border + 1 left-pad + content +
		// 1 right-pad + 1 right-border).
		InputBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true, true, true, true).
			BorderForeground(p.InputBorder).
			Padding(0, 1),

		ToolCardBorder: lipgloss.NewStyle().Foreground(p.SepFG),
		ToolCardHead:   lipgloss.NewStyle().Bold(true).Foreground(p.ToolFG),
		ToolCardBody:   lipgloss.NewStyle().Foreground(p.DimFG),

		StatusBar:      lipgloss.NewStyle().Foreground(p.Muted),
		StatusBarLabel: lipgloss.NewStyle().Foreground(p.AccentAI).Bold(true),

		FooterWorkspaceChip: footerWorkspaceChipStyle(p),

		SlashPickerName: lipgloss.NewStyle().Bold(true).Foreground(p.SlashPickName),
		SlashPickerDesc: lipgloss.NewStyle().Foreground(p.SlashPickDesc),

		AtRefChip: lipgloss.NewStyle().Bold(true).Foreground(p.SlashPickName),
	}
}
