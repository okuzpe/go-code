package chat

// English footer copy for transcript scrolling. A future UI locale catalog would replace this
// (see docs/goclaw/i18n.md — static TUI strings stay English until then).

const transcriptScrollNavCore = "Transcript: Ctrl+B browse · PgUp/PgDn · Alt+arrows"

func transcriptWheelSuffix(tuiMouseScroll bool) string {
	if tuiMouseScroll {
		return " · wheel on transcript or compose"
	}
	return ""
}

// transcriptScrollNavFooterLine is shown after long assistant replies when idle.
func transcriptScrollNavFooterLine(tuiMouseScroll bool) string {
	return transcriptScrollNavCore + transcriptWheelSuffix(tuiMouseScroll)
}

// streamBusyTranscriptScrollFooterLine is shown while the model streams or shows the thinking spinner.
// termNarrow follows composePlaceholderNarrowMaxW from the caller.
func streamBusyTranscriptScrollFooterLine(tuiMouseScroll, emptyCompose, termNarrow bool) string {
	w := transcriptWheelSuffix(tuiMouseScroll)
	if emptyCompose {
		if termNarrow {
			return "Transcript: ↑↓ j/k · PgUp · Alt+arrows · Ctrl+B" + w
		}
		return "Transcript: empty compose — ↑↓ j/k · Ctrl+B browse · PgUp/PgDn · Alt+arrows" + w
	}
	if termNarrow {
		return "Transcript: PgUp · Alt+arrows · Ctrl+B" + w
	}
	return transcriptScrollNavFooterLine(tuiMouseScroll)
}

// transcriptBrowseFooterLine is the dim footer while Ctrl+B transcript browse mode is active.
func transcriptBrowseFooterLine(tuiMouseScroll bool) string {
	return "Browse: ↑↓ j/k PgUp · [ ] tool · e expand · Ctrl+B editor · Esc back" + transcriptWheelSuffix(tuiMouseScroll)
}
