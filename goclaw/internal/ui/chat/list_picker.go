package chat

import "strings"

type listPickerItem struct {
	label    string
	selected bool
}

func renderListPicker(th *Theme, title string, items []listPickerItem, controls, footer string) string {
	if th == nil {
		th = DefaultTheme()
	}
	var b strings.Builder
	b.Grow(len(items)*64 + 256)
	b.WriteString(th.OverlayTitle.Render(title))
	b.WriteString("\n\n")
	for _, item := range items {
		if item.selected {
			b.WriteString(th.ShellChrome.Render("› ") + th.SlashPickerName.Render(item.label))
		} else {
			b.WriteString(th.ModalBody.Render("  " + item.label))
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(controls) != "" {
		b.WriteString("\n")
		b.WriteString(th.OverlayHint.Render(controls))
	}
	if strings.TrimSpace(footer) != "" {
		b.WriteString("\n")
		b.WriteString(th.OverlayHint.Render(footer))
	}
	return b.String()
}
