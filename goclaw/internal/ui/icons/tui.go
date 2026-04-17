package icons

import "strings"

// Set selects which glyphs the fullscreen TUI uses for small chrome (workspace chip, etc.).
// Terminals differ: Nerd Font codepoints need a patched font; emoji needs an emoji-capable font.
type Set uint8

const (
	// Unicode uses a compact geometric workspace marker (single-cell, works without emoji fonts).
	Unicode Set = iota
	// ASCII uses only 7-bit punctuation (log-only or strict terminals).
	ASCII
	// Emoji uses a standard pictograph where fonts support it (default preset for the workspace chip).
	Emoji
	// Nerd uses Font Awesome "folder" from merged Nerd Fonts (U+F07C); requires a Nerd-patched font.
	Nerd
)

// CanonicalTUIIcons normalizes settings.json tui_icons / GOCLAW_TUI_ICONS to unicode|ascii|emoji|nerd.
// Empty or unknown values default to emoji (folder) for the workspace chip; use "unicode" for the legacy ▣ marker.
func CanonicalTUIIcons(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "emoji"
	}
	switch s {
	case "unicode", "block", "geometric":
		return "unicode"
	case "ascii", "plain", "off", "0", "no", "false":
		return "ascii"
	case "emoji":
		return "emoji"
	case "nerd", "nerdfont", "nf":
		return "nerd"
	default:
		return "emoji"
	}
}

// SetFromCanonical maps the stored config value (always canonical) to a Set.
func SetFromCanonical(canonical string) Set {
	switch strings.ToLower(strings.TrimSpace(canonical)) {
	case "ascii":
		return ASCII
	case "emoji":
		return Emoji
	case "nerd":
		return Nerd
	default:
		return Unicode
	}
}

// WorkspaceGlyph is the idle-footer workspace chip prefix (before the directory basename).
func (st Set) WorkspaceGlyph() string {
	switch st {
	case ASCII:
		return "*"
	case Emoji:
		return "\U0001f4c1" // 📁 file folder
	case Nerd:
		return "\uf07c" // nf-fa-folder in Nerd Fonts
	default:
		return "\u25a3" // ▣ white square containing small white square
	}
}

// AssistantBullet is the transcript gutter before assistant text (single-cell where possible).
func (st Set) AssistantBullet() string {
	switch st {
	case ASCII:
		return ">"
	case Nerd:
		return "\uf075" // nf-fa-comment
	default:
		return "\u25cf" // ● (unicode + emoji: avoid wide emoji in the gutter)
	}
}

// ToolOK and ToolErr are outcome markers on tool cards, tool log, and error lines.
func (st Set) ToolOK() string {
	switch st {
	case ASCII:
		return "+"
	case Emoji:
		return "\u2705" // ✅
	case Nerd:
		return "\uf00c" // nf-fa-check
	default:
		return "\u2713" // ✓
	}
}

func (st Set) ToolErr() string {
	switch st {
	case ASCII:
		return "x"
	case Emoji:
		return "\u274c" // ❌
	case Nerd:
		return "\uf00d" // nf-fa-times
	default:
		return "\u2717" // ✗
	}
}

// ToolCardTopLeft is the box-drawing prefix before the tool name on a card header.
func (st Set) ToolCardTopLeft() string {
	if st == ASCII {
		return "+-"
	}
	return "╭─"
}

// ToolCardH is the horizontal rule character for cards and short separators.
func (st Set) ToolCardH() string {
	if st == ASCII {
		return "-"
	}
	return "─"
}

// ToolCardBottomLeft closes the card footer (before the outcome icon).
func (st Set) ToolCardBottomLeft() string {
	if st == ASCII {
		return "+-"
	}
	return "╰─"
}

// ToolCardV is the vertical accent bar inside tool cards.
func (st Set) ToolCardV() string {
	if st == ASCII {
		return "|"
	}
	return "│"
}

// WelcomeTopPrefix is the left corner run before the title on the wide welcome frame.
func (st Set) WelcomeTopPrefix() string {
	if st == ASCII {
		return "+-- "
	}
	return "╭── "
}

// WelcomeTopRightCorner and WelcomeBottomCorners frame the wide welcome panel.
func (st Set) WelcomeTopRightCorner() string {
	if st == ASCII {
		return "+"
	}
	return "╮"
}

func (st Set) WelcomeBottomLeftCorner() string {
	if st == ASCII {
		return "+"
	}
	return "╰"
}

func (st Set) WelcomeBottomRightCorner() string {
	if st == ASCII {
		return "+"
	}
	return "╯"
}

// ApprovalPromptGlyph prefixes the inline tool-approval strip (single width; avoids hardcoded emoji in chat).
func (st Set) ApprovalPromptGlyph() string {
	switch st {
	case ASCII:
		return "!"
	case Emoji:
		return "\u26a1" // high voltage (common single-cell lightning)
	case Nerd:
		return "\uf0e7" // nf-fa-bolt
	default:
		return "\u26a1"
	}
}

// DoctorBadge prefixes the fullscreen doctor report title (single cell where possible).
func (st Set) DoctorBadge() string {
	switch st {
	case ASCII:
		return "i "
	case Emoji:
		return "\u2022 " // bullet + space (narrow; avoids wide pictographs in the header)
	case Nerd:
		return "\uf0f9 " // nf-fa-stethoscope
	default:
		return "\u2023 " // ‣ triangular bullet + space
	}
}
