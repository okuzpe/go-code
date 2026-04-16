package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
)

const (
	onboardingTrustRuleInnerMin = 40
	onboardingTrustRuleMaxCols  = 76
	onboardingTrustWrapMin      = 48
)

func trustStyles(uiAppearance string) (
	titleStyle lipgloss.Style,
	ruleStyle lipgloss.Style,
	pathStyle lipgloss.Style,
	bodyStyle lipgloss.Style,
	hintStyle lipgloss.Style,
	selStyle lipgloss.Style,
	numStyle lipgloss.Style,
) {
	p := terminalstyle.PaletteForAppearance(uiAppearance)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(p.TrustAccent)
	ruleStyle = lipgloss.NewStyle().Foreground(p.Muted)
	pathStyle = lipgloss.NewStyle().
		Foreground(p.PathEmphasis).
		Bold(true)
	bodyStyle = lipgloss.NewStyle().Foreground(p.ModalBody)
	hintStyle = lipgloss.NewStyle().Foreground(p.Muted).Italic(true)
	selStyle = lipgloss.NewStyle().Foreground(p.TrustAccent2).Bold(true)
	numStyle = lipgloss.NewStyle().Foreground(p.TrustAccent).Bold(true)
	return
}

// renderOnboardingTrustStepTUI returns the workspace trust screen for Bubble Tea onboarding.
func renderOnboardingTrustStepTUI(uiAppearance, absWd string, cursor, width int) string {
	if width <= 0 {
		width = 80
	}
	inner := width - 2
	trustTitleStyle, trustRuleStyle, trustPathStyle, trustBodyStyle, trustHintStyle, trustSelStyle, _ := trustStyles(uiAppearance)
	title := trustTitleStyle.Render("Accessing workspace")
	rule := trustRuleStyle.Render(strings.Repeat("─", min(max(inner, onboardingTrustRuleInnerMin), onboardingTrustRuleMaxCols)))
	pathLine := trustPathStyle.Render(absWd)
	body := trustBodyStyle.Render(wrapPlain("Quick safety check: trust this folder for hooks under .goclaw/?", max(inner, onboardingTrustWrapMin)))

	var opts strings.Builder
	labels := []string{"Yes, I trust this folder", "No, exit"}
	for i, label := range labels {
		line := trustBodyStyle.Render(label)
		if i == cursor {
			opts.WriteString(trustSelStyle.Render("> ") + line + "\n")
		} else {
			opts.WriteString("  " + line + "\n")
		}
	}

	foot := trustHintStyle.Render("Press Enter to confirm · Press Esc to cancel")

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(rule)
	b.WriteString("\n\n")
	b.WriteString(pathLine)
	b.WriteString("\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(opts.String())
	b.WriteString("\n")
	b.WriteString(foot)
	b.WriteString("\n")
	return b.String()
}
