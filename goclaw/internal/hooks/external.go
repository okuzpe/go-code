package hooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"time"

	"github.com/okuzpe/goclaw/internal/tools"
)

const defaultExternalHookTimeout = 30 * time.Second

type externalCommand struct {
	path string
	args []string
}

type externalHTTP struct {
	url     string
	timeout time.Duration
}

func (r *Registry) OnCommand(event EventType, command string, args ...string) {
	if command == "" {
		return
	}
	r.cmdHooks[event] = append(r.cmdHooks[event], externalCommand{path: command, args: args})
}

// OnHTTP registers a POST hook; URL must pass the same policy as web_fetch (no loopback).
func (r *Registry) OnHTTP(event EventType, rawURL string, timeout time.Duration) {
	if rawURL == "" {
		return
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	r.httpHooks[event] = append(r.httpHooks[event], externalHTTP{url: rawURL, timeout: timeout})
}

func (r *Registry) fireExternal(ctx context.Context, e Event) error {
	payload, err := wireFromEvent(e)
	if err != nil {
		return fmt.Errorf("hooks: marshal payload: %w", err)
	}
	for _, c := range r.cmdHooks[e.Type] {
		if err := runExternalCommand(ctx, c, payload); err != nil {
			if e.Type == PreToolUse {
				return err
			}
			slog.WarnContext(ctx, "external command hook error", "event", string(e.Type), "cmd", c.path, "err", err)
		}
	}
	for _, h := range r.httpHooks[e.Type] {
		if err := runExternalHTTP(ctx, h, payload); err != nil {
			if e.Type == PreToolUse {
				return err
			}
			slog.WarnContext(ctx, "external http hook error", "event", string(e.Type), "url", h.url, "err", err)
		}
	}
	return nil
}

func runExternalCommand(ctx context.Context, c externalCommand, payload []byte) error {
	tctx, cancel := context.WithTimeout(ctx, defaultExternalHookTimeout)
	defer cancel()
	cmd := exec.CommandContext(tctx, c.path, c.args...)
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		return fmt.Errorf("hook %s blocked (exit 2): %s", c.path, string(out))
	}
	return fmt.Errorf("hook %s: %w: %s", c.path, err, string(out))
}

func runExternalHTTP(ctx context.Context, h externalHTTP, payload []byte) error {
	if err := tools.ValidateWebhookURL(h.url); err != nil {
		return fmt.Errorf("hook url: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(tctx, http.MethodPost, h.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("hook http status %d", resp.StatusCode)
	}
	return nil
}
