package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type seqConn struct {
	callFail atomic.Bool
	closed   atomic.Bool
}

func (s *seqConn) Initialize(context.Context) error { return nil }

func (s *seqConn) ListTools(context.Context) ([]ToolInfo, error) {
	return []ToolInfo{{Name: "t", InputSchema: map[string]any{"type": "object"}}}, nil
}

func (s *seqConn) CallTool(context.Context, string, string) (string, bool, error) {
	if s.callFail.Load() {
		return "", false, fmt.Errorf("mcp: connection closed while waiting for tools/call (server process may have exited): %w", io.EOF)
	}
	return "ok", false, nil
}

func (s *seqConn) Close() error {
	s.closed.Store(true)
	return nil
}

func TestResilientConnReconnectsOnCallToolFailure(t *testing.T) {
	var gen atomic.Int32
	dial := func(context.Context) (Conn, error) {
		n := gen.Add(1)
		c := &seqConn{}
		if n == 1 {
			c.callFail.Store(true)
		}
		return c, nil
	}

	ctx := context.Background()
	r, err := NewResilientConn(ctx, dial)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	out, isErr, err := r.CallTool(ctx, "t", `{}`)
	require.NoError(t, err)
	require.False(t, isErr)
	require.Equal(t, "ok", out)
	require.Equal(t, int32(2), gen.Load(), "expected second dial after first CallTool failed")
}

func TestResilientConnNoRetryOnLogicError(t *testing.T) {
	var gen atomic.Int32
	dial := func(context.Context) (Conn, error) {
		gen.Add(1)
		return &stubBadToolConn{}, nil
	}
	ctx := context.Background()
	r, err := NewResilientConn(ctx, dial)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	out, isErr, err := r.CallTool(ctx, "t", `{}`)
	require.NoError(t, err)
	require.True(t, isErr)
	require.Equal(t, "bad", out)
	require.Equal(t, int32(1), gen.Load())
}

type stubBadToolConn struct{}

func (stubBadToolConn) Initialize(context.Context) error { return nil }

func (stubBadToolConn) ListTools(context.Context) ([]ToolInfo, error) {
	return []ToolInfo{{Name: "t", InputSchema: map[string]any{"type": "object"}}}, nil
}

func (stubBadToolConn) CallTool(context.Context, string, string) (string, bool, error) {
	// Tool-level error (no Go error): must not trigger transport reconnect.
	return "bad", true, nil
}

func (stubBadToolConn) Close() error { return nil }

func TestIsRecoverableMCPError(t *testing.T) {
	require.True(t, isRecoverableMCPError(io.EOF))
	require.True(t, isRecoverableMCPError(errors.New("connection reset by peer")))
	require.False(t, isRecoverableMCPError(errors.New("tools/call decode: broken")))
}
