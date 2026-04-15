package chat

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unsafe"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	lipv2 "charm.land/lipgloss/v2"
	rw "github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// composeInputView returns the compose textarea view, with @workspace tokens and path chips
// styled like user transcript lines when reflection can read bubbles textarea internals.
// Otherwise it falls back to the stock textarea.View().
func (m *Model) composeInputView() string {
	if strings.TrimSpace(m.input.Value()) == "" {
		return m.input.View()
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	out, ok := textareaRichViewportView(&m.input, th, m.workdir)
	if !ok {
		return m.input.View()
	}
	return out
}

// textareaRichViewportView mirrors textarea.Model.View, replacing plain segments with
// renderUserRefsLine(..., style.Render). Wrap logic is adapted from bubbles/textarea (MIT).
func textareaRichViewportView(m *textarea.Model, th *Theme, workdir string) (string, bool) {
	vp := textareaReflectViewport(m)
	if vp == nil {
		return "", false
	}
	rows := textareaReflectValueRows(m)
	if rows == nil {
		return "", false
	}
	inner, ok := textareaRichInnerContent(m, rows, th, workdir)
	if !ok {
		return "", false
	}
	prev := vp.GetContent()
	vp.SetContent(inner)
	out := vp.View()
	vp.SetContent(prev)
	st := textareaActiveStyleState(m)
	return st.Base.Render(out), true
}

func textareaActiveStyleState(m *textarea.Model) *textarea.StyleState {
	s := m.Styles()
	if m.Focused() {
		return &s.Focused
	}
	return &s.Blurred
}

func composeComputedPrompt(st textarea.StyleState) lipv2.Style {
	return st.Prompt.Inherit(st.Base).Inline(true)
}

func composeComputedText(st textarea.StyleState) lipv2.Style {
	return st.Text.Inherit(st.Base).Inline(true)
}

func composeComputedCursorLine(st textarea.StyleState) lipv2.Style {
	return st.CursorLine.Inherit(st.Base).Inline(true)
}

func composeComputedLineNumber(st textarea.StyleState) lipv2.Style {
	return st.LineNumber.Inherit(st.Base).Inline(true)
}

func composeComputedCursorLineNumber(st textarea.StyleState) lipv2.Style {
	return st.CursorLineNumber.Inherit(st.CursorLine).Inherit(st.Base).Inline(true)
}

func composeComputedEndOfBuffer(st textarea.StyleState) lipv2.Style {
	return st.EndOfBuffer.Inherit(st.Base).Inline(true)
}

// composePromptRaw matches textarea.promptView when promptFunc is nil (goclaw does not set one).
func composePromptRaw(m *textarea.Model, _ int) string {
	return m.Prompt
}

func composeLineNumberView(m *textarea.Model, st *textarea.StyleState, n int, isCursorLine bool) string {
	str := " "
	if n > 0 {
		str = strconv.Itoa(n)
	}
	textStyle := composeComputedText(*st)
	lineNumberStyle := composeComputedLineNumber(*st)
	if isCursorLine {
		textStyle = composeComputedCursorLine(*st)
		lineNumberStyle = composeComputedCursorLineNumber(*st)
	}
	digits := len(strconv.Itoa(m.MaxHeight))
	str = fmt.Sprintf(" %*v ", digits, str)
	return textStyle.Render(lineNumberStyle.Render(str))
}

// textareaRichInnerContent mirrors textarea.(*Model).view for non-placeholder content.
func textareaRichInnerContent(m *textarea.Model, value [][]rune, th *Theme, workdir string) (string, bool) {
	vc := textareaReflectVirtualCursor(m)
	if vc == nil {
		return "", false
	}
	st := textareaActiveStyleState(m)
	lineInfo := m.LineInfo()
	var style lipv2.Style
	var s strings.Builder
	var widestLineNumber int
	displayLine := 0

	vc.TextStyle = composeComputedCursorLine(*st)

	for l, line := range value {
		wrappedLines := composeWrapLine(line, m.Width())
		if m.Line() == l {
			style = composeComputedCursorLine(*st)
		} else {
			style = composeComputedText(*st)
		}
		for wl, wrappedLine := range wrappedLines {
			prompt := composePromptRaw(m, displayLine)
			prompt = composeComputedPrompt(*st).Render(prompt)
			s.WriteString(style.Render(prompt))
			displayLine++

			var ln string
			if m.ShowLineNumbers {
				if wl == 0 {
					isCursorLine := m.Line() == l
					ln = composeLineNumberView(m, st, l+1, isCursorLine)
					s.WriteString(ln)
				} else {
					isCursorLine := m.Line() == l
					ln = composeLineNumberView(m, st, -1, isCursorLine)
					s.WriteString(ln)
				}
			}
			lnw := uniseg.StringWidth(ln)
			if lnw > widestLineNumber {
				widestLineNumber = lnw
			}

			strwidth := uniseg.StringWidth(string(wrappedLine))
			padding := m.Width() - strwidth
			if strwidth > m.Width() {
				wrappedLine = []rune(strings.TrimSuffix(string(wrappedLine), " "))
				padding -= m.Width() - strwidth
			}
			plainRender := func(p string) string {
				if p == "" {
					return ""
				}
				return style.Render(p)
			}
			if m.Line() == l && lineInfo.RowOffset == wl {
				left := string(wrappedLine[:lineInfo.ColumnOffset])
				s.WriteString(renderUserRefsLine(left, th, workdir, plainRender))
				if m.Column() >= len(line) && lineInfo.CharOffset >= m.Width() {
					vc.SetChar(" ")
					s.WriteString(style.Render(vc.View()))
				} else {
					vc.SetChar(string(wrappedLine[lineInfo.ColumnOffset]))
					s.WriteString(style.Render(vc.View()))
					right := string(wrappedLine[lineInfo.ColumnOffset+1:])
					s.WriteString(renderUserRefsLine(right, th, workdir, plainRender))
				}
			} else {
				s.WriteString(renderUserRefsLine(string(wrappedLine), th, workdir, plainRender))
			}
			s.WriteString(style.Render(strings.Repeat(" ", max(0, padding))))
			s.WriteRune('\n')
		}
	}

	for range m.Height() {
		s.WriteString(composePromptRaw(m, displayLine))
		displayLine++
		leftGutter := string(m.EndOfBufferCharacter)
		rightGapWidth := m.Width() - uniseg.StringWidth(leftGutter) + widestLineNumber
		rightGap := strings.Repeat(" ", max(0, rightGapWidth))
		s.WriteString(composeComputedEndOfBuffer(*st).Render(leftGutter + rightGap))
		s.WriteRune('\n')
	}

	return s.String(), true
}

// composeWrapLine is adapted from charm.land/bubbles/v2/textarea.wrap (MIT License).
func composeWrapLine(runes []rune, width int) [][]rune {
	var (
		lines  = [][]rune{{}}
		word   []rune
		row    int
		spaces int
	)

	for _, r := range runes {
		if unicode.IsSpace(r) {
			spaces++
		} else {
			word = append(word, r)
		}

		if spaces > 0 {
			if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces > width {
				row++
				lines = append(lines, []rune{})
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], composeRepeatSpaces(spaces)...)
				spaces = 0
				word = nil
			} else {
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], composeRepeatSpaces(spaces)...)
				spaces = 0
				word = nil
			}
		} else {
			lastCharLen := rw.RuneWidth(word[len(word)-1])
			if uniseg.StringWidth(string(word))+lastCharLen > width {
				if len(lines[row]) > 0 {
					row++
					lines = append(lines, []rune{})
				}
				lines[row] = append(lines[row], word...)
				word = nil
			}
		}
	}

	if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces >= width {
		lines = append(lines, []rune{})
		lines[row+1] = append(lines[row+1], word...)
		spaces++
		lines[row+1] = append(lines[row+1], composeRepeatSpaces(spaces)...)
	} else {
		lines[row] = append(lines[row], word...)
		spaces++
		lines[row] = append(lines[row], composeRepeatSpaces(spaces)...)
	}

	return lines
}

