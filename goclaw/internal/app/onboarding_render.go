package app

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
	"golang.org/x/term"
)

const (
	onboardingGlamourRuleMaxCols = 72
	onboardingPlainRuleMaxCols   = 64

	onboardingSecurityDocRuleMax = 78

	onboardingTermMinCols      = 40
	onboardingTermFallbackWrap = 68
	onboardingGlamourWrapMin   = 48
	onboardingGlamourWrapMax   = 100
	onboardingStdoutWrapMin    = 52
	onboardingStdoutWrapMax    = 78
)

//go:embed onboarding_welcome.md
var onboardingWelcomeMD string

//go:embed onboarding_security_full.md
var onboardingSecurityFullMD string

func formatVersionSuffix(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		return ""
	}
	return " " + v
}

// glamourRenderSecurityFull renders the bundled security.md mirror (ANSI) for the onboarding doc viewport.
func glamourRenderSecurityFull(uiAppearance string, wrap int) string {
	md := strings.TrimSpace(onboardingSecurityFullMD)
	if md == "" {
		return ""
	}
	if wrap < onboardingGlamourWrapMin {
		wrap = onboardingGlamourWrapMin
	}
	opts := config.GlamourTermRendererOptions(uiAppearance, wrap)
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return ""
	}
	out, err := r.Render(md)
	if err != nil {
		return ""
	}
	return strings.TrimRight(out, "\n")
}

func onboardingWelcomeTitle(version string) string {
	h1 := "Welcome to goclaw"
	if v := strings.TrimSpace(version); v != "" {
		h1 += " " + v
	}
	return h1
}

func onboardingWelcomeMarkdown(version string) string {
	h1 := onboardingWelcomeTitle(version)
	return strings.ReplaceAll(onboardingWelcomeMD, "__H1__", h1)
}

// termWidthForOnboarding returns a sensible wrap width for glamour (onboarding TUI / plain fallback).
func termWidthForOnboarding(explicit int) int {
	if explicit > 0 {
		return min(max(explicit-4, onboardingGlamourWrapMin), onboardingGlamourWrapMax)
	}
	return stdoutWrapWidth()
}

// stdoutWrapWidth is used for plain-text wrapping and lipgloss fallbacks.
func stdoutWrapWidth() int {
	fd := int(os.Stdout.Fd())
	w, _, err := term.GetSize(fd)
	if err != nil || w < onboardingTermMinCols {
		return onboardingTermFallbackWrap
	}
	return min(max(w-6, onboardingStdoutWrapMin), onboardingStdoutWrapMax)
}

// renderOnboardingSecurityMarkdown returns ANSI-styled welcome + security (glamour + lipgloss accent strip).
// uiAppearance should match config.Config.UIAppearance (same Glamour profile as chat.NewThemeForAppearance).
func renderOnboardingSecurityMarkdown(version, uiAppearance string, width int) string {
	md := onboardingWelcomeMarkdown(version)
	wrap := termWidthForOnboarding(width)
	opts := config.GlamourTermRendererOptions(uiAppearance, wrap)
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return ""
	}
	out, err := r.Render(md)
	if err != nil {
		return ""
	}
	out = strings.TrimRight(out, "\n")
	// Subtle top rule so it matches startup banner rhythm.
	p := terminalstyle.PaletteForAppearance(uiAppearance)
	rule := lipgloss.NewStyle().
		Foreground(p.Muted).
		Render(strings.Repeat("─", min(wrap+2, onboardingGlamourRuleMaxCols)))
	return rule + "\n" + out
}

const (
	onbBulletModel = "Models can misunderstand or suggest unsafe steps. Read every reply before running shell commands or accepting edits."
	onbBulletTrust = "Prompt injection: untrusted repos, dependencies, or pasted text can steer the agent. Use goclaw only on codebases and inputs you trust."
)

// onboardingWelcomePlainBlock is plain text (wrapped) for non-TTY stdout or TUI glamour fallback.
func onboardingWelcomePlainBlock(version string, width int) string {
	w := width
	if w <= 0 {
		w = stdoutWrapWidth()
	}
	var o strings.Builder
	o.WriteString("\n")
	o.WriteString(fmt.Sprintf(" Welcome to goclaw%s\n", formatVersionSuffix(version)))
	o.WriteString(strings.Repeat("─", min(w, onboardingPlainRuleMaxCols)))
	o.WriteString("\n\n")
	o.WriteString(wrapPlain("Before you start", w))
	o.WriteString("\n\n")
	o.WriteString(wrapPlain("• "+onbBulletModel, w))
	o.WriteString("\n\n")
	o.WriteString(wrapPlain("• "+onbBulletTrust, w))
	o.WriteString("\n\n")
	o.WriteString(wrapPlain(securityDocURL()+" is a repo path, not a clickable link here. Press s for the bundled full text.", w))
	o.WriteString("\n\n")
	o.WriteString(wrapPlain("Press Enter or Esc to continue", w))
	o.WriteString("\n\n")
	o.WriteString(wrapPlain("Press Ctrl+C to exit", w))
	o.WriteString("\n")
	return o.String()
}

// renderOnboardingSecurityDocFrame draws the path header + rule + viewport body + footer for the security doc view.
func renderOnboardingSecurityDocFrame(viewportText string, w int, uiAppearance string) string {
	p := terminalstyle.PaletteForAppearance(uiAppearance)
	pathStyle := lipgloss.NewStyle().Bold(true).Foreground(p.TrustAccent2)
	rule := lipgloss.NewStyle().Foreground(p.Muted).Render(strings.Repeat("╌", min(max(w-2, 40), onboardingSecurityDocRuleMax)))
	head := pathStyle.Render("docs/goclaw/security.md") + "\n" + rule
	foot := lipgloss.NewStyle().
		Foreground(p.Muted).
		Italic(true).
		Render("↑ ↓ PgUp PgDn · scroll · wheel · Press Enter or Esc to continue")
	return "\n" + head + "\n\n" + viewportText + "\n" + foot + "\n"
}
