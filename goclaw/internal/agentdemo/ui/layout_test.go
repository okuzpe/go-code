package ui

import "testing"

func TestTranscriptViewportHeight(t *testing.T) {
	t.Parallel()
	if g := TranscriptViewportHeight(24, 3, false); g < minTranscriptRows {
		t.Fatalf("got %d", g)
	}
	if g := TranscriptViewportHeight(24, 3, true); g < 1 {
		t.Fatalf("overlay got %d", g)
	}
}
