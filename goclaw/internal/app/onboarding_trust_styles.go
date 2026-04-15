package app

import (
	"fmt"
	"os"
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

// trustReadlineTTYGlamBody is the Lip Gloss trust block without the title line (rule, path, copy, prompt).
// Used for a two-phase print so a plain heading can flush before this heavier ANSI block is built.
func trustReadlineTTYGlamBody(uiAppearance, absWd string, width int) string {
	if width <= 0 {
		width = 80
	}
	inner := max(width-2, onboardingTrustWrapMin)
	_, trustRuleStyle, trustPathStyle, trustBodyStyle, trustHintStyle, _, trustNumStyle := trustStyles(uiAppearance)
	rule := trustRuleStyle.Render(strings.Repeat("─", min(inner, onboardingTrustRuleMaxCols)))
	pathLine := trustPathStyle.Render(absWd)

	p1 := trustBodyStyle.Render(wrapPlain("Quick safety check: Is this a project you created or one you trust (your own code, a well-known open source project, or work from your team)? If not, review this folder first.", inner))
	p2 := trustBodyStyle.Render(wrapPlain("goclaw can read, edit, and run tools in this directory. With trusted_workspace, project hooks under .goclaw/ and plugin hooks may also run.", inner))

	opt1 := trustNumStyle.Render("1.") + " " + trustBodyStyle.Render("Yes, I trust this folder")
	opt2 := trustNumStyle.Render("2.") + " " + trustBodyStyle.Render("No, exit")
	prompt := trustHintStyle.Render("Choose (1-2): ")

	var b strings.Builder
	b.WriteString(rule)
	b.WriteString("\n\n")
	b.WriteString(pathLine)
	b.WriteString("\n\n")
	b.WriteString(p1)
	b.WriteString("\n\n")
	b.WriteString(p2)
	b.WriteString("\n\n")
	b.WriteString(opt1)
	b.WriteString("\n")
	b.WriteString(opt2)
	b.WriteString("\n\n")
	b.WriteString(prompt)
	return b.String()
}

// renderOnboardingTrustStepReadlineTTY is the Lip Gloss version for line-based onboarding.
func renderOnboardingTrustStepReadlineTTY(uiAppearance, absWd string, width int) string {
	if width <= 0 {
		width = 80
	}
	trustTitleStyle, _, _, _, _, _, _ := trustStyles(uiAppearance)
	title := trustTitleStyle.Render("Accessing workspace")
	return "\n" + title + "\n" + trustReadlineTTYGlamBody(uiAppearance, absWd, width)
}

func flushOnboardingStdout() {
	_ = os.Stdout.Sync()
}

func printOnboardingTrustStepReadlinePlain(absWd string) {
	fmt.Println()
	fmt.Println(" Accessing workspace:")
	fmt.Println()
	fmt.Printf(" %s\n", absWd)
	fmt.Println()
	fmt.Println(" Quick safety check: Is this a project you created or one you trust (your own")
	fmt.Println(" code, a well-known open source project, or work from your team)? If not, review")
	fmt.Println(" this folder first.")
	fmt.Println()
	fmt.Println(" goclaw can read, edit, and run tools in this directory. With trusted_workspace,")
	fmt.Println(" project hooks under .goclaw/ and plugin hooks may also run.")
	fmt.Println()
	fmt.Println("  1. Yes, I trust this folder")
	fmt.Println("  2. No, exit")
	fmt.Print("\n Choose (1-2): ")
}

// printOnboardingTrustStepReadline prints the trust step with Lip Gloss on a TTY.
func printOnboardingTrustStepReadline(uiAppearance, absWd string) {
	if !isTTY(os.Stdout) {
		printOnboardingTrustStepReadlinePlain(absWd)
		return
	}
	w := stdoutWrapWidth()
	// Plain heading first: appears immediately after Bubble Tea tears down, while Lip Gloss builds below.
	fmt.Print("\nAccessing workspace\n")
	flushOnboardingStdout()
	fmt.Print(trustReadlineTTYGlamBody(uiAppearance, absWd, w))
	flushOnboardingStdout()
}
