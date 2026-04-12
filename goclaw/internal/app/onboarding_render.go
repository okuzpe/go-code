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

	onboardingTermMinCols      = 40
	onboardingTermFallbackWrap = 68
	onboardingGlamourWrapMin   = 48
	onboardingGlamourWrapMax   = 100
	onboardingStdoutWrapMin    = 52
	onboardingStdoutWrapMax    = 78
)

//go:embed onboarding_welcome.md
var onboardingWelcomeMD string

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

// termWidthForOnboarding returns a sensible wrap width for glamour (readline or TUI).
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

// printOnboardingSecurityWelcome prints the first-run security screen (Charm lipgloss + glamour on TTY).
func printOnboardingSecurityWelcome(version, uiAppearance string) {
	if !isTTY(os.Stdout) {
		printOnboardingWelcomePlain(version)
		return
	}
	rendered := renderOnboardingSecurityMarkdown(version, uiAppearance, 0)
	if rendered == "" {
		printOnboardingWelcomeLipglossFallback(version, uiAppearance)
		return
	}
	fmt.Println()
	fmt.Println(rendered)
	fmt.Println()
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

func printOnboardingWelcomePlain(version string) {
	fmt.Print(onboardingWelcomePlainBlock(version, stdoutWrapWidth()))
}

func printOnboardingWelcomeLipglossFallback(version, uiAppearance string) {
	w := stdoutWrapWidth()
	p := terminalstyle.PaletteForAppearance(uiAppearance)
	title := lipgloss.NewStyle().Bold(true).Foreground(p.TrustAccent).Render(onboardingWelcomeTitle(version))
	rule := lipgloss.NewStyle().Foreground(p.Muted).Render(strings.Repeat("─", min(w, onboardingGlamourRuleMaxCols)))
	sub := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#E5E7EB"}).
		Render("Before you start")
	body := lipgloss.NewStyle().Foreground(p.ModalBody)
	dim := lipgloss.NewStyle().Foreground(p.Muted).Italic(true)

	fmt.Println()
	fmt.Println(title)
	fmt.Println(rule)
	fmt.Println()
	fmt.Println(sub)
	fmt.Println()
	fmt.Println(body.Render(wrapPlain("• "+onbBulletModel, w)))
	fmt.Println()
	fmt.Println(body.Render(wrapPlain("• "+onbBulletTrust, w)))
	fmt.Println()
	fmt.Println(body.Render(wrapPlain(securityDocURL()+" is a repo path, not a clickable link. Press s for the bundled full text.", w)))
	fmt.Println()
	fmt.Println(dim.Render("Press Enter or Esc to continue"))
	fmt.Println(dim.Render("Press Ctrl+C to exit"))
	fmt.Println()
}
