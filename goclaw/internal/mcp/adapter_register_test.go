package mcp

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/stretchr/testify/require"
)

func TestRegisterSessionToolsExposesNormalizedNames(t *testing.T) {
	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- runMockMCPServer(c2sR, s2cW) }()

	sess := NewPipedSession(c2sW, s2cR)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, sess.Initialize(ctx))

	reg := tools.New()
	require.NoError(t, RegisterSessionTools(ctx, reg, sess, "demo-srv"))
	tool, ok := reg.Get("mcp__demo-srv__echo")
	require.True(t, ok, "registry should contain normalized MCP tool name")
	require.Equal(t, "mcp__demo-srv__echo", tool.Name())

	_ = tool // appease linters if Name is only check
	_ = sess.Close()
	_ = c2sW.Close()
	_ = s2cW.Close()
	<-done
}
