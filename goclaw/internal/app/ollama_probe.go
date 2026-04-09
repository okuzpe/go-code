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

// maybeWarnOllamaUnreachable logs a short warning when provider is Ollama but the
// host does not respond. Does not block startup.
func maybeWarnOllamaUnreachable(cfg config.Config) {
	if !strings.EqualFold(strings.TrimSpace(cfg.Provider), "ollama") {
		return
	}
	host := strings.TrimSpace(cfg.OllamaHost)
	if host == "" {
		return
	}
	u := strings.TrimRight(host, "/") + "/api/tags"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("ollama unreachable (start continues); first chat may fail until the server is up",
			"host", host,
			"err", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("ollama returned non-OK status (start continues); check the server",
			"host", host,
			"status", fmt.Sprintf("http %d", resp.StatusCode))
	}
}
