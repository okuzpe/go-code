package app

import tea "charm.land/bubbletea/v2"

// teaKeyIsEnter reports Enter/Return for onboarding dismissal.
//
// Ultraviolet maps CR to [tea.KeyEnter] but maps raw LF to ctrl+j (see key_table); some
// terminals or line disciplines emit LF for Return, so we accept ctrl+j and a few
// string fallbacks. [tea.KeyKpEnter] stringifies as "enter" but is listed explicitly.
func teaKeyIsEnter(msg tea.KeyMsg) bool {
	k := msg.Key()
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
	// Legacy encoding: CR as ctrl+m instead of Enter (ultraviolet LegacyKeyEncoding.CtrlM).
	if msg.String() == "ctrl+m" || k.Keystroke() == "ctrl+m" {
		return true
	}
	t := k.Text
	return t == "\r" || t == "\n" || t == "\r\n"
}

// teaKeyIsEsc reports Escape (accepts legacy "esc" string match too).
func teaKeyIsEsc(msg tea.KeyMsg) bool {
	switch msg.Key().Code {
	case tea.KeyEscape: // KeyEsc is the same code as KeyEscape
		return true
	default:
		return msg.String() == "esc"
	}
}

func teaKeyIsEnterOrEsc(msg tea.KeyMsg) bool {
	return teaKeyIsEnter(msg) || teaKeyIsEsc(msg)
}

// teaKeyIsCtrlC reports Ctrl+C / interrupt for onboarding abort.
// [tea.Key.String] prefers [tea.Key.Text]; some stacks surface ETX (U+0003) as Text instead of "ctrl+c".
func teaKeyIsCtrlC(msg tea.KeyMsg) bool {
	if msg.String() == "ctrl+c" {
		return true
	}
	k := msg.Key()
	if k.Keystroke() == "ctrl+c" {
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
