package chat

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/text"
)

// WelcomeOptions configures the optional startup panel (Phase 2 parity with Claude Code home).
type WelcomeOptions struct {
	Version  string
	Subtitle string // e.g. provider · model · profile
	Workdir  string
	Profile  string // active profile name; used to show profile-specific tips
	// FileWriteToolsHidden when true, show how to switch to a coding profile or delegate via spawn_agent.
	FileWriteToolsHidden bool
	// HubDelegatesCoding when true with FileWriteToolsHidden, hint mentions spawn_agent (coordinator-style profiles).
	HubDelegatesCoding bool
	// WriteApprovalRequired when true (and FileWriteToolsHidden is false), hint that write tools need per-call approval
	// and how to enable auto-approval. Set when yolo_threshold is below the workspace-write risk score (60).
	WriteApprovalRequired bool
	// OllamaWarning is a short English notice shown on the welcome panel when Ollama is unreachable or the model is missing.
	OllamaWarning string
}

// defaultWelcomeWrap is used when terminal width is not known yet (before first WindowSizeMsg).
const defaultWelcomeWrap = 72

// welcomeWideMinCols enables a two-column dashboard when the terminal is wide enough.
const welcomeWideMinCols = 86

// welcomeNarrowContentMaxFloor avoids clipping on very narrow terminals (see WindowSizeMsg path).
const welcomeNarrowContentMaxFloor = 12

// WelcomeDashboardLines returns a framed home panel before the transcript.
// termWidth is the terminal width in cells; use 0 to wrap at defaultWelcomeWrap.
// Empty Version skips the panel.
func WelcomeDashboardLines(th *Theme, opt WelcomeOptions, termWidth int) []string {
	v := strings.TrimSpace(opt.Version)
	if v == "" {
		return nil
	}
	if th == nil {
		th = DefaultTheme()
	}

	contentMax := defaultWelcomeWrap
	if termWidth > 0 {
		contentMax = termWidth - 4
		// Never wider than the terminal body; keep a small floor so narrow windows do not clip.
		if contentMax < welcomeNarrowContentMaxFloor {
			contentMax = welcomeNarrowContentMaxFloor
		}
	}

	if termWidth >= welcomeWideMinCols {
		return welcomeDashboardWide(th, opt, v, termWidth)
	}
	return welcomeDashboardNarrow(th, opt, v, termWidth, contentMax)
}

func welcomeBorderColor() lipgloss.TerminalColor {
	return lipgloss.AdaptiveColor{Light: "#C7CCD6", Dark: "#3D4450"}
}

// welcomeOSUser returns a short display name for "Welcome back …" (USER / USERNAME).
func welcomeOSUser() string {
	if s := strings.TrimSpace(os.Getenv("USER")); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("USERNAME")); s != "" {
		return s
	}
	return ""
}

// welcomeBrandGlyphLines returns a compact terminal panda for the hero column (centered by callers).
// Face is full-block art only: █ plus spaces (eyes and bridge are negative space).
func welcomeBrandGlyphLines() []string {
	return []string{
		"     ██     ██     ",
		"    ███████████    ",
		"   ███   █   ███   ",
		"    ███████████    ",
		"     █████████     ",
		"    ██     ██      ",
	}
}

// welcomeBrandRibbon is a single-row wordmark with soft rules under the mascot.
func welcomeBrandRibbon(th *Theme, columnWidth int) string {
	if columnWidth < 10 {
		return ""
	}
	dim := th.Dim
	accent := lipgloss.NewStyle().Bold(true).Foreground(th.Assistant.GetForeground())
	inner := dim.Render("╌╌") + " " + accent.Render("goclaw") + " " + dim.Render("╌╌")
	return lipgloss.PlaceHorizontal(columnWidth, lipgloss.Center, inner, lipgloss.WithWhitespaceChars(" "))
}

