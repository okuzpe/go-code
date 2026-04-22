// Package terminalstyle holds shared Lip Gloss palette tokens for TTY output keyed by
// config ui_appearance, so chat TUI, startup banner, and onboarding stay consistent.
package terminalstyle

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/config"
)

// Palette is a complete set of terminal colors for one ui_appearance preset.
type Palette struct {
	Appearance string

	// GlamourStyle is passed to glamour: "auto", "dark", "light", or "ascii".
	GlamourStyle string

	AccentUser    lipgloss.TerminalColor
	AccentAI      lipgloss.TerminalColor
	Muted         lipgloss.TerminalColor
	DimFG         lipgloss.TerminalColor
	ToolFG        lipgloss.TerminalColor
	ModalBody     lipgloss.TerminalColor
	ErrorFG       lipgloss.TerminalColor
	SepFG         lipgloss.TerminalColor
	ModalBorder   lipgloss.TerminalColor
	InputBorder   lipgloss.TerminalColor
	SlashPickName lipgloss.TerminalColor
	SlashPickDesc lipgloss.TerminalColor
	// WelcomeBorder frames the startup welcome panel (wide box + narrow rounded frame).
	WelcomeBorder lipgloss.TerminalColor

	// Banner / onboarding chrome (non-TTY banner and fallbacks).
	BannerLogo    lipgloss.TerminalColor
	BannerKey     lipgloss.TerminalColor
	BannerValue   lipgloss.TerminalColor
	BannerWarning lipgloss.TerminalColor
	TrustAccent   lipgloss.TerminalColor
	TrustAccent2  lipgloss.TerminalColor
	PathEmphasis  lipgloss.TerminalColor
}

// PaletteForAppearance returns the palette for raw ui_appearance (empty = auto).
func PaletteForAppearance(raw string) Palette {
	app := config.NormalizeUIAppearance(raw)
	switch app {
	case config.UIAppearanceAuto:
		return paletteAuto()
	case config.UIAppearanceDark:
		return paletteFixed(app, "dark",
			lipgloss.Color("#34D399"), lipgloss.Color("#C4B5FD"),
			lipgloss.Color("#9CA3AF"), lipgloss.Color("#6B7280"),
			lipgloss.Color("#FBBF24"), lipgloss.Color("#D1D5DB"),
			lipgloss.Color("#F87171"), lipgloss.Color("#2d333b"),
			lipgloss.Color("#C4B5FD"), lipgloss.Color("#3d4450"),
		)
	case config.UIAppearanceLight:
		return paletteFixed(app, "light",
			lipgloss.Color("#059669"), lipgloss.Color("#7C3AED"),
			lipgloss.Color("#6B7280"), lipgloss.Color("#9CA3AF"),
			lipgloss.Color("#B45309"), lipgloss.Color("#374151"),
			lipgloss.Color("#DC2626"), lipgloss.Color("#E5E7EB"),
			lipgloss.Color("#7C3AED"), lipgloss.Color("#D1D5DB"),
		)
	case config.UIAppearanceDarkColorblind:
		return paletteFixed(app, "dark",
			lipgloss.Color("#60A5FA"), lipgloss.Color("#FBBF24"),
			lipgloss.Color("#9CA3AF"), lipgloss.Color("#6B7280"),
			lipgloss.Color("#FCD34D"), lipgloss.Color("#D1D5DB"),
			lipgloss.Color("#F87171"), lipgloss.Color("#2d333b"),
			lipgloss.Color("#60A5FA"), lipgloss.Color("#3d5568"),
		)
	case config.UIAppearanceLightColorblind:
		return paletteFixed(app, "light",
			lipgloss.Color("#2563EB"), lipgloss.Color("#D97706"),
			lipgloss.Color("#6B7280"), lipgloss.Color("#9CA3AF"),
			lipgloss.Color("#B45309"), lipgloss.Color("#374151"),
			lipgloss.Color("#DC2626"), lipgloss.Color("#E5E7EB"),
			lipgloss.Color("#2563EB"), lipgloss.Color("#93C5FD"),
		)
	case config.UIAppearanceDarkANSI:
		return paletteANSI(app, "ascii",
			lipgloss.Color("2"), lipgloss.Color("5"),
			lipgloss.Color("7"), lipgloss.Color("8"),
			lipgloss.Color("3"), lipgloss.Color("7"),
			lipgloss.Color("1"), lipgloss.Color("8"),
			lipgloss.Color("5"), lipgloss.Color("8"),
		)
	case config.UIAppearanceLightANSI:
		return paletteANSI(app, "ascii",
			lipgloss.Color("2"), lipgloss.Color("5"),
			lipgloss.Color("8"), lipgloss.Color("7"),
			lipgloss.Color("3"), lipgloss.Color("0"),
			lipgloss.Color("1"), lipgloss.Color("7"),
			lipgloss.Color("5"), lipgloss.Color("7"),
		)
	case config.UIAppearanceCyberpunk:
		return paletteCyberpunk()
	default:
		return paletteAuto()
	}
}

