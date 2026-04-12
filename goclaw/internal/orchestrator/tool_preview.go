package orchestrator

import (
	"encoding/json"
	"strconv"
	"strings"
)

const (
	toolUsePreviewMaxRunes        = 160
	toolUsePreviewSpawnAgentRunes = 280
)

// FormatToolUsePreview turns tool JSON input into a short human-readable summary
// for terminals and approval prompts (no raw JSON).
func FormatToolUsePreview(toolName, input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	if s := formatKnownToolPreview(toolName, input); s != "" {
		max := toolUsePreviewMaxRunes
		if toolName == "spawn_agent" {
			max = toolUsePreviewSpawnAgentRunes
		}
		return truncatePreviewRunes(s, max)
	}
	return truncatePreviewRunes(genericJSONSummary(input), toolUsePreviewMaxRunes)
}

func formatKnownToolPreview(toolName, input string) string {
	switch toolName {
	case "web_search":
		var v struct {
			Query string `json:"query"`
		}
		if json.Unmarshal([]byte(input), &v) == nil {
			return strings.TrimSpace(v.Query)
		}
	case "web_fetch":
		var v struct {
			URL string `json:"url"`
		}
		if json.Unmarshal([]byte(input), &v) == nil {
			return strings.TrimSpace(v.URL)
		}
	case "read_file", "write_file", "edit_file", "patch":
		var v struct {
			Path string `json:"path"`
		}
		if json.Unmarshal([]byte(input), &v) == nil {
			return strings.TrimSpace(v.Path)
		}
	case "glob":
		var v struct {
			Pattern string `json:"pattern"`
		}
		if json.Unmarshal([]byte(input), &v) == nil {
			return strings.TrimSpace(v.Pattern)
		}
	case "grep":
		var v struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if json.Unmarshal([]byte(input), &v) == nil {
			p := strings.TrimSpace(v.Pattern)
			if sub := strings.TrimSpace(v.Path); sub != "" {
				return p + " in " + sub
			}
			return p
		}
	case "bash":
		var v struct {
			Command     string `json:"command"`
			Cwd         string `json:"cwd"`
			Description string `json:"description"`
		}
		if json.Unmarshal([]byte(input), &v) == nil {
			if desc := strings.TrimSpace(v.Description); desc != "" {
				return desc
			}
			cmd := strings.TrimSpace(v.Command)
			if cwd := strings.TrimSpace(v.Cwd); cwd != "" {
				return cmd + " (cwd " + cwd + ")"
			}
			return cmd
		}
	case "script":
		var v struct {
			Script string `json:"script"`
			Cwd    string `json:"cwd"`
		}
		if json.Unmarshal([]byte(input), &v) == nil {
			// Show first non-empty line of the script as the preview.
			first := firstNonEmptyLine(v.Script)
			if cwd := strings.TrimSpace(v.Cwd); cwd != "" {
				return first + " (cwd " + cwd + ")"
			}
			return first
		}
	case "todo_write":
		var v struct {
			Merge *bool             `json:"merge"`
			Todos []json.RawMessage `json:"todos"`
		}
		if json.Unmarshal([]byte(input), &v) == nil {
			n := len(v.Todos)
			if v.Merge != nil && *v.Merge {
				return "merge · " + strconv.Itoa(n) + " task(s)"
			}
			return "replace · " + strconv.Itoa(n) + " task(s)"
		}
	case "spawn_agent":
		var v struct {
			Profile     string `json:"profile"`
			Task        string `json:"task"`
			Interactive bool   `json:"interactive"`
			TimeoutSec  int    `json:"timeout_sec"`
		}
		if json.Unmarshal([]byte(input), &v) != nil {
			return ""
		}
		prof := strings.TrimSpace(v.Profile)
		if prof == "" {
			prof = "(profile?)"
		}
		mode := "one-shot worker"
		if v.Interactive {
			mode = "interactive worker (then /focus <id>)"
		}
		task := strings.TrimSpace(v.Task)
		task = strings.Join(strings.Fields(task), " ")
		if task == "" {
			task = "(no task text)"
		}
		suffix := ""
		if v.TimeoutSec > 0 {
			suffix = " · timeout " + strconv.Itoa(v.TimeoutSec) + "s"
		}
		return prof + " · " + mode + " · " + task + suffix
	case "stop_task":
		var v struct {
			TaskID string `json:"task_id"`
		}
		if json.Unmarshal([]byte(input), &v) == nil {
			id := strings.TrimSpace(v.TaskID)
			if id == "" {
				return "(missing task_id)"
			}
			return "cancel worker " + id
		}
	}
	return ""
}

func genericJSONSummary(raw string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		s := strings.TrimSpace(raw)
		s = strings.TrimPrefix(s, "{")
		s = strings.TrimSuffix(s, "}")
		return strings.TrimSpace(s)
	}
	for _, key := range []string{"query", "path", "url", "command", "pattern", "target_file", "file_path"} {
		if v, ok := m[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return "…"
}

func truncatePreviewRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

// ToolWorkingPhrase is a short present-participle label for status UI while a tool runs.
func ToolWorkingPhrase(toolName string) string {
	switch toolName {
	case "web_search":
		return "Searching the web"
	case "web_fetch":
		return "Fetching a page"
	case "read_file":
		return "Reading a file"
	case "write_file":
		return "Writing a file"
	case "edit_file":
		return "Editing a file"
	case "patch":
		return "Applying a patch"
	case "glob":
		return "Finding paths"
	case "grep":
		return "Searching in files"
	case "bash":
		return "Running a command"
	case "script":
		return "Running shell script"
	case "todo_write":
		return "Updating tasks"
	case "spawn_agent":
		return "Spawning sub-agent"
	case "stop_task":
		return "Stopping worker"
	default:
		if strings.HasPrefix(toolName, "mcp__") {
			return "Running MCP tool"
		}
		return humanizeToolName(toolName)
	}
}

// ToolFinishedPhrase is a short past-tense label for a completed tool line.
func ToolFinishedPhrase(toolName string) string {
	switch toolName {
	case "web_search":
		return "Searched the web"
	case "web_fetch":
		return "Fetched page"
	case "read_file":
		return "Read file"
	case "write_file":
		return "Wrote file"
	case "edit_file":
		return "Edited file"
	case "patch":
		return "Applied patch"
	case "glob":
		return "Matched paths"
	case "grep":
		return "Searched files"
	case "bash":
		return "Ran command"
	case "script":
		return "Ran shell script"
	case "todo_write":
		return "Updated tasks"
	case "spawn_agent":
		return "Spawned sub-agent"
	case "stop_task":
		return "Stopped worker"
	default:
		if strings.HasPrefix(toolName, "mcp__") {
			return "Ran MCP tool"
		}
		return humanizeToolName(toolName)
	}
}

func humanizeToolName(toolName string) string {
	return strings.ReplaceAll(toolName, "_", " ")
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return strings.TrimSpace(s)
}
