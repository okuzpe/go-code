package app

import (
	"fmt"
	"io"
	"os"
	"strings"

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

var (
	colAccent  = lipgloss.Color("#7C3AED")
	colAccent2 = lipgloss.Color("#06B6D4")
	colUser    = lipgloss.Color("#10B981")
	colAI      = lipgloss.Color("#818CF8")
	colWarning = lipgloss.Color("#F59E0B")
	colMuted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	colBorder  = lipgloss.Color("#4C1D95")
)

var asciiLogo = `
  ██████╗  ██████╗  ██████╗██╗      █████╗ ██╗    ██╗
 ██╔════╝ ██╔═══██╗██╔════╝██║     ██╔══██╗██║    ██║
 ██║  ███╗██║   ██║██║     ██║     ███████║██║ █╗ ██║
 ██║   ██║██║   ██║██║     ██║     ██╔══██║██║███╗██║
 ╚██████╔╝╚██████╔╝╚██████╗███████╗██║  ██║╚███╔███╔╝
  ╚═════╝  ╚═════╝  ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝`

// printStartupBanner renders the ASCII header and session summary. useTUI selects copy for the UI line.
func printStartupBanner(version, provider, model, profileName, sessionID, workdir string, disableTools, useTUI bool) {
	uiMode := "readline REPL (default; type at the > prompt)"
	if useTUI {
		uiMode = "fullscreen TUI (--tui)"
	}

	if !isTTY(os.Stdout) {
		fmt.Printf("goclaw %s  provider=%s  model=%s  profile=%s  session=%s\n",
			version, provider, model, profileName, sessionID)
		fmt.Printf("Workspace: %s\n", workdir)
		if disableTools {
			fmt.Println("Tools disabled.")
		} else {
			fmt.Println("Tools in Ask mode — answer y/N. Ctrl+C to exit.")
		}
		fmt.Printf("UI: %s\n", uiMode)
		fmt.Println()
		return
	}

	logoStyle := lipgloss.NewStyle().
		Foreground(colAccent).
		Bold(true)
	logo := logoStyle.Render(asciiLogo)

	pill := func(label, value string, fg lipgloss.Color) string {
		lStyle := lipgloss.NewStyle().
			Background(fg).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1).
			Render(label)
		vStyle := lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"}).
			Render(value)
		return lStyle + " " + vStyle
	}

	dim := lipgloss.NewStyle().Foreground(colMuted)
	sep := dim.Render(strings.Repeat("─", 58))

	metaLine := strings.Join([]string{
		pill("v", version, colAccent),
		pill("provider", provider, colAccent2),
		pill("model", model, colAI),
	}, "  ")

	sessionLine := strings.Join([]string{
		pill("profile", profileName, colUser),
		pill("session", truncate(sessionID, 12), lipgloss.Color("#6366F1")),
	}, "  ")

	workLine := dim.Render("📂 " + workdir)

	var toolsNote string
	if disableTools {
		toolsNote = lipgloss.NewStyle().Foreground(colWarning).Render("⚠  Tools disabled (--no-tools or GOCLAW_DISABLE_TOOLS=1)")
	} else {
		toolsNote = dim.Render("🔧 Tools active — ask mode prompts y/N before execution")
	}

	var helpLine string
	if useTUI {
		helpLine = dim.Render("💡 /help · /edit multiline · Ctrl+L clear · Esc or Ctrl+C exit")
	} else {
		helpLine = dim.Render("💡 /help · /edit multiline · Ctrl+C exit · type freely at the prompt (claw-style REPL)")
	}
	uiLine := dim.Render("🖥  UI: " + uiMode)

	fmt.Println(logo)
	fmt.Println(sep)
	fmt.Println(" " + metaLine)
	fmt.Println(" " + sessionLine)
	fmt.Println(" " + workLine)
	fmt.Println(sep)
	fmt.Println(" " + toolsNote)
	fmt.Println(" " + helpLine)
	fmt.Println(" " + uiLine)
	fmt.Println(sep)
	fmt.Println()
}

func printToolApprovalPrompt(w io.Writer, toolName, preview string) {
	if !isTTY(w) {
		fmt.Fprintf(w, "\n[tool] %s\narguments: %s\n", toolName, preview)
		return
	}
	label := lipgloss.NewStyle().
		Background(colWarning).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1).
		Render("TOOL")
	name := lipgloss.NewStyle().Bold(true).Foreground(colAccent2).Render(toolName)
	argsStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"})
	fmt.Fprintf(w, "\n%s %s\n%s %s\n", label, name, argsStyle.Render("arguments:"), argsStyle.Render(preview))
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
