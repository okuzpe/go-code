package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/agentdemo"
	"github.com/okuzpe/goclaw/internal/agentdemo/ui"
	"github.com/okuzpe/goclaw/internal/config"
)

func main() {
	fileCfg := loadFileConfig()

	// Load ~/.goclaw/settings.json as lowest-priority defaults so agentdemo
	// respects the user's existing goclaw model/host configuration.
	goCfg, _ := config.Load(config.Default(), "")

	ollama := flag.String("ollama", getenv("OLLAMA_HOST", fileCfg["ollama"], pickFirst(goCfg.OllamaHost, "http://127.0.0.1:11434")), "Ollama base URL")
	model := flag.String("model", getenv("OLLAMA_MODEL", fileCfg["model"], pickFirst(goCfg.OllamaModel, "llama3.2")), "Ollama model name")
	workdir := flag.String("workdir", fileCfg["workdir"], "Workspace root for file tools (defaults to cwd)")
	unsafe := flag.Bool("unsafe", false, "Enable write_file, edit_file, and bash tools (use with caution)")
	flag.Parse()

	cfg := agentdemo.Config{
		OllamaHost: *ollama,
		Model:      *model,
		Workdir:    *workdir,
		Unsafe:     *unsafe,
	}
	if err := pingOllama(cfg.OllamaHost); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: cannot reach Ollama (%v). The TUI still runs; sends may fail.\n", err)
	}
	if err := ui.Run(context.Background(), cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func pingOllama(rawURL string) error {
	base := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if base == "" {
		base = "http://127.0.0.1:11434"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama: %s", resp.Status)
	}
	return nil
}

// pickFirst returns the first non-empty string.
func pickFirst(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// getenv returns the first non-empty value among: environment variable, config
// file value, and fallback default.
func getenv(envKey, fileValue, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if fileValue != "" {
		return fileValue
	}
	return fallback
}

// loadFileConfig reads ~/.agentdemo as a simple key=value file.
// Unknown keys are silently ignored. Returns an empty map on any error.
func loadFileConfig() map[string]string {
	result := make(map[string]string)
	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}
	file, err := os.Open(filepath.Join(home, ".agentdemo"))
	if err != nil {
		return result
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		result[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return result
}
