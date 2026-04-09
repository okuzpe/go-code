package orchestrator

import "github.com/okuzpe/goclaw/internal/llm"

const maxJSONToolResultRunes = 64_000

// JSONToolCall records one tool execution for automation JSON output.
type JSONToolCall struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Input   string `json:"input"`
	Result  string `json:"result"`
	IsError bool   `json:"isError"`
}

// JSONTurnResult is the stdout payload for goclaw --output-format json (and --json-output).
type JSONTurnResult struct {
	Response  string         `json:"response"`
	ToolCalls []JSONToolCall `json:"toolCalls"`
}

func truncateForJSONOutput(content string) string {
	runes := []rune(content)
	if len(runes) <= maxJSONToolResultRunes {
		return content
	}
	return string(runes[:maxJSONToolResultRunes]) + "… [truncated]"
}

func appendJSONToolTrace(trace *[]JSONToolCall, pending []llm.ToolUse, results []llm.ToolResultRecord) {
	if trace == nil {
		return
	}
	for i := range results {
		in := ""
		id := ""
		if i < len(pending) {
			in = pending[i].Input
			id = pending[i].ID
		}
		*trace = append(*trace, JSONToolCall{
			ID:      id,
			Name:    results[i].ToolName,
			Input:   in,
			Result:  truncateForJSONOutput(results[i].Content),
			IsError: results[i].IsError,
		})
	}
}
