package main

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := int(f.Fd())
	return term.IsTerminal(fd)
}

func printStartupBanner(provider, model, profileName, sessionID string, disableTools bool) {
	line := fmt.Sprintf("goclaw v0.1.0  provider=%s  model=%s  profile=%s  session=%s",
		provider, model, profileName, sessionID)
	toolsLine := "Tools in Ask mode prompt on stderr (answer y/N). Ctrl+C to exit."
	if disableTools {
		toolsLine = "Tools disabled (--no-tools or GOCLAW_DISABLE_TOOLS=1)."
	}
	helpLine := "Type /help for slash commands (plan, apply-plan, profile, new, save, memory, compact)."

	if !isTTY(os.Stdout) {
		fmt.Println(line)
		fmt.Println(toolsLine)
		fmt.Println(helpLine)
		fmt.Println()
		return
	}

	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#9ca3af"})
	detail := fmt.Sprintf(" v0.1.0  provider=%s  model=%s  profile=%s  session=%s",
		provider, model, profileName, sessionID)
	banner := accent.Render("goclaw") + dim.Render(detail)
	fmt.Println(banner)
	fmt.Println(dim.Render(toolsLine))
	fmt.Println(dim.Render(helpLine))
	fmt.Println()
}

// printToolApprovalPrompt writes the tool name and argument preview before Ask-mode confirmation.
func printToolApprovalPrompt(w io.Writer, toolName, preview string) {
	if !isTTY(w) {
		fmt.Fprintf(w, "\n[tool] %s\narguments: %s\n", toolName, preview)
		return
	}
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render("[tool]")
	name := lipgloss.NewStyle().Bold(true).Render(toolName)
	argsStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#d1d5db"})
	fmt.Fprintf(w, "\n%s %s\n%s %s\n", label, name, argsStyle.Render("arguments:"), argsStyle.Render(preview))
}
