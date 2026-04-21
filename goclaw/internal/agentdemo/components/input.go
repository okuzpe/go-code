package components

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
)

// NewComposer returns a focused multiline textarea with Shift+Enter / Alt+Enter for newline.
func NewComposer(accent lipgloss.Style) textarea.Model {
	in := textarea.New()
	in.Placeholder = "Message…  Tab completes demo commands · Shift+Enter newline · Ctrl+M mode · Ctrl+E logs"
	in.Prompt = "> "
	in.ShowLineNumbers = true
	in.SetHeight(1)
	in.SetWidth(0)
	in.CharLimit = 0
	in.Focus()
	st := in.Styles()
	st.Focused.Prompt = st.Focused.Prompt.Foreground(accent.GetForeground())
	st.Focused.LineNumber = st.Focused.LineNumber.Foreground(lipgloss.Color("240"))
	st.Focused.CursorLineNumber = st.Focused.CursorLineNumber.Foreground(accent.GetForeground())
	in.SetStyles(st)
	km := in.KeyMap
	km.InsertNewline.SetKeys("shift+enter", "alt+enter")
	in.KeyMap = km
	return in
}

// EnterInsertsNewline reports whether Enter should insert a newline (Shift/Alt held).
func EnterInsertsNewline(msg tea.KeyMsg) bool {
	k := msg.Key()
	if k.Code != tea.KeyEnter && k.Code != tea.KeyReturn {
		return false
	}
	return k.Mod&(tea.ModShift|tea.ModAlt) != 0
}