func paletteAuto() Palette {
	accentUser := lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	accentAI := lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"}
	muted := lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	dimFG := lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
	toolFG := lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	modalBody := lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"}
	errorFG := lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	sepFG := lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#2d333b"}
	slashPick := lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#5b9bd5"}
	slashDesc := lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#7c8798"}
	modalBorder := lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#b8a9e8"}
	inputBorder := lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#3d4450"}
	welcomeBorder := lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#3f4a56"}
	pathEmphasis := lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#E2E8F0"}
	return Palette{
		Appearance:    config.UIAppearanceAuto,
		GlamourStyle:  "auto",
		AccentUser:    accentUser,
		AccentAI:      accentAI,
		Muted:         muted,
		DimFG:         dimFG,
		ToolFG:        toolFG,
		ModalBody:     modalBody,
		ErrorFG:       errorFG,
		SepFG:         sepFG,
		ModalBorder:   modalBorder,
		InputBorder:   inputBorder,
		SlashPickName: slashPick,
		SlashPickDesc: slashDesc,
		WelcomeBorder: welcomeBorder,
		BannerLogo:    accentAI,
		BannerKey:     muted,
		BannerValue:   lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"},
		BannerWarning: lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"},
		TrustAccent:   accentAI,
		TrustAccent2:  lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"},
		PathEmphasis:  pathEmphasis,
	}
}

func paletteFixed(app, glam string,
	accentUser, accentAI, muted, dimFG, toolFG, modalBody, errorFG, sepFG, modalBorder, inputBorder lipgloss.Color,
) Palette {
	slashPick := lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#5b9bd5"}
	slashDesc := lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#7c8798"}
	return Palette{
		Appearance:    app,
		GlamourStyle:  glam,
		AccentUser:    accentUser,
		AccentAI:      accentAI,
		Muted:         muted,
		DimFG:         dimFG,
		ToolFG:        toolFG,
		ModalBody:     modalBody,
		ErrorFG:       errorFG,
		SepFG:         sepFG,
		ModalBorder:   modalBorder,
		InputBorder:   inputBorder,
		SlashPickName: slashPick,
		SlashPickDesc: slashDesc,
		WelcomeBorder: sepFG,
		BannerLogo:    accentAI,
		BannerKey:     muted,
		BannerValue:   lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"},
		BannerWarning: lipgloss.Color("#D97706"),
		TrustAccent:   accentAI,
		TrustAccent2:  lipgloss.Color("#0891B2"),
		PathEmphasis:  lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#E2E8F0"},
	}
}

func paletteANSI(app, glam string,
	accentUser, accentAI, muted, dimFG, toolFG, modalBody, errorFG, sepFG, modalBorder, inputBorder lipgloss.Color,
) Palette {
	p := paletteFixed(app, glam,
		accentUser, accentAI, muted, dimFG, toolFG, modalBody, errorFG, sepFG, modalBorder, inputBorder,
	)
	p.SlashPickName = lipgloss.Color("4")
	p.SlashPickDesc = lipgloss.Color("8")
	p.WelcomeBorder = p.SepFG
	return p
}

// paletteCyberpunk returns a dark neon palette — cyan user accent, purple AI accent.
// On-screen: near-black background with glowing cyan/purple text and tool labels in amber-gold.
func paletteCyberpunk() Palette {
	// Neon cyan — user prompt, done states, borders
	accentUser := lipgloss.Color("#00F5FF")
	// Electric purple — AI/assistant, spinner
	accentAI := lipgloss.Color("#BD00FF")
	// Slate-grey muted text
	muted := lipgloss.Color("#7B8FA1")
	// Dimmer version for secondary text
	dimFG := lipgloss.Color("#4A5568")
	// Amber-gold for tool labels (warm contrast against the cold cyan/purple)
	toolFG := lipgloss.Color("#F6C90E")
	// Bright foreground for card body text
	modalBody := lipgloss.Color("#D0E8FF")
	// Hot pink/red for errors
	errorFG := lipgloss.Color("#FF2D6B")
	// Very dark separator (barely-visible rule)
	sepFG := lipgloss.Color("#1E2D3D")
	// Cyan border for modals — ties back to user accent
	modalBorder := lipgloss.Color("#00C8D7")
	// Dark teal for the input box border
	inputBorder := lipgloss.Color("#1C3A4A")
	// Brighter cyan for slash-picker names
	slashPickName := lipgloss.Color("#00F5FF")
	// Dim purple for slash-picker descriptions
	slashPickDesc := lipgloss.Color("#7B5EA7")
	// Neon teal for welcome frame
	welcomeBorder := lipgloss.Color("#0D2F3F")

	return Palette{
		Appearance:    config.UIAppearanceCyberpunk,
		GlamourStyle:  "dark",
		AccentUser:    accentUser,
		AccentAI:      accentAI,
		Muted:         muted,
		DimFG:         dimFG,
		ToolFG:        toolFG,
		ModalBody:     modalBody,
		ErrorFG:       errorFG,
		SepFG:         sepFG,
		ModalBorder:   modalBorder,
		InputBorder:   inputBorder,
		SlashPickName: slashPickName,
		SlashPickDesc: slashPickDesc,
		WelcomeBorder: welcomeBorder,
		BannerLogo:    accentAI,
		BannerKey:     muted,
		BannerValue:   lipgloss.Color("#D0E8FF"),
		BannerWarning: lipgloss.Color("#F6C90E"),
		TrustAccent:   accentAI,
		TrustAccent2:  lipgloss.Color("#00C8D7"),
		PathEmphasis:  lipgloss.Color("#E2F5FF"),
	}
}
