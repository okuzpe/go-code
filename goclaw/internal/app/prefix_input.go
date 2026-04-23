package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
)

// LocalPrefixToolResult captures a local ! / @ / & tool execution handled before the model runs.
type LocalPrefixToolResult struct {
	ToolName      string
	ToolInputJSON string
	Content       string
	Reply         string
	IsError       bool
}

// FormatPrefixToolReply formats local prefix tool output for session history and display.
func FormatPrefixToolReply(toolName, content string, isError bool) string {
	content = strings.TrimRight(content, "\n")
	if isError {
		return fmt.Sprintf("(%s tool error)\n%s", toolName, content)
	}
	return fmt.Sprintf("(%s)\n%s", toolName, content)
}

// RunLocalPrefixToolIfAny runs ! / @ / & locally when applicable. When mock is true
// it is a no-op and returns handled=false. Callers must not invoke this when routing
// input to a focused coordinator worker (deliver the raw line to the worker instead).
func RunLocalPrefixToolIfAny(
	ctx context.Context,
	mock bool,
	orch *orchestrator.Orchestrator,
	sess *session.Session,
	userText string,
	sink orchestrator.StreamSink,
) (*LocalPrefixToolResult, bool, error) {
	if mock {
		return nil, false, nil
	}
	analysis, err := inputprefix.Analyze(userText)
	if err != nil {
		return nil, true, err
	}
	if analysis.Kind != inputprefix.KindLocalTool {
		return nil, false, nil
	}
	content, isError, runErr := orch.RunToolInvocation(ctx, analysis.ToolName, analysis.ToolInputJSON, sink)
	if runErr != nil {
		return nil, true, runErr
	}
	body := FormatPrefixToolReply(analysis.ToolName, content, isError)
	result := &LocalPrefixToolResult{
		ToolName:      analysis.ToolName,
		ToolInputJSON: analysis.ToolInputJSON,
		Content:       content,
		Reply:         body,
		IsError:       isError,
	}
	if sess != nil {
		sess.Add("user", analysis.UserLine)
		sess.AddAssistant(body, nil)
	}
	if sink != nil {
		for _, line := range strings.Split(body, "\n") {
			sink.OnTextDelta(line + "\n")
		}
		sink.OnDone(body)
	}
	return result, true, nil
}
