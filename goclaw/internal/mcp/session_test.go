package mcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/mcp"
)

// runMockMCPServer reads JSON-RPC lines from in and writes responses to out.
func runMockMCPServer(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, mcp.MaxMessageBytes)
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
				"id":      json.RawMessage(msg.ID),
				"result": map[string]any{
					"protocolVersion": mcp.ProtocolVersion,
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]any{"name": "mock", "version": "1.0"},
				},
			})
		case "notifications/initialized":
			// no response for notification
		case "tools/list":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      json.RawMessage(msg.ID),
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
				"id":      json.RawMessage(msg.ID),
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
				"id":      json.RawMessage(msg.ID),
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

	sess := mcp.NewPipedSession(c2sW, s2cR)
	ctx := context.Background()

	if err := sess.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	tools, err := sess.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools: %+v", tools)
	}
	out, isErr, err := sess.CallTool(ctx, "echo", `{"msg":"x"}`)
	if err != nil {
		t.Fatal(err)
	}
	if isErr {
		t.Fatal("unexpected isError")
	}
	if !strings.Contains(out, "hello from mock") {
		t.Fatalf("got %q", out)
	}

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
		sess := mcp.NewPipedSession(c2sW, s2cR)
		ctx := context.Background()

		// The server will inject a server-initiated request before returning
		// the tools/list response. The client must reply with an error for
		// that request and still successfully read the tools/list response.
		if err := sess.Initialize(ctx); err != nil {
			t.Errorf("initialize: %v", err)
			return
		}
		tools, err := sess.ListTools(ctx)
		if err != nil {
			t.Errorf("list tools: %v", err)
			return
		}
		if len(tools) != 1 || tools[0].Name != "echo" {
			t.Errorf("unexpected tools: %+v", tools)
		}
		_ = sess.Close()
		_ = c2sW.Close()
	}()

	// Custom mock server that injects a server-initiated request.
	sc := bufio.NewScanner(c2sR)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, mcp.MaxMessageBytes)
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
				"id":      json.RawMessage(msg.ID),
				"result": map[string]any{
					"protocolVersion": mcp.ProtocolVersion,
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
				"id":      json.RawMessage(msg.ID),
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

	if !gotErrorResponse {
		t.Fatal("expected client to send a -32601 error response for the server-initiated request")
	}
}
