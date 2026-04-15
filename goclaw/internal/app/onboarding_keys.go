package app

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	tea "charm.land/bubbletea/v2"
)

// teaKeyIsEnter reports Enter/Return for onboarding dismissal.
//
// Ultraviolet maps CR to [tea.KeyEnter] but maps raw LF to ctrl+j (see key_table); some
// terminals or line disciplines emit LF for Return, so we accept ctrl+j and a few
// string fallbacks. [tea.KeyKpEnter] stringifies as "enter" but is listed explicitly.
func teaKeyIsEnter(msg tea.KeyMsg) bool {
	k := msg.Key()
	// Ultraviolet's matcher handles named keys and modifier chords (e.g. shift+enter).
	if uv.Key(k).MatchString("enter", "kpenter", "shift+enter", "ctrl+j", "ctrl+m") {
		return true
	}
	switch k.Code {
	case tea.KeyEnter, tea.KeyKpEnter:
		return true
	case '\n': // LF as key code (distinct from [tea.KeyEnter], which is CR)
		return true
	}
	switch msg.String() {
	case "enter", "\r", "\n", "\r\n", "ctrl+j":
		return true
	}
	if k.Keystroke() == "ctrl+m" || msg.String() == "ctrl+m" {
		return true
	}
	t := k.Text
	return t == "\r" || t == "\n" || t == "\r\n"
}

// teaKeyIsEsc reports Escape for onboarding (dismiss / abort paths).
// Matches legacy ESC-as-ctrl+[ (ultraviolet LegacyKeyEncoding.CtrlOpenBracket) and literal ESC in Text.
func teaKeyIsEsc(msg tea.KeyMsg) bool {
	k := msg.Key()
	switch k.Code {
	case tea.KeyEscape: // KeyEsc is the same code as KeyEscape
		return true
	}
	s := msg.String()
	if s == "esc" || s == "\x1b" {
		return true
	}
	if k.Keystroke() == "esc" {
		return true
	}
	if k.Text == "\x1b" {
		return true
	}
	// Legacy: ESC byte decoded as ctrl+[ instead of KeyEscape.
	if k.Mod.Contains(tea.ModCtrl) && k.Code == '[' {
		return true
	}
	return false
}

// teaKeyIsOnboardingSecurityDocKey is the "show full security doc" shortcut (S / s).
func teaKeyIsOnboardingSecurityDocKey(msg tea.KeyMsg) bool {
	s := msg.String()
	if len(s) == 1 && strings.EqualFold(s, "s") {
		return true
	}
	k := msg.Key()
	if k.Code == 's' || k.Code == 'S' {
		return !k.Mod.Contains(tea.ModCtrl) && !k.Mod.Contains(tea.ModAlt)
	}
	return false
}

func teaKeyIsEnterOrEsc(msg tea.KeyMsg) bool {
	return teaKeyIsEnter(msg) || teaKeyIsEsc(msg)
}

// teaKeyIsCtrlC reports Ctrl+C / interrupt for onboarding abort.
// [tea.Key.String] prefers [tea.Key.Text]; some stacks surface ETX (U+0003) as Text instead of "ctrl+c".
func teaKeyIsCtrlC(msg tea.KeyMsg) bool {
	k := msg.Key()
	if uv.Key(k).MatchString("ctrl+c") {
		return true
	}
	if msg.String() == "ctrl+c" {
		return true
	}
	if k.Mod.Contains(tea.ModCtrl) && (k.Code == 'c' || k.Code == 'C') {
		return true
	}
	if k.Text == "\x03" || msg.String() == "\x03" {
		return true
	}
	if k.Code == '\x03' {
		return true
	}
	return false
}
