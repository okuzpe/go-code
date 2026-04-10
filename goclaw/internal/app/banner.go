package app

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

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
)

var asciiLogo = `
  ██████╗  ██████╗  ██████╗██╗      █████╗ ██╗    ██╗
 ██╔════╝ ██╔═══██╗██╔════╝██║     ██╔══██╗██║    ██║
 ██║  ███╗██║   ██║██║     ██║     ███████║██║ █╗ ██║
 ██║   ██║██║   ██║██║     ██║     ██╔══██║██║███╗██║
 ╚██████╔╝╚██████╔╝╚██████╗███████╗██║  ██║╚███╔███╔╝
  ╚═════╝  ╚═════╝  ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝`

// printStartupBanner renders the ASCII header and session summary for the readline REPL path (and the
// plain-text branch when stdout is not a TTY). Default fullscreen TUI does not call this — startup UX
// lives in internal/ui/chat (welcome panel, footer).
func printStartupBanner(version, provider, model, profileName, sessionID, workdir string, disableTools bool) {
	uiMode := "readline REPL (use --readline or GOCLAW_USE_READLINE=1, or GOCLAW_USE_TUI=0)"

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

	var modeLine string
	showCoordinatorHub := strings.EqualFold(strings.TrimSpace(profileName), "coordinator")
	if showCoordinatorHub {
		modeLine = dim.Render("🧭 Coordinator hub — ask for several spawn_agent calls in one message for parallel workers; /workers lists only interactive workers; /profile general-purpose to edit the repo yourself")
	}

	helpLine := dim.Render("/help · /capabilities · /workers · /focus · /detach · /edit multiline · Ctrl+C exit · claw-style REPL")
	uiLine := dim.Render("🖥  UI: " + uiMode)

	fmt.Println(logo)
	fmt.Println(sep)
	fmt.Println(" " + metaLine)
	fmt.Println(" " + sessionLine)
	fmt.Println(" " + workLine)
	fmt.Println(sep)
	fmt.Println(" " + toolsNote)
	if showCoordinatorHub {
		fmt.Println(" " + modeLine)
	}
	fmt.Println(" " + helpLine)
	fmt.Println(" " + uiLine)
	fmt.Println(sep)
	fmt.Println()
}

func printToolApprovalPrompt(w io.Writer, toolName, preview string) {
	if !isTTY(w) {
		fmt.Fprintf(w, "\n[tool] %s\n%s\n", toolName, preview)
		return
	}
	head := lipgloss.NewStyle().Bold(true).Foreground(colWarning).Render("⚡  Tool approval")
	name := lipgloss.NewStyle().Bold(true).Foreground(colAccent2).Render(toolName)
	body := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"}).
		Render(wrapPlain(preview, 76))
	foot := lipgloss.NewStyle().Foreground(colMuted).Italic(true).Render("y + Enter = allow · n = deny · readline prompt below →")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#4B5563"}).
		Padding(0, 1).
		Width(78).
		Render(name + "\n" + body)
	fmt.Fprintln(w)
	fmt.Fprintln(w, head)
	fmt.Fprintln(w, box)
	fmt.Fprintln(w, foot)
}

// wrapPlain breaks text at spaces to fit width (runes); newlines become spaces first.
func wrapPlain(text string, width int) string {
	if width < 12 {
		width = 76
	}
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	var b strings.Builder
	lineLen := 0
	for _, w := range words {
		wl := utf8.RuneCountInString(w)
		if lineLen == 0 {
			b.WriteString(w)
			lineLen = wl
			continue
		}
		if lineLen+1+wl > width {
			lines = append(lines, b.String())
			b.Reset()
			b.WriteString(w)
			lineLen = wl
		} else {
			b.WriteByte(' ')
			b.WriteString(w)
			lineLen += 1 + wl
		}
	}
	if b.Len() > 0 {
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// FormatChatWindowTitle keeps the TUI header on one line for typical terminal widths.
func FormatChatWindowTitle(provider, model, profile string) string {
	if len(model) > 44 {
		model = model[:43] + "…"
	}
	return fmt.Sprintf("goclaw · %s · %s · %s", provider, model, profile)
}
