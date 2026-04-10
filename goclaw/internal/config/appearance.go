package config

import "strings"

// Known UI appearance presets for the chat TUI (settings key ui_appearance).
const (
	UIAppearanceAuto            = "auto"
	UIAppearanceDark          = "dark"
	UIAppearanceLight         = "light"
	UIAppearanceDarkColorblind  = "dark_colorblind"
	UIAppearanceLightColorblind = "light_colorblind"
	UIAppearanceDarkANSI      = "dark_ansi"
	UIAppearanceLightANSI     = "light_ansi"
)

// UIAppearanceChoices lists presets shown in onboarding and /theme (excluding auto for picker).
var UIAppearanceChoices = []string{
	UIAppearanceDark,
	UIAppearanceLight,
	UIAppearanceDarkColorblind,
	UIAppearanceLightColorblind,
	UIAppearanceDarkANSI,
	UIAppearanceLightANSI,
}

// NormalizeUIAppearance returns a canonical preset or "auto" when empty or unknown.
func NormalizeUIAppearance(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", UIAppearanceAuto:
		return UIAppearanceAuto
	case UIAppearanceDark, UIAppearanceLight,
		UIAppearanceDarkColorblind, UIAppearanceLightColorblind,
		UIAppearanceDarkANSI, UIAppearanceLightANSI:
		return v
	default:
		return UIAppearanceAuto
	}
}