func composeRepeatSpaces(n int) []rune {
	return []rune(strings.Repeat(" ", n))
}

// textareaReflectViewport returns the textarea's internal viewport pointer.
// Go 1.26+ forbids reflect.Value.Interface on unexported fields, so we read the pointer word with unsafe.
func textareaReflectViewport(m *textarea.Model) *viewport.Model {
	fv := reflect.ValueOf(m).Elem().FieldByName("viewport")
	if !fv.IsValid() || fv.Kind() != reflect.Ptr || fv.IsNil() {
		return nil
	}
	pp := (*unsafe.Pointer)(unsafe.Pointer(fv.UnsafeAddr()))
	return (*viewport.Model)(*pp)
}

// textareaReflectVirtualCursor returns a pointer to the textarea's embedded virtual cursor model.
// Same unexported-field constraint as textareaReflectViewport.
func textareaReflectVirtualCursor(m *textarea.Model) *cursor.Model {
	fv := reflect.ValueOf(m).Elem().FieldByName("virtualCursor")
	if !fv.IsValid() {
		return nil
	}
	return (*cursor.Model)(unsafe.Pointer(fv.UnsafeAddr()))
}

func textareaReflectValueRows(m *textarea.Model) [][]rune {
	v := reflect.ValueOf(m).Elem().FieldByName("value")
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return nil
	}
	n := v.Len()
	out := make([][]rune, n)
	for i := 0; i < n; i++ {
		row := v.Index(i)
		if row.Kind() != reflect.Slice {
			return nil
		}
		rr := make([]rune, row.Len())
		for j := 0; j < row.Len(); j++ {
			rr[j] = rune(row.Index(j).Int())
		}
		out[i] = rr
	}
	return out
}
