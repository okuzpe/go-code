package hooks

import (
	"encoding/json"
)

type wirePayload struct {
	HookEvent   string `json:"hook_event_name"`
	ToolName    string `json:"tool_name,omitempty"`
	ToolInput   string `json:"tool_input,omitempty"`
	ToolOut     string `json:"tool_output,omitempty"`
	FailureKind string `json:"failure_kind,omitempty"`
}

func wireFromEvent(e Event) ([]byte, error) {
	p := wirePayload{
		HookEvent:   string(e.Type),
		ToolName:    e.ToolName,
		ToolInput:   e.Input,
		ToolOut:     e.Output,
		FailureKind: e.FailureKind,
	}
	return json.Marshal(p)
}