func welcomeDashboardWide(th *Theme, opt WelcomeOptions, version string, termWidth int) []string {
	borderFG := welcomeBorderColor()
	border := lipgloss.NewStyle().Foreground(borderFG)
	titleAccent := lipgloss.NewStyle().Bold(true).Foreground(th.Assistant.GetForeground())
	welcomeHi := lipgloss.NewStyle().Bold(true).Foreground(th.Assistant.GetForeground())
	dim := th.Dim
	section := lipgloss.NewStyle().Bold(true).Foreground(th.ToolCardHead.GetForeground())

	inner := termWidth - 2
	if inner < 1 {
		inner = 1
	}
	mid := border.Render(th.Icons.ToolCardV())
	// Wider hero column, tips column wide enough for wrapped sentences (min 28 when room allows).
	rightW := 28
	if inner < 50 {
		rightW = 22
	}
	if rightW > inner/3 {
		rightW = inner / 3
	}
	leftW := inner - 1 - rightW
	if leftW < 24 {
		leftW = 24
		rightW = inner - 1 - leftW
	}

	wd := strings.TrimSpace(opt.Workdir)
	if wd != "" {
		if abs, err := filepath.Abs(wd); err == nil {
			wd = abs
		}
	}
	home := workspaceLooksLikeUserHome(wd)

	centerInLeft := func(s string) string {
		return lipgloss.PlaceHorizontal(leftW, lipgloss.Center, s, lipgloss.WithWhitespaceChars(" "))
	}

	var leftLines []string
	leftLines = append(leftLines, "")
	if u := welcomeOSUser(); u != "" {
		leftLines = append(leftLines, centerInLeft(welcomeHi.Render("Welcome back, "+u)))
	} else {
		leftLines = append(leftLines, centerInLeft(welcomeHi.Render("Welcome to goclaw")))
	}
	leftLines = append(leftLines, "")
	for _, ln := range welcomeBrandGlyphLines() {
		leftLines = append(leftLines, centerInLeft(dim.Render(strings.TrimSpace(ln))))
	}
	leftLines = append(leftLines, "")
	if ribbon := welcomeBrandRibbon(th, leftW); ribbon != "" {
		leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(ribbon))
	}
	leftLines = append(leftLines, "")
	if sub := strings.TrimSpace(opt.Subtitle); sub != "" {
		for _, ln := range wrapSubtitle(sub, leftW-2) {
			leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(dim.Render(ln)))
		}
	}
	if wd != "" {
		for _, ln := range wrapWorkdir(wd, leftW-2) {
			leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(dim.Render(ln)))
		}
	}
	if home {
		for _, ln := range wrapPlainWords("Note: started in your home directory. cd into a project for a focused workspace.", leftW-2) {
			leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(dim.Render(ln)))
		}
	}
	if w := strings.TrimSpace(opt.OllamaWarning); w != "" {
		leftLines = append(leftLines, "")
		for _, ln := range wrapPlainWords(w, leftW-2) {
			leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(th.ErrorStyle.Render(ln)))
		}
	}

	var rightLines []string
	rightLines = append(rightLines, "")
	rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(section.Render("Tips")))
	for _, ln := range wrapPlainWords("/help — commands. Ctrl+P — profiles. Ctrl+T — tool history.", rightW-1) {
		rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
	}
	for _, ln := range wrapPlainWords("One message = one turn; say continue if it stopped after reads only.", rightW-1) {
		rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
	}
	rightLines = append(rightLines, "")
	rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(section.Render("Flows")))
	for _, ln := range wrapPlainWords("Plan: /profile plan, then /plan run when you are ready to implement.", rightW-1) {
		rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
	}
	for _, ln := range wrapPlainWords("Hub: /profile coordinator — /workers and /focus for delegates.", rightW-1) {
		rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
	}
	if opt.FileWriteToolsHidden {
		rightLines = append(rightLines, "")
		if opt.HubDelegatesCoding {
			for _, ln := range wrapPlainWords("This profile hides write_file from the model — delegate coding with spawn_agent (e.g. profile general-purpose or stack-coder), or /profile general-purpose for direct edits.", rightW-1) {
				rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
			}
		} else {
			for _, ln := range wrapPlainWords("Read-only profile: use /profile general-purpose to edit files in this session.", rightW-1) {
				rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
			}
		}
	} else if opt.WriteApprovalRequired {
		rightLines = append(rightLines, "")
		for _, ln := range wrapPlainWords("Write tools available · each call needs approval (y/n/Enter) — or /allow-writes to skip prompts this session.", rightW-1) {
			rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
		}
	}
	rightLines = append(rightLines, "")

	n := len(leftLines)
	if len(rightLines) > n {
		n = len(rightLines)
	}
	for len(leftLines) < n {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < n {
		rightLines = append(rightLines, "")
	}

	rows := make([]string, 0, n)
	for i := 0; i < n; i++ {
		lCell := lipgloss.NewStyle().Width(leftW).Align(lipgloss.Left).Render(leftLines[i])
		rCell := lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(rightLines[i])
		row := lipgloss.JoinHorizontal(lipgloss.Top, lCell, mid, rCell)
		fullRow := lipgloss.JoinHorizontal(lipgloss.Top, border.Render(th.Icons.ToolCardV()), row, border.Render(th.Icons.ToolCardV()))
		rows = append(rows, fullRow)
	}

	titlePlain := "goclaw v" + version
	topPrefix := th.Icons.WelcomeTopPrefix()
	h := th.Icons.ToolCardH()
	dashAfterTitle := termWidth - lipgloss.Width(topPrefix) - lipgloss.Width(titlePlain) - 2
	if dashAfterTitle < 1 {
		dashAfterTitle = 1
	}
	topLine := border.Render(topPrefix) + titleAccent.Render(titlePlain) + border.Render(" "+strings.Repeat(h, dashAfterTitle)+th.Icons.WelcomeTopRightCorner())
	botLine := border.Render(th.Icons.WelcomeBottomLeftCorner() + strings.Repeat(h, inner) + th.Icons.WelcomeBottomRightCorner())

	innerBlock := strings.Join(rows, "\n")
	framed := topLine + "\n" + innerBlock + "\n" + botLine
	return strings.Split(framed, "\n")
}

