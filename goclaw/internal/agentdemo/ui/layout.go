package ui

const (
	statusLines       = 2
	layoutGap         = 1
	minTranscriptRows = 4
)

// TranscriptViewportHeight returns viewport height from terminal and chrome.
func TranscriptViewportHeight(termHeight, inputVisualLines int, logOverlay bool) int {
	if termHeight < 10 {
		termHeight = 10
	}
	used := statusLines + layoutGap + inputVisualLines + layoutGap
	if logOverlay {
		used += 6
	}
	h := termHeight - used
	if h < minTranscriptRows {
		h = minTranscriptRows
	}
	return h
}
