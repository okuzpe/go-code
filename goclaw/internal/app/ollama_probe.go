package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/config"
)

const ollamaStartupProbeTimeout = 2 * time.Second

// OllamaStartupProbe summarizes a lightweight GET /api/tags check at chat startup.
type OllamaStartupProbe struct {
	Host            string
	ConfiguredModel string
	Reachable       bool
	HTTPStatus      int
	Err             error // transport or context failure before a response status is known
	// ModelInLibrary is true when /api/tags lists a model whose name exactly matches ConfiguredModel.
	ModelInLibrary bool
	// ModelNames is the full list of model names returned by /api/tags (empty when not reachable).
	// Used by /doctor to verify compaction_model and task_models entries.
	ModelNames []string
}

// ollamaTagsProbeURL returns the GET /api/tags URL for the given host (empty host → default localhost).
func ollamaTagsProbeURL(host string) string {
	base := effectiveOllamaHost(host)
	return strings.TrimRight(base, "/") + "/api/tags"
}

// ProbeOllamaStartup runs GET /api/tags with the standard chat startup timeout.
func ProbeOllamaStartup(cfg config.Config) OllamaStartupProbe {
	return probeOllamaStartup(cfg, ollamaStartupProbeTimeout)
}

func probeOllamaStartup(cfg config.Config, timeout time.Duration) OllamaStartupProbe {
	out := OllamaStartupProbe{
		Host:            effectiveOllamaHost(cfg.OllamaHost),
		ConfiguredModel: strings.TrimSpace(cfg.Model()),
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Provider), "ollama") {
		out.Reachable = true
		out.ModelInLibrary = true
		return out
	}
	if out.ConfiguredModel == "" {
		out.ConfiguredModel = strings.TrimSpace(cfg.OllamaModel)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	u := ollamaTagsProbeURL(cfg.OllamaHost)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		out.Err = err
		return out
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		out.Err = err
		return out
	}
	defer func() { _ = resp.Body.Close() }()
	out.HTTPStatus = resp.StatusCode
	out.Reachable = resp.StatusCode >= 200 && resp.StatusCode < 300
	if !out.Reachable {
		return out
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		out.Err = err
		out.Reachable = false
		return out
	}
	names, err := parseOllamaTagsModelNames(body)
	if err != nil {
		out.Err = err
		return out
	}
	out.ModelInLibrary = ollamaModelNameListed(out.ConfiguredModel, names)
	out.ModelNames = names
	return out
}

type ollamaTagsWire struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func parseOllamaTagsModelNames(body []byte) ([]string, error) {
	var w ollamaTagsWire
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("parse /api/tags: %w", err)
	}
	out := make([]string, 0, len(w.Models))
	for _, m := range w.Models {
		if n := strings.TrimSpace(m.Name); n != "" {
			out = append(out, n)
		}
	}
	return out, nil
}

func ollamaModelNameListed(want string, listed []string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, name := range listed {
		if name == want {
			return true
		}
	}
	return false
}

// FormatOllamaWelcomeWarning returns a single-line English notice for the TUI welcome panel, or empty.
func FormatOllamaWelcomeWarning(rt *ChatRuntime) string {
	if rt == nil || !strings.EqualFold(strings.TrimSpace(rt.Cfg.Provider), "ollama") {
		return ""
	}
	p := rt.OllamaProbe
	m := strings.TrimSpace(p.ConfiguredModel)
	if m == "" {
		m = strings.TrimSpace(rt.Cfg.Model())
	}
	if !p.Reachable {
		if p.Err != nil {
			return fmt.Sprintf("Ollama not reachable at %s (%v). Chat will fail until the server is up. Try: goclaw doctor", p.Host, p.Err)
		}
		if p.HTTPStatus != 0 {
			return fmt.Sprintf("Ollama at %s returned HTTP %d. Chat may fail until the server is healthy. Try: goclaw doctor", p.Host, p.HTTPStatus)
		}
		return fmt.Sprintf("Ollama not reachable at %s. Chat will fail until the server is up. Try: goclaw doctor", p.Host)
	}
	if m != "" && !p.ModelInLibrary {
		return fmt.Sprintf("Model %q is not in the local Ollama library (ollama pull %s). Try: goclaw doctor", m, m)
	}
	return ""
}

// maybeWarnOllamaUnreachable logs when the Ollama host is unhealthy at startup. Does not block startup.
func maybeWarnOllamaUnreachable(rt *ChatRuntime) {
	if rt == nil || !strings.EqualFold(strings.TrimSpace(rt.Cfg.Provider), "ollama") {
		return
	}
	p := rt.OllamaProbe
	if !p.Reachable {
		if p.Err != nil {
			slog.Warn("ollama unreachable (start continues); first chat may fail until the server is up",
				"host", p.Host,
				"err", p.Err)
			return
		}
		slog.Warn("ollama returned non-OK status (start continues); check the server",
			"host", p.Host,
			"status", fmt.Sprintf("http %d", p.HTTPStatus))
		return
	}
	if m := strings.TrimSpace(p.ConfiguredModel); m != "" && !p.ModelInLibrary {
		slog.Warn("ollama model not in local library (start continues); pull the model before chatting works",
			"host", p.Host,
			"model", m,
			"hint", "ollama pull "+m)
	}
}