func welcomeDashboardNarrow(th *Theme, opt WelcomeOptions, version string, termWidth int, contentMax int) []string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(th.Assistant.GetForeground())
	dim := th.Dim
	accent := lipgloss.NewStyle().Bold(true).Foreground(th.ToolCardHead.GetForeground())

	var body strings.Builder
	writeJoined := func(lines []string, render func(...string) string) {
		for i, ln := range lines {
			body.WriteString(render(ln))
			if i < len(lines)-1 {
				body.WriteString("\n")
			}
		}
	}
	if u := welcomeOSUser(); u != "" {
		writeJoined(wrapPlainWords("Welcome back, "+u, contentMax), titleStyle.Render)
		body.WriteString("\n")
		writeJoined(wrapPlainWords("goclaw v"+version, contentMax), dim.Render)
	} else {
		writeJoined(wrapPlainWords("goclaw v"+version, contentMax), titleStyle.Render)
	}
	if sub := strings.TrimSpace(opt.Subtitle); sub != "" {
		body.WriteString("\n")
		for _, ln := range wrapSubtitle(sub, contentMax) {
			body.WriteString(dim.Render(ln))
		}
	}
	if wd := strings.TrimSpace(opt.Workdir); wd != "" {
		if abs, err := filepath.Abs(wd); err == nil {
			wd = abs
		}
		body.WriteString("\n")
		for _, ln := range wrapWorkdir(wd, contentMax) {
			body.WriteString(dim.Render(ln))
		}
		if workspaceLooksLikeUserHome(wd) {
			body.WriteString("\n")
			body.WriteString(dim.Render("Tip: cd into a project folder for focused context"))
		}
	}
	body.WriteString("\n\n")
	body.WriteString(accent.Render("Tips"))
	body.WriteString("\n")
	for _, ln := range wrapPlainWords("/help · Ctrl+P (profiles) · Ctrl+T (tool history) · one message = one turn; continue if it stopped early.", contentMax) {
		body.WriteString(dim.Render(ln))
		body.WriteString("\n")
	}
	for _, ln := range wrapPlainWords("Plan: /profile plan → /plan run. Hub: /profile coordinator → /workers, /focus.", contentMax) {
		body.WriteString(dim.Render(ln))
		body.WriteString("\n")
	}
	if opt.FileWriteToolsHidden {
		body.WriteString("\n")
		if opt.HubDelegatesCoding {
			for _, ln := range wrapPlainWords("This profile hides write tools — spawn_agent for coding, or /profile general-purpose for direct edits.", contentMax) {
				body.WriteString(dim.Render(ln))
				body.WriteString("\n")
			}
		} else {
			for _, ln := range wrapPlainWords("Read-only profile — /profile general-purpose for direct file edits.", contentMax) {
				body.WriteString(dim.Render(ln))
				body.WriteString("\n")
			}
		}
	} else if opt.WriteApprovalRequired {
		body.WriteString("\n")
		for _, ln := range wrapPlainWords("Write tools available · each call needs approval (y/n/Enter) — or /allow-writes to skip prompts this session.", contentMax) {
			body.WriteString(dim.Render(ln))
			body.WriteString("\n")
		}
	}
	if w := strings.TrimSpace(opt.OllamaWarning); w != "" {
		body.WriteString("\n\n")
		for _, ln := range wrapPlainWords(w, contentMax) {
			body.WriteString(th.ErrorStyle.Render(ln))
			body.WriteString("\n")
		}
	}

	// Mascot glyphs are fixed-width art (~21 cells). Skip them when the body is too narrow or
	// PlaceHorizontal would not constrain wide runes, so framed lines stay within the terminal.
	if contentMax >= 20 {
		body.WriteString("\n")
		for _, ln := range welcomeBrandGlyphLines() {
			plain := strings.TrimSpace(ln)
			if lipgloss.Width(plain) > contentMax {
				plain = text.TruncateRunesHard(plain, contentMax)
			}
			centered := lipgloss.PlaceHorizontal(contentMax, lipgloss.Center, plain, lipgloss.WithWhitespaceChars(" "))
			body.WriteString(dim.Render(centered))
			body.WriteString("\n")
		}
	}

	ribbonWidth := contentMax + 4
	if termWidth > 0 && termWidth-4 < ribbonWidth {
		ribbonWidth = termWidth - 4
		if ribbonWidth < 12 {
			ribbonWidth = 12
		}
	}
	ribbon := welcomeBrandRibbon(th, ribbonWidth)
	trimmed := strings.TrimSpace(body.String())
	if ribbon != "" {
		trimmed = trimmed + "\n" + ribbon
	}

	frameStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(welcomeBorderColor()).
		Padding(0, 1)
	if termWidth > 0 {
		frameStyle = frameStyle.Width(termWidth)
	}
	frame := frameStyle.Render(trimmed)
	return strings.Split(frame, "\n")
}

