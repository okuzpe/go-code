package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// AugmentOrchestratorErr appends user-facing recovery hints when the wrapped error is recognized.
// model is the active model id (e.g. for ollama pull hints). errors.Is / errors.As still work on the returned value.
func AugmentOrchestratorErr(model string, err error) error {
	if err == nil {
		return nil
	}
	hints := orchestratorFailureHints(model, err)
	if len(hints) == 0 {
		return err
	}
	return fmt.Errorf("%w\n%s", err, strings.Join(hints, "\n"))
}

func orchestratorFailureHints(model string, err error) []string {
	msg := err.Error()
	low := strings.ToLower(msg)
	out := make([]string, 0, 4)

	if strings.Contains(low, "cannot reach ollama") || strings.Contains(low, "connection refused") {
		out = append(out, "hint: start Ollama (desktop app or `ollama serve`) and ensure OLLAMA_HOST matches (default http://localhost:11434).")
	}
	if strings.Contains(low, "no such host") {
		out = append(out, "hint: check OLLAMA_HOST for typos, VPN, or DNS issues.")
	}
	if strings.Contains(low, "ollama") && strings.Contains(msg, "404") {
		m := strings.TrimSpace(model)
		if m == "" {
			m = "<model>"
		}
		out = append(out, "hint: if the model is missing, run: ollama pull "+m)
	} else if strings.Contains(low, "ollama") && (strings.Contains(low, "file does not exist") ||
		strings.Contains(low, "model") && (strings.Contains(low, "not found") || strings.Contains(low, "unknown model"))) {
		m := strings.TrimSpace(model)
		if m == "" {
			m = "<model>"
		}
		out = append(out, "hint: pull the model into the local Ollama library: ollama pull "+m)
	}

	if strings.Contains(low, "iteration limit") {
		out = append(out, "hint: context loop hit max steps for one message — try /compact, /new, or a narrower request.")
	}
	if strings.Contains(low, "tool call limit") {
		out = append(out, "hint: too many tool calls in one turn — ask for one step at a time or simplify the task.")
	}
	if errorsIsActionStalled(err) {
		out = append(out,
			"hint: the model failed to take real tool-driven action for a coding request.",
			"hint: try a stronger coding model, narrow the request, or check /doctor for tool-calling degradation.",
		)
		if strings.Contains(low, "fence-only") || strings.Contains(low, "placeholder response") {
			out = append(out, "hint: the model emitted placeholder markdown instead of usable tool calls; retry in /mode build, or switch to a stronger tools-capable local model.")
		}
		if strings.Contains(low, "path recovery exhausted") || strings.Contains(low, "path recovery was still pending") {
			out = append(out, "hint: the model kept guessing repo paths; retry with a narrower request or rediscover the target first with glob/read_file in /mode build.")
		}
	}
	if strings.Contains(low, "no approver configured") || strings.Contains(low, "requires user approval") {
		out = append(out, "hint: this mode needs interactive tool approval — use the fullscreen TUI on a real terminal (not a pipe-only workflow).")
	}

	return dedupeHintLines(out)
}

func errorsIsActionStalled(err error) bool {
	return err != nil && (errors.Is(err, orchestrator.ErrActionStalled) ||
		strings.Contains(strings.ToLower(err.Error()), strings.ToLower(orchestrator.ErrActionStalled.Error())))
}

func dedupeHintLines(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
