package app

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
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

var asciiLogo = `
  ┏━╸┏━┓┏━╸┏  ┏━┓╻ ╻
  ┃╺┓┃ ┃┃  ┃  ┣━┫┃╻┃
  ┗━┛┗━┛┗━╸┗━╸╹ ╹┗┻┛`

// printStartupBanner renders the ASCII header and session summary for the readline REPL path (and the
// plain-text branch when stdout is not a TTY). Default fullscreen TUI does not call this — startup UX
// lives in internal/ui/chat (welcome panel, footer).
func printStartupBanner(version, provider, model, profileName, sessionID, workdir string, disableTools bool, uiAppearance string, profile agents.Profile) {
	if !isTTY(os.Stdout) {
		fmt.Printf("goclaw %s  provider=%s  model=%s  profile=%s  session=%s\n",
			version, provider, model, profileName, sessionID)
		fmt.Printf("Workspace: %s\n", workdir)
		if disableTools {
			fmt.Println("Tools disabled.")
		} else {
			fmt.Println("Tools in Ask mode — answer y/N. Ctrl+C to exit.")
		}
		if !profile.AllowsWorkspaceFileWrites() {
			if profile.AllowsSpawnAgentDelegation() {
				fmt.Println("Note: hub profile (write tools hidden) — use spawn_agent for coding or switch to profile general-purpose.")
			} else {
				fmt.Println("Note: read-only profile — use profile general-purpose for direct file edits.")
			}
		}
		fmt.Println()
		return
	}

	p := terminalstyle.PaletteForAppearance(uiAppearance)
	keySt := lipgloss.NewStyle().Foreground(p.BannerKey)
	valSt := lipgloss.NewStyle().Foreground(p.BannerValue)
	dim := lipgloss.NewStyle().Foreground(p.Muted)
	accent := lipgloss.NewStyle().Foreground(p.BannerLogo).Bold(true)

	logo := accent.Render(asciiLogo)
	sep := dim.Render(strings.Repeat("─", 44))

	kv := func(key, value string) string {
		return keySt.Render(key) + dim.Render(": ") + valSt.Render(value)
	}

	info := lipgloss.JoinVertical(lipgloss.Left,
		"  "+kv("provider", provider)+"  "+kv("model", model),
		"  "+kv("profile", profileName)+"  "+kv("session", truncate(sessionID, 12)),
		"  "+kv("workspace", workdir),
	)

	var toolsNote string
	if disableTools {
		toolsNote = lipgloss.NewStyle().Foreground(p.BannerWarning).Render("  Tools disabled")
	} else {
		toolsNote = dim.Render("  Tools: ask mode (y/N before execution)")
	}

	helpLine := dim.Render("  /help  /capabilities  /workers  Ctrl+C exit")

	fmt.Println(logo)
	fmt.Println(sep)
	fmt.Println(info)
	fmt.Println(sep)
	fmt.Println(toolsNote)

	if !profile.AllowsWorkspaceFileWrites() {
		if profile.AllowsSpawnAgentDelegation() {
			fmt.Println(dim.Render("  Hub profile (no direct write tools) — delegate with spawn_agent or /profile general-purpose"))
		} else {
			fmt.Println(dim.Render("  Read-only profile — /profile general-purpose for direct file edits"))
		}
	}

	fmt.Println(helpLine)
	fmt.Println(sep)
	fmt.Println()
}

func printToolApprovalPrompt(w io.Writer, toolName, preview string, uiAppearance string) {
	p := terminalstyle.PaletteForAppearance(uiAppearance)
	if !isTTY(w) {
		fmt.Fprintf(w, "\n[tool] %s\n%s\n", toolName, preview)
		return
	}
	dim := lipgloss.NewStyle().Foreground(p.Muted)
	head := lipgloss.NewStyle().Bold(true).Foreground(p.BannerWarning).Render("⚡ Tool approval")
	name := lipgloss.NewStyle().Bold(true).Foreground(p.TrustAccent2).Render(toolName)
	body := lipgloss.NewStyle().Foreground(p.ModalBody).Render(wrapPlain(preview, 76))
	foot := dim.Italic(true).Render("y/Enter allow  n deny")

	cardBorder := lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}
	box := lipgloss.NewStyle().
		Border(cardBorder, true, true, true, true).
		BorderForeground(p.InputBorder).
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
	if len(model) > 40 {
		model = model[:39] + "…"
	}
	return fmt.Sprintf("goclaw  %s/%s  %s", provider, model, profile)
}