// wrapPlainWords breaks at spaces; fits lines to maxW display cells (plain text, no ANSI).
func wrapPlainWords(text string, maxW int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxW < 12 {
		if lipgloss.Width(text) <= maxW {
			return []string{text}
		}
		return wrapHardRunes(text, maxW)
	}
	if lipgloss.Width(text) <= maxW {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return wrapHardRunes(text, maxW)
	}
	var lines []string
	var b strings.Builder
	curW := 0
	for _, w := range words {
		ww := lipgloss.Width(w)
		if ww > maxW {
			if b.Len() > 0 {
				lines = append(lines, b.String())
				b.Reset()
				curW = 0
			}
			lines = append(lines, wrapHardRunes(w, maxW)...)
			continue
		}
		if curW == 0 {
			b.WriteString(w)
			curW = ww
			continue
		}
		if curW+1+ww <= maxW {
			b.WriteByte(' ')
			b.WriteString(w)
			curW += 1 + ww
		} else {
			lines = append(lines, b.String())
			b.Reset()
			b.WriteString(w)
			curW = ww
		}
	}
	if b.Len() > 0 {
		lines = append(lines, b.String())
	}
	return lines
}

func wrapHardRunes(s string, maxW int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if maxW < 8 {
		return []string{s}
	}
	r := []rune(s)
	var out []string
	for len(r) > maxW {
		out = append(out, string(r[:maxW]))
		r = r[maxW:]
	}
	if len(r) > 0 {
		out = append(out, string(r))
	}
	return out
}

