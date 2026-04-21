package ui

// streamDeltaMsg carries batched assistant text from the Ollama goroutine.
type streamDeltaMsg struct {
	text string
}

// streamDoneMsg ends the LLM stream (error optional).
type streamDoneMsg struct {
	err error
}

// demoToolDoneMsg completes the stub tool animation.
type demoToolDoneMsg struct {
	summary string
}
