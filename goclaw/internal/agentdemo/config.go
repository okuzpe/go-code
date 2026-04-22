package agentdemo

// Config holds runtime options for the agentdemo TUI.
type Config struct {
	OllamaHost string
	Model      string
	Workdir    string
	// Unsafe enables write_file, edit_file, and bash tools (off by default).
	Unsafe bool
}
