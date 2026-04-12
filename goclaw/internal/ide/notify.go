// Package ide holds optional local editor integration (D21).
package ide

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const ideNotifyHTTPTimeout = 2 * time.Second

// Notifier is called after selected tools complete (best-effort).
type Notifier interface {
	AfterTool(toolName string, resultBytes int, isError bool)
}

// Noop does nothing.
type Noop struct{}

func (Noop) AfterTool(string, int, bool) {}

// FromEnv returns an HTTP notifier when GOCLAW_IDE_NOTIFY_URL is set to
// http://127.0.0.1:* or http://localhost:* only (no remote URLs).
func FromEnv() Notifier {
	raw := strings.TrimSpace(os.Getenv("GOCLAW_IDE_NOTIFY_URL"))
	if raw == "" {
		return Noop{}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" {
		return Noop{}
	}
	host := strings.ToLower(u.Hostname())
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return Noop{}
	}
	return &localHTTPNotify{url: raw, client: &http.Client{Timeout: ideNotifyHTTPTimeout}}
}

type localHTTPNotify struct {
	url    string
	client *http.Client
}

func (l *localHTTPNotify) AfterTool(toolName string, resultBytes int, isError bool) {
	body := fmt.Sprintf(`{"tool":%q,"result_bytes":%d,"is_error":%v}`, toolName, resultBytes, isError)
	req, err := http.NewRequest(http.MethodPost, l.url, bytes.NewReader([]byte(body)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	_, _ = l.client.Do(req)
}
