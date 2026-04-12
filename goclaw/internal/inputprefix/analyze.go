// Package inputprefix parses Claude-style REPL prefixes (!, @, &) before text
// is sent to the language model. See docs/goclaw/prefix-input-modes.md.
package inputprefix

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Kind describes how the REPL should handle analyzed input.
type Kind int

const defaultSpawnAgentTimeoutSec = 120

const (
	// KindPassthrough sends the original input to the model unchanged.
	KindPassthrough Kind = iota
	// KindLocalTool runs one tool via orchestrator.RunToolInvocation.
	KindLocalTool
)

// Analysis is the outcome of Analyze on one user submission.
type Analysis struct {
	Kind Kind
	// Original is the trimmed full buffer (for passthrough and logging).
	Original string
	// UserLine is stored in the session for KindLocalTool (first line, trimmed).
	UserLine string
	// ToolName and ToolInputJSON are set when Kind == KindLocalTool.
	ToolName      string
	ToolInputJSON string
}

// BtwRewrite wraps body text after a /btw slash command for the model.
func BtwRewrite(rest string) string {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return rest
	}
	return "Side question — answer briefly; do not discard your main plan or context for the repo:\n\n" + rest
}

// Analyze classifies REPL input. Prefix modes use only the first line; if the
// buffer contains additional non-empty lines, it returns an error.
func Analyze(raw string) (Analysis, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Analysis{Kind: KindPassthrough, Original: trimmed}, nil
	}
	first, extra, err := splitFirstLine(trimmed)
	if err != nil {
		return Analysis{}, err
	}
	if extra != "" {
		switch {
		case strings.HasPrefix(first, "!"):
			return Analysis{}, fmt.Errorf("prefix ! must be a single line (extra text after newline)")
		case strings.HasPrefix(first, "&"):
			return Analysis{}, fmt.Errorf("prefix & must be a single line (extra text after newline)")
			// @ with extra lines falls through to KindPassthrough — inline expansion handles it.
		}
	}

	switch {
	case strings.HasPrefix(first, "!"):
		command := strings.TrimSpace(strings.TrimPrefix(first, "!"))
		if command == "" {
			return Analysis{}, fmt.Errorf("prefix ! requires a non-empty command")
		}
		payload, err := json.Marshal(map[string]string{"command": command})
		if err != nil {
			return Analysis{}, fmt.Errorf("prefix !: %w", err)
		}
		return Analysis{
			Kind:          KindLocalTool,
			Original:      trimmed,
			UserLine:      first,
			ToolName:      "bash",
			ToolInputJSON: string(payload),
		}, nil

	case strings.HasPrefix(first, "@"):
		// Multi-line @ messages (e.g. "@go.mod\nexplain this") fall through to
		// KindPassthrough so inline expansion can inject the file content alongside
		// the user's instruction. Only pure single-line @path is a local tool.
		if extra != "" {
			break
		}
		path := strings.TrimSpace(strings.TrimPrefix(first, "@"))
		if path == "" {
			return Analysis{}, fmt.Errorf("prefix @ requires a path")
		}
		payload, err := json.Marshal(map[string]string{"path": path})
		if err != nil {
			return Analysis{}, fmt.Errorf("prefix @: %w", err)
		}
		return Analysis{
			Kind:          KindLocalTool,
			Original:      trimmed,
			UserLine:      first,
			ToolName:      "read_file",
			ToolInputJSON: string(payload),
		}, nil

	case strings.HasPrefix(first, "&"):
		task := strings.TrimSpace(strings.TrimPrefix(first, "&"))
		if task == "" {
			return Analysis{}, fmt.Errorf("prefix & requires a task description")
		}
		type spawnPayload struct {
			Profile    string `json:"profile"`
			Task       string `json:"task"`
			TimeoutSec int    `json:"timeout_sec"`
		}
		payload, err := json.Marshal(spawnPayload{
			Profile:    "general-purpose",
			Task:       task,
			TimeoutSec: defaultSpawnAgentTimeoutSec,
		})
		if err != nil {
			return Analysis{}, fmt.Errorf("prefix &: %w", err)
		}
		return Analysis{
			Kind:          KindLocalTool,
			Original:      trimmed,
			UserLine:      first,
			ToolName:      "spawn_agent",
			ToolInputJSON: string(payload),
		}, nil
	}

	return Analysis{Kind: KindPassthrough, Original: trimmed}, nil
}

func splitFirstLine(s string) (firstLine string, extraTrimmed string, err error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	s = strings.ReplaceAll(s, "\r", "\n")
	idx := strings.IndexByte(s, '\n')
	if idx < 0 {
		return strings.TrimSpace(s), "", nil
	}
	firstLine = strings.TrimSpace(s[:idx])
	rest := strings.TrimSpace(s[idx+1:])
	return firstLine, rest, nil
}
