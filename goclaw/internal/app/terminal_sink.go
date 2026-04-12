package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/okuzpe/goclaw/internal/orchestrator"
)

const terminalToolOutputCap = 2 * 1024 // 2 KiB display cap in readline mode

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

func (s *terminalSink) OnToolUse(name, rawInput string) {
	if s.needsNL {
		fmt.Println()
		s.needsNL = false
	}
	label := capitalizeToolName(name)
	desc := toolDescription(name, rawInput)
	inText := toolINText(name, rawInput)

	if desc != "" {
		fmt.Fprintf(os.Stderr, "\n%s %s\n", label, desc)
	} else {
		fmt.Fprintf(os.Stderr, "\n%s\n", label)
	}
	if inText != "" {
		fmt.Fprintf(os.Stderr, "IN\n%s\n", inText)
	}
}

func (s *terminalSink) OnToolResult(_ string, content string, isError bool) {
	icon := "✓"
	if isError {
		icon = "✗"
	}
	displayed := strings.TrimSpace(content)
	truncated := false
	if len(displayed) > terminalToolOutputCap {
		displayed = displayed[:terminalToolOutputCap]
		truncated = true
	}
	if displayed != "" {
		fmt.Fprintf(os.Stderr, "\nOUT\n%s", displayed)
		if truncated {
			fmt.Fprintf(os.Stderr, "\n… (output truncated)")
		}
		fmt.Fprintf(os.Stderr, "\n%s\n", icon)
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", icon)
	}
	s.needsNL = false
}

func (s *terminalSink) OnDone(_ string) {
	if s.needsNL {
		fmt.Println()
		s.needsNL = false
	}
}

// capitalizeToolName converts "read_file" → "Read file", "bash" → "Bash".
func capitalizeToolName(name string) string {
	s := strings.ReplaceAll(name, "_", " ")
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// toolDescription returns the LLM-supplied description (bash) or the working phrase for other tools.
func toolDescription(name, rawInput string) string {
	if name == "bash" {
		var m map[string]json.RawMessage
		if json.Unmarshal([]byte(rawInput), &m) == nil {
			if v, ok := m["description"]; ok {
				var desc string
				if json.Unmarshal(v, &desc) == nil && desc != "" {
					return desc
				}
			}
		}
	}
	return orchestrator.ToolWorkingPhrase(name)
}

// toolINText returns the human-readable input text for the IN section.
func toolINText(name, rawInput string) string {
	return orchestrator.FormatToolUsePreview(name, rawInput)
}

// completedTool is one entry in the readline REPL tool history.
type completedTool struct {
	name    string
	summary string // input preview
	content string // full result
	isError bool
	elapsed time.Duration
}

// loggingTerminalSink wraps terminalSink and accumulates a tool history log.
// Used by the readline REPL to power the /tools command.
type loggingTerminalSink struct {
	terminalSink
	mu          sync.Mutex
	log         []completedTool
	currentName string
	currentSumm string
	currentStart time.Time
}

func (s *loggingTerminalSink) OnToolUse(name, preview string) {
	s.terminalSink.OnToolUse(name, preview)
	s.mu.Lock()
	s.currentName = name
	s.currentSumm = preview
	s.currentStart = time.Now()
	s.mu.Unlock()
}

func (s *loggingTerminalSink) OnToolResult(name string, content string, isError bool) {
	s.terminalSink.OnToolResult(name, content, isError)
	s.mu.Lock()
	elapsed := time.Duration(0)
	if !s.currentStart.IsZero() {
		elapsed = time.Since(s.currentStart)
	}
	s.log = append(s.log, completedTool{
		name:    name,
		summary: s.currentSumm,
		content: content,
		isError: isError,
		elapsed: elapsed,
	})
	s.currentName = ""
	s.currentSumm = ""
	s.currentStart = time.Time{}
	s.mu.Unlock()
}

// FormatToolLog returns a human-readable summary of the tool history.
// If n > 0, it returns the full content of the nth entry (1-based).
func (s *loggingTerminalSink) FormatToolLog(n int) string {
	s.mu.Lock()
	entries := append([]completedTool(nil), s.log...)
	s.mu.Unlock()

	if len(entries) == 0 {
		return "(no tool calls this session)"
	}

	// Detail view: show full content of entry n (1-based).
	if n > 0 && n <= len(entries) {
		entry := entries[n-1]
		icon := "✓"
		if entry.isError {
			icon = "✗"
		}
		var b strings.Builder
		fmt.Fprintf(&b, "[%s] %s\n", icon, orchestrator.ToolFinishedPhrase(entry.name))
		if entry.summary != "" {
			fmt.Fprintf(&b, "Input: %s\n", entry.summary)
		}
		fmt.Fprintf(&b, "Elapsed: %.2fs  Size: %d bytes\n\n", entry.elapsed.Seconds(), len(entry.content))
		b.WriteString(entry.content)
		return b.String()
	}

	// List view.
	var b strings.Builder
	fmt.Fprintf(&b, "Tool history (%d steps):\n", len(entries))
	for i, entry := range entries {
		icon := "✓"
		if entry.isError {
			icon = "✗"
		}
		label := orchestrator.ToolFinishedPhrase(entry.name)
		summ := entry.summary
		if len(summ) > 60 {
			summ = summ[:60] + "…"
		}
		fmt.Fprintf(&b, "  %d. [%s] %s", i+1, icon, label)
		if summ != "" {
			fmt.Fprintf(&b, " — %s", summ)
		}
		fmt.Fprintf(&b, " (%d bytes, %.1fs)\n", len(entry.content), entry.elapsed.Seconds())
	}
	b.WriteString("\nRun /tools N to see full output of step N.")
	return b.String()
}
