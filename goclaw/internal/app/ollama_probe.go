package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/config"
)

const ollamaStartupProbeTimeout = 2 * time.Second

// ollamaTagsProbeURL returns the GET /api/tags URL for the given host (empty host → default localhost).
func ollamaTagsProbeURL(host string) string {
	base := effectiveOllamaHost(host)
	return strings.TrimRight(base, "/") + "/api/tags"
}

// maybeWarnOllamaUnreachable logs a short warning when provider is Ollama but the
// host does not respond. Does not block startup.
func maybeWarnOllamaUnreachable(cfg config.Config) {
	if !strings.EqualFold(strings.TrimSpace(cfg.Provider), "ollama") {
		return
	}
	host := effectiveOllamaHost(cfg.OllamaHost)
	u := ollamaTagsProbeURL(cfg.OllamaHost)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: ollamaStartupProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("ollama unreachable (start continues); first chat may fail until the server is up",
			"host", host,
			"err", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("ollama returned non-OK status (start continues); check the server",
			"host", host,
			"status", fmt.Sprintf("http %d", resp.StatusCode))
	}
}
