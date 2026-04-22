package ui

const (
	statusLines       = 2
	agentBarLines     = 2 // separator line + status line
	layoutGap         = 1
	minTranscriptRows = 4
	inputMaxHeight    = 8
	minComposeWidth   = 28
)

// TranscriptViewportHeight returns viewport height from terminal and chrome.
func TranscriptViewportHeight(termHeight, inputVisualLines int, logOverlay bool) int {
	if termHeight < 10 {
		termHeight = 10
	}
	used := statusLines + layoutGap + agentBarLines + layoutGap + inputVisualLines + layoutGap
	if logOverlay {
		used += 6
	}
	h := termHeight - used
	if h < minTranscriptRows {
		h = minTranscriptRows
	}
	return h
}
