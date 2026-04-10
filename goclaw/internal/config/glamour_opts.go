package config

import "github.com/charmbracelet/glamour"

// GlamourTermRendererOptions returns options for glamour.NewTermRenderer that match
// chat.NewThemeForAppearance markdown styling (dark / light / ascii / auto).
// Use this from internal/app or other packages that must not import internal/ui/chat.
func GlamourTermRendererOptions(uiAppearance string, wordWrap int) []glamour.TermRendererOption {
	opts := []glamour.TermRendererOption{glamour.WithWordWrap(wordWrap)}
	switch NormalizeUIAppearance(uiAppearance) {
	case UIAppearanceDark, UIAppearanceDarkColorblind:
		opts = append(opts, glamour.WithStandardStyle("dark"))
	case UIAppearanceLight, UIAppearanceLightColorblind:
		opts = append(opts, glamour.WithStandardStyle("light"))
	case UIAppearanceDarkANSI, UIAppearanceLightANSI:
		opts = append(opts, glamour.WithStandardStyle("ascii"))
	default:
		opts = append(opts, glamour.WithAutoStyle())
	}
	return opts
}
