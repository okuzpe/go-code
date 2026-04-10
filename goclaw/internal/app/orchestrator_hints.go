package app

import (
	"fmt"
	"strings"
)

// AugmentOrchestratorErr appends user-facing recovery hints when the wrapped error is recognized.
// errors.Is / errors.As still work on the returned value.
func AugmentOrchestratorErr(provider, model string, err error) error {
	if err == nil {
		return nil
	}
	hints := orchestratorFailureHints(provider, model, err)
	if len(hints) == 0 {
		return err
	}
	return fmt.Errorf("%w\n%s", err, strings.Join(hints, "\n"))
}

func orchestratorFailureHints(provider, model string, err error) []string {
	msg := err.Error()
	low := strings.ToLower(msg)
	var out []string

	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		if strings.Contains(msg, "401") || strings.Contains(low, "authentication") || strings.Contains(low, "invalid x-api-key") {
			out = append(out, "hint: set ANTHROPIC_API_KEY or fix the key in ~/.goclaw/settings.json (or project .goclaw/settings.json).")
		}
		if strings.Contains(msg, "429") || strings.Contains(low, "rate_limit") || strings.Contains(low, "rate limit") {
			out = append(out, "hint: rate limited — wait and retry, or switch model/provider in settings.")
		}
		if strings.Contains(msg, "529") || strings.Contains(low, "overloaded") {
			out = append(out, "hint: API temporarily overloaded — retry shortly or use provider=ollama with a local model.")
		}
	case "openai_compatible":
		if strings.Contains(msg, "401") || (strings.Contains(low, "invalid") && strings.Contains(low, "key")) {
			out = append(out, "hint: set OPENAI_API_KEY or \"openai_api_key\" in settings; confirm the key matches the chosen base URL.")
		}
		if strings.Contains(msg, "429") || strings.Contains(low, "rate") {
			out = append(out, "hint: rate limited — wait and retry, pick another free model, or use provider=ollama locally.")
		}
	default:
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
		}
	}

	if strings.Contains(low, "iteration limit") {
		out = append(out, "hint: context loop hit max steps for one message — try /compact, /new, or a narrower request.")
	}
	if strings.Contains(low, "tool call limit") {
		out = append(out, "hint: too many tool calls in one turn — ask for one step at a time or simplify the task.")
	}
	if strings.Contains(low, "no approver configured") || strings.Contains(low, "requires user approval") {
		out = append(out, "hint: this mode needs interactive tool approval — use the readline or TUI REPL (not a pipe-only workflow).")
	}

	return dedupeHintLines(out)
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
