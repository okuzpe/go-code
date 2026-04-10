package chat

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// WelcomeOptions configures the optional startup panel (Phase 2 parity with Claude Code home).
type WelcomeOptions struct {
	Version          string
	Subtitle         string // e.g. provider · model · profile
	Workdir          string
	RecentSessionIDs []string
}

// defaultWelcomeWrap is used when terminal width is not known yet (before first WindowSizeMsg).
const defaultWelcomeWrap = 72

// WelcomeDashboardLines returns minimal framed content to prepend before the chat title.
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
		contentMax = termWidth - 3 // left border + inner padding
		if contentMax < 20 {
			contentMax = 20
		}
	}

	name := strings.TrimSpace(os.Getenv("USER"))
	if name == "" {
		name = strings.TrimSpace(os.Getenv("USERNAME"))
	}
	greeting := "Welcome back!"
	if name != "" {
		greeting = "Welcome back, " + name + "!"
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(th.Assistant.GetForeground())
	dim := th.Dim

	var body strings.Builder
	body.WriteString(titleStyle.Render("goclaw " + v))
	body.WriteString("\n")
	body.WriteString(dim.Render(greeting))

	if sub := strings.TrimSpace(opt.Subtitle); sub != "" {
		for _, ln := range wrapSubtitle(sub, contentMax) {
			body.WriteString("\n")
			body.WriteString(dim.Render(ln))
		}
	}

	if wd := strings.TrimSpace(opt.Workdir); wd != "" {
		if abs, err := filepath.Abs(wd); err == nil {
			wd = abs
		}
		body.WriteString("\n")
		for _, ln := range wrapWorkdir(wd, contentMax) {
			body.WriteString("\n")
			body.WriteString(dim.Render(ln))
		}
		if workspaceLooksLikeUserHome(wd) {
			for _, ln := range wrapPlainWords("Tip: cd into a project folder for a focused workspace.", contentMax) {
				body.WriteString("\n")
				body.WriteString(dim.Render(ln))
			}
		}
	}

	// Compact tips — short lines so they rarely wrap mid-command.
	body.WriteString("\n")
	body.WriteString(dim.Render("Plain language or /capabilities"))
	body.WriteString("\n")
	body.WriteString(dim.Render("/help   /plan   /sessions   /theme"))
	body.WriteString("\n")
	body.WriteString(dim.Render("Docs: CLAUDE.md  .goclaw/"))

	if len(opt.RecentSessionIDs) > 0 {
		body.WriteString("\n")
		var parts []string
		for i, id := range opt.RecentSessionIDs {
			if i >= 4 {
				break
			}
			parts = append(parts, shortenSessionID(id))
		}
		recent := "Recent: " + strings.Join(parts, " · ")
		for _, ln := range wrapPlainWords(recent, contentMax) {
			body.WriteString("\n")
			body.WriteString(dim.Render(ln))
		}
	}

	// Single left rule — calmer than a full box; avoids heavy corners in narrow terminals.
	frame := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#4B5563"}).
		Padding(0, 1).
		Render(strings.TrimSpace(body.String()))

	return strings.Split(frame, "\n")
}

// wrapPlainWords breaks at spaces; fits lines to maxW display cells (plain text, no ANSI).
func wrapPlainWords(text string, maxW int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxW < 12 {
		return []string{text}
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

// wrapSubtitle prefers breaks at " · " so provider / model / profile stay readable.
func wrapSubtitle(sub string, maxW int) []string {
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return nil
	}
	parts := strings.Split(sub, " · ")
	var chunks []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			chunks = append(chunks, p)
		}
	}
	if len(chunks) == 0 {
		return wrapPlainWords(sub, maxW)
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
	// Rejoin drive + first chunk for Windows (e.g. C: + \Users\...)
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

func shortenSessionID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…"
}
