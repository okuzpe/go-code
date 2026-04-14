package app

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/text"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
	"golang.org/x/term"
)

const (
	bannerSeparatorWidth    = 44
	bannerSessionIDMaxRunes = 12
	chatTitleModelMaxRunes  = 40
	toolPreviewWrapWidth    = 76
	toolApprovalBoxWidth    = 78
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

// profileWriteFootnote returns a hint when the profile cannot use workspace write tools (read-only or hub).
func profileWriteFootnote(profile agents.Profile) (string, bool) {
	if profile.AllowsWorkspaceFileWrites() {
		return "", false
	}
	if profile.AllowsSpawnAgentDelegation() {
		return "Hub profile (no direct write tools) — delegate with spawn_agent or /profile general-purpose.", true
	}
	return "Read-only profile — /profile general-purpose for direct file edits.", true
}

// printStartupBanner renders the ASCII header and session summary for the readline REPL path (and the
// plain-text branch when stdout is not a TTY). Default fullscreen TUI does not call this — startup UX
// lives in internal/ui/chat (welcome panel, footer).
func printStartupBanner(version, provider, model, profileName, sessionID, workdir string, disableTools bool, uiAppearance string, profile agents.Profile, ollamaNumCtx int) {
	footnote, showFootnote := profileWriteFootnote(profile)

	if !isTTY(os.Stdout) {
		fmt.Printf("goclaw %s  provider=%s  model=%s  profile=%s  session=%s\n",
			version, provider, model, profileName, sessionID)
		fmt.Printf("Workspace: %s\n", workdir)
		if disableTools {
			fmt.Println("Tools disabled.")
		} else {
			fmt.Println("Tools in Ask mode — answer y/N. Ctrl+C to exit.")
		}
		if showFootnote {
			fmt.Println("Note: " + footnote)
		}
		if ollamaNumCtx > 0 && ollamaNumCtx < OllamaNumCtxBannerWarnBelow {
			fmt.Printf("Note: ollama_num_ctx=%d is below %d — long prompts plus tool schemas may truncate; see docs/goclaw/usage.md.\n", ollamaNumCtx, OllamaNumCtxBannerWarnBelow)
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
	sep := dim.Render(strings.Repeat("─", bannerSeparatorWidth))

	kv := func(key, value string) string {
		return keySt.Render(key) + dim.Render(": ") + valSt.Render(value)
	}

	info := lipgloss.JoinVertical(lipgloss.Left,
		"  "+kv("provider", provider)+"  "+kv("model", model),
		"  "+kv("profile", profileName)+"  "+kv("session", truncate(sessionID, bannerSessionIDMaxRunes)),
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

	if showFootnote {
		fmt.Println(dim.Render("  " + footnote))
	}
	if ollamaNumCtx > 0 && ollamaNumCtx < OllamaNumCtxBannerWarnBelow {
		fmt.Println(dim.Render(fmt.Sprintf("  Note: ollama_num_ctx=%d is below %d — long prompts plus tool schemas may truncate; see docs/goclaw/usage.md.", ollamaNumCtx, OllamaNumCtxBannerWarnBelow)))
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
	body := lipgloss.NewStyle().Foreground(p.ModalBody).Render(wrapPlain(preview, toolPreviewWrapWidth))
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
		Width(toolApprovalBoxWidth).
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
	for _, word := range words {
		wordLen := utf8.RuneCountInString(word)
		if lineLen == 0 {
			b.WriteString(word)
			lineLen = wordLen
			continue
		}
		if lineLen+1+wordLen > width {
			lines = append(lines, b.String())
			b.Reset()
			b.WriteString(word)
			lineLen = wordLen
		} else {
			b.WriteByte(' ')
			b.WriteString(word)
			lineLen += 1 + wordLen
		}
	}
	if b.Len() > 0 {
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, max int) string {
	return text.TruncateRunes(s, max)
}

// FormatChatWindowTitle keeps the TUI header on one line for typical terminal widths.
func FormatChatWindowTitle(provider, model, profile string) string {
	model = truncate(model, chatTitleModelMaxRunes)
	return fmt.Sprintf("goclaw  %s/%s  %s", provider, model, profile)
}
