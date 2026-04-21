package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/okuzpe/goclaw/internal/agentdemo"
	"github.com/okuzpe/goclaw/internal/agentdemo/llm"
	"github.com/okuzpe/goclaw/internal/agentdemo/ui"
)

func main() {
	ollama := flag.String("ollama", getenv("OLLAMA_HOST", "http://127.0.0.1:11434"), "Ollama base URL")
	model := flag.String("model", getenv("OLLAMA_MODEL", "llama3.2"), "Ollama model name")
	workdir := flag.String("workdir", "", "Optional workspace label (display only for this demo)")
	flag.Parse()

	_, _ = fmt.Fprintf(os.Stderr, "note: agentdemo is a minimal Ollama+TUI smoke build. For tools + agent loop use: goclaw\n")

	cfg := agentdemo.Config{
		OllamaHost: *ollama,
		Model:      *model,
		Workdir:    *workdir,
	}
	if err := llm.PingOllama(context.Background(), cfg.OllamaHost); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: cannot reach Ollama (%v). The TUI still runs; sends may fail.\n", err)
	}
	if err := ui.Run(context.Background(), cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
