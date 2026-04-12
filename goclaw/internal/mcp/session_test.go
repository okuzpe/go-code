package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// runMockMCPServer reads JSON-RPC lines from in and writes responses to out.
func runMockMCPServer(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, MaxMessageBytes)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := sc.Bytes()
		var msg struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if msg.Method == "" {
			continue
		}
		switch msg.Method {
		case "initialize":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result": map[string]any{
					"protocolVersion": ProtocolVersion,
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]any{"name": "mock", "version": "1.0"},
				},
			})
		case "notifications/initialized":
			// no response for notification
		case "tools/list":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result": map[string]any{
					"tools": []map[string]any{
						{
							"name":        "echo",
							"description": "echo args",
							"inputSchema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"msg": map[string]any{"type": "string"},
								},
							},
						},
					},
				},
			})
		case "tools/call":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result": map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": "hello from mock"},
					},
					"isError": false,
				},
			})
		default:
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"error": map[string]any{
					"code":    -32601,
					"message": fmt.Sprintf("unknown method %s", msg.Method),
				},
			})
		}
	}
	return sc.Err()
}

func TestPipedSessionInitializeListCall(t *testing.T) {
	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()

	done := make(chan error, 1)
	go func() {
		done <- runMockMCPServer(c2sR, s2cW)
	}()

	sess := NewPipedSession(c2sW, s2cR)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	require.NoError(t, sess.Initialize(ctx))
	tools, err := sess.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "echo", tools[0].Name)
	out, isErr, err := sess.CallTool(ctx, "echo", `{"msg":"x"}`)
	require.NoError(t, err)
	require.False(t, isErr)
	require.Contains(t, out, "hello from mock")

	_ = sess.Close()
	_ = c2sW.Close()
	_ = s2cW.Close()
	<-done
}

// TestServerInitiatedRequestGetsErrorResponse verifies that when the MCP server
// sends a JSON-RPC request (method + id), the client responds with a -32601
// "method not found" error instead of silently dropping it.
func TestServerInitiatedRequestGetsErrorResponse(t *testing.T) {
	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		sess := NewPipedSession(c2sW, s2cR)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// The server will inject a server-initiated request before returning
		// the tools/list response. The client must reply with an error for
		// that request and still successfully read the tools/list response.
		require.NoError(t, sess.Initialize(ctx))
		tools, err := sess.ListTools(ctx)
		require.NoError(t, err)
		require.Len(t, tools, 1)
		require.Equal(t, "echo", tools[0].Name)
		_ = sess.Close()
		_ = c2sW.Close()
	}()

	// Custom mock server that injects a server-initiated request.
	sc := bufio.NewScanner(c2sR)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, MaxMessageBytes)
	enc := json.NewEncoder(s2cW)

	var gotErrorResponse bool
	for sc.Scan() {
		var msg struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Error   *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(sc.Bytes(), &msg); err != nil {
			continue
		}

		// Check if client sent us an error response to our server-initiated request.
		if msg.Error != nil && msg.Error.Code == -32601 {
			gotErrorResponse = true
			continue
		}

		if msg.Method == "" {
			continue
		}
		switch msg.Method {
		case "initialize":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result": map[string]any{
					"protocolVersion": ProtocolVersion,
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]any{"name": "mock", "version": "1.0"},
				},
			})
		case "notifications/initialized":
			// no response
		case "tools/list":
			// Inject a server-initiated request BEFORE the tools/list response.
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      42,
				"method":  "sampling/createMessage",
				"params":  map[string]any{"prompt": "test"},
			})
			// Then send the real tools/list response.
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result": map[string]any{
					"tools": []map[string]any{
						{"name": "echo", "description": "echo args", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
					},
				},
			})
		}
	}
	_ = s2cW.Close()
	<-clientDone

	require.True(t, gotErrorResponse, "expected client to send a -32601 error response for the server-initiated request")
}
