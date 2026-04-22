package ui

// streamDeltaMsg carries batched assistant text from the Ollama goroutine.
type streamDeltaMsg struct {
	text string
}

// streamDoneMsg ends the LLM stream (error optional).
type streamDoneMsg struct {
	err error
}

// agentThinkingMsg is posted when the orchestrator starts a new LLM call.
type agentThinkingMsg struct {
	phase string
}

// agentToolStartMsg is posted when the orchestrator calls a tool.
type agentToolStartMsg struct {
	name string
}

// agentToolDoneMsg is posted when a tool finishes execution.
type agentToolDoneMsg struct {
	name    string
	content string
	isError bool
}