// wrapSubtitle prefers breaks at " · " or double-space chunks (new title format).
func wrapSubtitle(sub string, maxW int) []string {
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return nil
	}

	if strings.Contains(sub, " · ") {
		if lines := wrapSubtitleFromChunks(splitSubtitleChunks(sub, " · "), maxW); len(lines) > 0 {
			return lines
		}
	}
	if strings.Contains(sub, "  ") {
		var chunks []string
		for _, p := range strings.Split(sub, "  ") {
			p = strings.TrimSpace(p)
			if p != "" {
				chunks = append(chunks, p)
			}
		}
		if len(chunks) > 1 {
			if lines := wrapSubtitleFromChunks(chunks, maxW); len(lines) > 0 {
				return lines
			}
		}
	}
	return wrapPlainWords(sub, maxW)
}

func splitSubtitleChunks(sub, sep string) []string {
	parts := strings.Split(sub, sep)
	var chunks []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			chunks = append(chunks, p)
		}
	}
	return chunks
}

func wrapSubtitleFromChunks(chunks []string, maxW int) []string {
	if len(chunks) == 0 {
		return nil
	}
	var lines []string
	var line strings.Builder
	for i, ch := range chunks {
		sep := ""
		if i > 0 {
			sep = " · "
		}
		cand := line.String() + sep + ch
		if line.Len() == 0 {
			line.WriteString(ch)
			continue
		}
		if lipgloss.Width(cand) <= maxW {
			line.WriteString(sep)
			line.WriteString(ch)
		} else {
			lines = append(lines, strings.TrimSpace(line.String()))
			line.Reset()
			line.WriteString(ch)
		}
	}
	if line.Len() > 0 {
		lines = append(lines, strings.TrimSpace(line.String()))
	}

	var out []string
	for _, ln := range lines {
		if lipgloss.Width(ln) <= maxW {
			out = append(out, ln)
			continue
		}
		out = append(out, wrapPlainWords(ln, maxW)...)
	}
	return out
}

// wrapWorkdir breaks long paths at separators first, then hard-wraps segments.
func wrapWorkdir(path string, maxW int) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if lipgloss.Width(path) <= maxW {
		return []string{path}
	}
	sep := "/"
	if strings.Contains(path, "\\") {
		sep = "\\"
	}
	raw := strings.Split(path, sep)
	var pieces []string
	for i, p := range raw {
		if i > 0 && p != "" {
			pieces = append(pieces, sep+p)
		} else if p != "" {
			pieces = append(pieces, p)
		} else if i == 0 && sep == "\\" {
			pieces = append(pieces, "")
		}
	}
	if len(pieces) >= 2 && strings.HasSuffix(pieces[0], ":") {
		pieces[1] = pieces[0] + pieces[1]
		pieces = pieces[1:]
	}

	var lines []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
		}
	}
	for _, piece := range pieces {
		if piece == "" {
			continue
		}
		if cur.Len() == 0 {
			cur.WriteString(piece)
			continue
		}
		if lipgloss.Width(cur.String()+piece) <= maxW {
			cur.WriteString(piece)
		} else {
			flush()
			if lipgloss.Width(piece) > maxW {
				lines = append(lines, wrapHardRunes(piece, maxW)...)
			} else {
				cur.WriteString(piece)
			}
		}
	}
	flush()

	var out []string
	for _, ln := range lines {
		if lipgloss.Width(ln) <= maxW {
			out = append(out, ln)
		} else {
			out = append(out, wrapHardRunes(ln, maxW)...)
		}
	}
	return out
}

func workspaceLooksLikeUserHome(workdir string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	w, err := filepath.Abs(workdir)
	if err != nil {
		return false
	}
	h, err := filepath.Abs(home)
	if err != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(w), filepath.Clean(h))
}
