package chat

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
)

// NewThemeForAppearance returns a Theme for the given ui_appearance preset (see config package).
// Unknown or empty values use the same styling as DefaultTheme (auto).
func NewThemeForAppearance(raw string) *Theme {
	p := terminalstyle.PaletteForAppearance(raw)
	return newThemeFromPalette(p)
}

func newThemeFromPalette(p terminalstyle.Palette) *Theme {
	topRule := lipgloss.Border{Top: "─"}
	return &Theme{
		AssistantName:  "",
		UserLabel:      "",
		UserEmoji:      ">",
		AssistantEmoji: "●",
		InputPrompt:    "> ",
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
		InputBorder: lipgloss.NewStyle().
			Border(topRule, true, false, false, false).
			BorderForeground(p.InputBorder).
			Padding(0, 0, 0, 2),

		ToolCardBorder: lipgloss.NewStyle().Foreground(p.SepFG),
		ToolCardHead:   lipgloss.NewStyle().Bold(true).Foreground(p.ToolFG),
		ToolCardBody:   lipgloss.NewStyle().Foreground(p.DimFG),

		StatusBar:      lipgloss.NewStyle().Foreground(p.Muted),
		StatusBarLabel: lipgloss.NewStyle().Foreground(p.AccentAI).Bold(true),

		SlashPickerName: lipgloss.NewStyle().Bold(true).Foreground(p.SlashPickName),
		SlashPickerDesc: lipgloss.NewStyle().Foreground(p.SlashPickDesc),

		AtRefChip: lipgloss.NewStyle().Bold(true).Foreground(p.SlashPickName),
	}
}
