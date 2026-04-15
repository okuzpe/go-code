package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/text"
)

const transcriptOutcomeMaxRunes = 100

// toolCardListSampleLines caps how many result lines (paths / grep hits) appear on the TUI tool card.
const toolCardListSampleLines = 12

// toolCardListLineMaxRunes limits each path line width inside the card (terminal-friendly).
const toolCardListLineMaxRunes = 96

// TranscriptOutcomeSnippet returns a short second line for TUI tool cards: what the
// tool produced (counts, first error line, tail of shell output), without dumping
// large file bodies into the transcript.
func TranscriptOutcomeSnippet(toolName, content string, isError bool) string {
	content = strings.TrimSpace(content)
	if content == "" {
		if isError {
			return "error (empty message)"
		}
		return ""
	}
	if isError {
		return text.TruncateRunes(firstLineOf(content), transcriptOutcomeMaxRunes)
	}

	if strings.HasPrefix(toolName, "mcp__") {
		return mcpTranscriptOutcomeSnippet(content)
	}

	switch toolName {
	case "spawn_agent":
		return spawnAgentTranscriptOutcomeSnippet(content)
	case "glob", "grep":
		main, hadCap := toolListMainBody(content)
		n := countNonEmptyLinesIn(main)
		if main == "(no matches)" {
			return "(no matches)"
		}
		if n == 0 {
			return ""
		}
		noun := "paths"
		if toolName == "grep" {
			noun = "matches"
		}
		if hadCap {
			return fmt.Sprintf("%d+ %s (truncated)", n, noun)
		}
		return fmt.Sprintf("%d %s", n, noun)
	case "read_file":
		lines := strings.Count(content, "\n")
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			lines++
		}
		chars := utf8.RuneCountInString(content)
		if lines <= 1 && chars <= 120 {
			return text.TruncateRunes(strings.ReplaceAll(content, "\n", " "), transcriptOutcomeMaxRunes)
		}
		return fmt.Sprintf("%d lines · %d chars", lines, chars)
	case "write_file", "edit_file", "patch", "todo_write", "web_search", "web_fetch":
		return text.TruncateRunes(firstLineOf(content), transcriptOutcomeMaxRunes)
	case "bash", "script":
		if tail := lastNonEmptyLine(content); tail != "" {
			return text.TruncateRunes(tail, transcriptOutcomeMaxRunes)
		}
		return "(no output)"
	default:
		if strings.Count(content, "\n") >= 4 {
			return fmt.Sprintf("%d lines", countNonEmptyLinesIn(content))
		}
		return text.TruncateRunes(firstLineOf(content), transcriptOutcomeMaxRunes)
	}
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}

func countNonEmptyLinesIn(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// nonEmptyLinesFromMain returns trimmed non-empty lines from glob/grep list output.
func nonEmptyLinesFromMain(main string) []string {
	var out []string
	for _, line := range strings.Split(main, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ToolCardSummaryBody builds the multi-line summary shown inside a completed tool card.
// For glob/grep it shows the tool input preview (pattern) plus a sample of result lines so the
// transcript reads like a sequential trace (similar to other agent CLIs) instead of only a count.
func ToolCardSummaryBody(toolName, inputPreview, content string, isError bool) string {
	inputPreview = strings.TrimSpace(inputPreview)
	if isError {
		if inputPreview != "" {
			return text.TruncateRunes(inputPreview, toolCardListLineMaxRunes)
		}
		return text.TruncateRunes(strings.TrimSpace(content), toolCardListLineMaxRunes)
	}
	switch toolName {
	case "glob", "grep":
		return buildListToolCardSummary(inputPreview, content)
	default:
		if inputPreview == "" {
			return ""
		}
		return text.TruncateRunes(inputPreview, toolCardListLineMaxRunes)
	}
}

func buildListToolCardSummary(inputPreview, content string) string {
	main, capped := toolListMainBody(content)
	lines := nonEmptyLinesFromMain(main)
	var b strings.Builder
	if inputPreview != "" {
		b.WriteString(text.TruncateRunes(inputPreview, toolCardListLineMaxRunes))
	}
	if len(lines) == 0 {
		if strings.TrimSpace(main) != "" && b.Len() == 0 {
			return text.TruncateRunes(strings.TrimSpace(main), toolCardListLineMaxRunes)
		}
		if b.Len() == 0 {
			return ""
		}
		return b.String()
	}
	shown := 0
	for _, line := range lines {
		if shown >= toolCardListSampleLines {
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		lineDisp := text.AtRefDisplayMaybePathLine(line)
		b.WriteString(text.TruncateRunes(lineDisp, toolCardListLineMaxRunes))
		shown++
	}
	if len(lines) > shown {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "… +%d more", len(lines)-shown)
	}
	if capped {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("(hit tool match cap)")
	}
	return b.String()
}

// toolListMainBody splits glob/grep style output from an optional trailing cap notice.
func toolListMainBody(content string) (main string, capped bool) {
	content = strings.TrimSpace(content)
	idx := strings.Index(content, "\n\n[")
	if idx < 0 {
		return content, strings.Contains(content, "[truncated")
	}
	main = strings.TrimSpace(content[:idx])
	suffix := content[idx:]
	return main, strings.Contains(suffix, "[truncated")
}

// spawnAgentTranscriptOutcomeSnippet parses WorkerNotification JSON from spawn_agent results.
func spawnAgentTranscriptOutcomeSnippet(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	var w struct {
		TaskID  string `json:"task_id"`
		Profile string `json:"profile"`
		Status  string `json:"status"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(content), &w); err != nil {
		return text.TruncateRunes(firstLineOf(content), transcriptOutcomeMaxRunes)
	}
	var parts []string
	if p := strings.TrimSpace(w.Profile); p != "" {
		parts = append(parts, "profile "+p)
	}
	if s := strings.TrimSpace(w.Status); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(w.Summary); s != "" {
		parts = append(parts, text.TruncateRunes(s, 72))
	} else if id := strings.TrimSpace(w.TaskID); id != "" {
		if len(id) > 12 {
			parts = append(parts, "task "+id[:8]+"…")
		} else {
			parts = append(parts, "task "+id)
		}
	}
	if len(parts) == 0 {
		return text.TruncateRunes(firstLineOf(content), transcriptOutcomeMaxRunes)
	}
	return text.TruncateRunes(strings.Join(parts, " · "), transcriptOutcomeMaxRunes)
}

func mcpTranscriptOutcomeSnippet(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(content), &raw) != nil {
		return text.TruncateRunes(firstLineOf(content), transcriptOutcomeMaxRunes)
	}
	for _, key := range []string{"result", "content", "text", "message", "output"} {
		if v, ok := raw[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil && strings.TrimSpace(s) != "" {
				return text.TruncateRunes(strings.TrimSpace(s), transcriptOutcomeMaxRunes)
			}
		}
	}
	return text.TruncateRunes(firstLineOf(content), transcriptOutcomeMaxRunes)
}
