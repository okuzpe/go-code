package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// terminalSink streams LLM text and tool lifecycle events to the terminal in readline mode.
// Text deltas are printed directly to stdout; tool events go to stderr so they don't
// mix with the streamed response.
type terminalSink struct {
	needsNL bool
}

var _ orchestrator.StreamSink = (*terminalSink)(nil)

func (s *terminalSink) OnTextDelta(text string) {
	fmt.Print(text)
	if len(text) > 0 {
		s.needsNL = text[len(text)-1] != '\n'
	}
}

func (s *terminalSink) OnToolUse(name, preview string) {
	if s.needsNL {
		fmt.Println()
		s.needsNL = false
	}
	line := orchestrator.ToolWorkingPhrase(name)
	if p := strings.TrimSpace(preview); p != "" {
		fmt.Fprintf(os.Stderr, "\n· %s — %s\n", line, p)
	} else {
		fmt.Fprintf(os.Stderr, "\n· %s…\n", line)
	}
}

func (s *terminalSink) OnToolResult(name string, _ int, isError bool) {
	phrase := orchestrator.ToolFinishedPhrase(name)
	if isError {
		fmt.Fprintf(os.Stderr, "  ✗ %s\n", phrase)
	} else {
		fmt.Fprintf(os.Stderr, "  ✓ %s\n", phrase)
	}
}

func (s *terminalSink) OnDone(_ string) {
	if s.needsNL {
		fmt.Println()
		s.needsNL = false
	}
}
