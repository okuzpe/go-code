package chat

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

func welcomeSectionLines(label string, lines []string, width int, labelStyle, bodyStyle lipgloss.Style) []string {
	if len(lines) == 0 {
		return nil
	}
	out := []string{lipgloss.NewStyle().Width(width).Render(labelStyle.Render(label))}
	for _, ln := range lines {
		out = append(out, lipgloss.NewStyle().Width(width).Render(bodyStyle.Render(ln)))
	}
	return out
}

func welcomePrimarySummary(opt WelcomeOptions, version string) string {
	summary := "goclaw v" + version
	if sub := strings.TrimSpace(opt.Subtitle); sub != "" {
		summary += " · " + sub
	}
	return summary
}

func welcomeDashboardWide(th *Theme, opt WelcomeOptions, version string, termWidth int) []string {
	border := th.WelcomeFrame
	titleAccent := lipgloss.NewStyle().Bold(true).Foreground(th.Assistant.GetForeground())
	dim := th.Dim
	section := th.OverlayTitle

	inner := termWidth - 2
	if inner < 1 {
		inner = 1
	}
	mid := border.Render(th.Icons.ToolCardV())
	rightW := inner / 2
	leftW := inner - 1 - rightW

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
	if u := welcomeOSUser(); u != "" {
		leftLines = append(leftLines, centerInLeft(titleAccent.Render("Welcome back, "+u)))
	} else {
		leftLines = append(leftLines, centerInLeft(titleAccent.Render("Welcome to goclaw")))
	}
	leftLines = append(leftLines, "")
	for _, ln := range wrapSubtitle(welcomePrimarySummary(opt, version), leftW-2) {
		leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(dim.Render(ln)))
	}
	if wd != "" {
		leftLines = append(leftLines, "")
		leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(section.Render("Workspace")))
		for _, ln := range wrapWorkdir(wd, leftW-2) {
			leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(dim.Render(ln)))
		}
	}
	if home {
		leftLines = append(leftLines, "")
		for _, ln := range wrapPlainWords("Started in your home directory. cd into a project for tighter context.", leftW-2) {
			leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(dim.Render(ln)))
		}
	}
	if w := strings.TrimSpace(opt.OllamaWarning); w != "" {
		leftLines = append(leftLines, "")
		for _, ln := range wrapPlainWords(w, leftW-2) {
			leftLines = append(leftLines, lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(th.ErrorStyle.Render(ln)))
		}
	}

	quickStart := wrapPlainWords("/help — commands. Ctrl+P — profile. Ctrl+T — tool history. / opens actions.", rightW-1)
	workflows := wrapPlainWords("Chat naturally, use @ for files, ! for shell, and /mode plan when you want planning first.", rightW-1)
	var rightLines []string
	rightLines = append(rightLines, welcomeSectionLines("Quick Start", quickStart, rightW, section, dim)...)
	rightLines = append(rightLines, "")
	rightLines = append(rightLines, welcomeSectionLines("Workflows", workflows, rightW, section, dim)...)
	if opt.FileWriteToolsHidden {
		rightLines = append(rightLines, "")
		if opt.HubDelegatesCoding {
			for _, ln := range wrapPlainWords("This mode/profile is read-only for workspace edits — delegate coding with spawn_agent or switch to /mode build.", rightW-1) {
				rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
			}
		} else {
			for _, ln := range wrapPlainWords("Read-only mode/profile — switch to /mode build to edit files here.", rightW-1) {
				rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
			}
		}
	} else if opt.WriteApprovalRequired {
		rightLines = append(rightLines, "")
		for _, ln := range wrapPlainWords("Write tools are available — approve each call, or use /allow-writes for this session.", rightW-1) {
			rightLines = append(rightLines, lipgloss.NewStyle().Width(rightW).Align(lipgloss.Left).Render(dim.Render(ln)))
		}
	}

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

	titlePlain := " goclaw "
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
	accent := th.OverlayTitle

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
		writeJoined(wrapPlainWords(welcomePrimarySummary(opt, version), contentMax), dim.Render)
	} else {
		writeJoined(wrapPlainWords("Welcome to goclaw", contentMax), titleStyle.Render)
		body.WriteString("\n")
		writeJoined(wrapPlainWords(welcomePrimarySummary(opt, version), contentMax), dim.Render)
	}
	if wd := strings.TrimSpace(opt.Workdir); wd != "" {
		if abs, err := filepath.Abs(wd); err == nil {
			wd = abs
		}
		body.WriteString("\n")
		body.WriteString(accent.Render("Workspace"))
		body.WriteString("\n")
		for _, ln := range wrapWorkdir(wd, contentMax) {
			body.WriteString(dim.Render(ln))
			body.WriteString("\n")
		}
		if workspaceLooksLikeUserHome(wd) {
			body.WriteString("\n")
			body.WriteString(dim.Render("Tip: cd into a project folder for focused context"))
		}
	}
	body.WriteString("\n\n")
	body.WriteString(accent.Render("Quick Start"))
	body.WriteString("\n")
	for _, ln := range wrapPlainWords("/help · Ctrl+P profile · Ctrl+T tool history · / shows actions.", contentMax) {
		body.WriteString(dim.Render(ln))
		body.WriteString("\n")
	}
	body.WriteString("\n")
	body.WriteString(accent.Render("Workflows"))
	body.WriteString("\n")
	for _, ln := range wrapPlainWords("Chat naturally, use @ for files, ! for shell, and /mode plan when you want planning first.", contentMax) {
		body.WriteString(dim.Render(ln))
		body.WriteString("\n")
	}
	if opt.FileWriteToolsHidden {
		body.WriteString("\n")
		if opt.HubDelegatesCoding {
			for _, ln := range wrapPlainWords("This mode/profile is read-only for workspace edits — spawn_agent for coding, or /mode build for direct edits.", contentMax) {
				body.WriteString(dim.Render(ln))
				body.WriteString("\n")
			}
		} else {
			for _, ln := range wrapPlainWords("Read-only mode/profile — switch to /mode build for file edits.", contentMax) {
				body.WriteString(dim.Render(ln))
				body.WriteString("\n")
			}
		}
	} else if opt.WriteApprovalRequired {
		body.WriteString("\n")
		for _, ln := range wrapPlainWords("Write tools are available — approve each call, or use /allow-writes for this session.", contentMax) {
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

	trimmed := strings.TrimSpace(body.String())

	frameStyle := th.WelcomeFrame.Copy().
		Border(lipgloss.RoundedBorder(), true, true, true, true).
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
