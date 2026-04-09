package mcp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
)

// ResilientConn wraps an MCP Conn and re-dials once when a tool call fails with a
// recoverable transport error (EOF, broken pipe, connection reset, etc.).
type ResilientConn struct {
	mu   sync.Mutex
	inner Conn
	dial func(context.Context) (Conn, error)
}

// NewResilientConn dials once, runs Initialize, and returns a wrapper for runtime use.
func NewResilientConn(ctx context.Context, dial func(context.Context) (Conn, error)) (*ResilientConn, error) {
	c, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.Initialize(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	return &ResilientConn{inner: c, dial: dial}, nil
}

// Initialize is a no-op; the handshake already ran in NewResilientConn and on each reconnect.
func (r *ResilientConn) Initialize(context.Context) error {
	return nil
}

func (r *ResilientConn) ListTools(ctx context.Context) ([]ToolInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inner.ListTools(ctx)
}

func (r *ResilientConn) CallTool(ctx context.Context, toolName, argumentsJSON string) (string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	content, isErr, err := r.inner.CallTool(ctx, toolName, argumentsJSON)
	if err == nil || !isRecoverableMCPError(err) {
		return content, isErr, err
	}
	firstErr := err
	if rerr := r.reconnectUnlocked(ctx); rerr != nil {
		return "", false, firstErr
	}
	slog.Debug("mcp: reconnected after transport error", "err", firstErr)
	content, isErr, err = r.inner.CallTool(ctx, toolName, argumentsJSON)
	return content, isErr, err
}

func (r *ResilientConn) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.inner == nil {
		return nil
	}
	err := r.inner.Close()
	r.inner = nil
	return err
}

func (r *ResilientConn) reconnectUnlocked(ctx context.Context) error {
	if r.dial == nil {
		return errors.New("mcp: no dial function")
	}
	_ = r.inner.Close()
	c, err := r.dial(ctx)
	if err != nil {
		return err
	}
	if err := c.Initialize(ctx); err != nil {
		_ = c.Close()
		return err
	}
	r.inner = c
	return nil
}

func isRecoverableMCPError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "connection closed"),
		strings.Contains(s, "broken pipe"),
		strings.Contains(s, "connection reset"),
		strings.Contains(s, "use of closed network connection"),
		strings.Contains(s, "server process may have exited"),
		strings.Contains(s, "mcp: send tools/call"),
		strings.Contains(s, "mcp: send"),
		strings.Contains(s, "unexpected eof"),
		strings.Contains(s, "mcp http: status 5"):
		return true
	default:
		return false
	}
}
