package config

import "strings"

// Known UI appearance presets for the chat TUI (settings key ui_appearance).
const (
	UIAppearanceAuto            = "auto"
	UIAppearanceDark            = "dark"
	UIAppearanceLight           = "light"
	UIAppearanceDarkColorblind  = "dark_colorblind"
	UIAppearanceLightColorblind = "light_colorblind"
	UIAppearanceDarkANSI        = "dark_ansi"
	UIAppearanceLightANSI       = "light_ansi"
	UIAppearanceCyberpunk       = "cyberpunk"
)

// UIAppearanceChoices lists presets shown in onboarding and /theme (excluding auto for picker).
var UIAppearanceChoices = []string{
	UIAppearanceDark,
	UIAppearanceLight,
	UIAppearanceDarkColorblind,
	UIAppearanceLightColorblind,
	UIAppearanceDarkANSI,
	UIAppearanceLightANSI,
	UIAppearanceCyberpunk,
}

// ThemePresetList returns ui_appearance values for interactive pickers (auto first, then UIAppearanceChoices).
func ThemePresetList() []string {
	out := make([]string, 0, 1+len(UIAppearanceChoices))
	out = append(out, UIAppearanceAuto)
	out = append(out, UIAppearanceChoices...)
	return out
}

// NormalizeUIAppearance returns a canonical preset or "auto" when empty or unknown.
func NormalizeUIAppearance(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", UIAppearanceAuto:
		return UIAppearanceAuto
	case UIAppearanceDark, UIAppearanceLight,
		UIAppearanceDarkColorblind, UIAppearanceLightColorblind,
		UIAppearanceDarkANSI, UIAppearanceLightANSI,
		UIAppearanceCyberpunk:
		return v
	default:
		return UIAppearanceAuto
	}
}
