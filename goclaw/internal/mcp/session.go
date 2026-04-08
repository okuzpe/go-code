// Package mcp implements a minimal MCP client over stdio (JSON-RPC, newline-delimited).
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// MaxMessageBytes is the maximum single-line JSON-RPC message size on stdio.
const MaxMessageBytes = 16 * 1024 * 1024

// Session is one MCP server connection over subprocess stdio.
type Session struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	cancel context.CancelFunc

	writeMu sync.Mutex
	idGen   atomic.Int64

	// pendingReply counts in-flight JSON-RPC error replies to server-initiated requests.
	// Close waits so tests and shutdown do not drop those lines on the floor.
	pendingReply sync.WaitGroup
}

// StartStdioSession launches command with args, wires stdin/stdout for MCP messages,
// and streams stderr to slog at debug level.
func StartStdioSession(ctx context.Context, command string, args []string, extraEnv []string, workDir string) (*Session, error) {
	if command == "" {
		return nil, fmt.Errorf("mcp: empty command")
	}
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, command, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	if len(extraEnv) > 0 {
		cmd.Env = append(append([]string{}, os.Environ()...), extraEnv...)
	} else {
		cmd.Env = os.Environ()
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("mcp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = stdin.Close()
		return nil, fmt.Errorf("mcp stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		_ = stdin.Close()
		return nil, fmt.Errorf("mcp stderr: %w", err)
	}
	go drainStderr(ctx, stderr)

	if err := cmd.Start(); err != nil {
		cancel()
		_ = stdin.Close()
		return nil, fmt.Errorf("mcp start: %w", err)
	}

	s := &Session{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		cancel: cancel,
	}
	return s, nil
}

func drainStderr(ctx context.Context, r io.Reader) {
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			slog.DebugContext(ctx, "mcp server stderr", "line", sc.Text())
		}
	}
}

// Close terminates the subprocess and releases pipes.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.pendingReply.Wait()
	if s.cancel != nil {
		s.cancel()
	}
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
	return nil
}

func (s *Session) nextID() int64 {
	return s.idGen.Add(1)
}

func (s *Session) writeLine(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if bytes.Contains(b, []byte{'\n'}) || bytes.Contains(b, []byte{'\r'}) {
		return fmt.Errorf("mcp: json must not contain newlines")
	}
	if len(b) > MaxMessageBytes {
		return fmt.Errorf("mcp: message exceeds max size")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.stdin.Write(append(b, '\n'))
	return err
}

// rpcEnvelope matches JSON-RPC 2.0 responses and server-originated requests.
type rpcEnvelope struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id"`
	Method  string           `json:"method"`
	Result  json.RawMessage  `json:"result"`
	Error   *jsonRPCErrorObj `json:"error"`
}

type jsonRPCErrorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func idMatches(want int64, raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n == want
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		m, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return false
		}
		return m == want
	}
	return false
}

func (s *Session) readLine(ctx context.Context) ([]byte, error) {
	type res struct {
		line []byte
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		line, err := s.stdout.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		ch <- res{line, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.line, r.err
	}
}

func (s *Session) readUntilResponse(ctx context.Context, wantID int64) (json.RawMessage, error) {
	for {
		line, err := s.readLine(ctx)
		if err != nil {
			return nil, err
		}
		if len(line) == 0 {
			continue
		}
		if len(line) > MaxMessageBytes {
			return nil, fmt.Errorf("mcp: response exceeds max size")
		}
		var env rpcEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		// Server-initiated JSON-RPC notification (no id) — ignore safely.
		if env.Method != "" && len(env.ID) == 0 {
			continue
		}
		// Server-initiated JSON-RPC request (method + id): the MCP spec
		// requires us to respond, otherwise the server may consider the
		// connection broken. Reply with "method not found" (-32601).
		if env.Method != "" && len(env.ID) > 0 {
			slog.Debug("mcp: rejecting server-initiated request", "method", env.Method)
			errResp := map[string]any{
				"jsonrpc": "2.0",
				"id":      env.ID,
				"error": map[string]any{
					"code":    -32601,
					"message": fmt.Sprintf("method not supported: %s", env.Method),
				},
			}
			// Async write: if we block here, the mock server can deadlock while still
			// encoding the next stdout line before it returns to Scan on stdin.
			s.pendingReply.Add(1)
			go func() {
				defer s.pendingReply.Done()
				if err := s.writeLine(errResp); err != nil {
					slog.Debug("mcp: write reject response", "err", err)
				}
			}()
			continue
		}
		if len(env.ID) == 0 {
			continue
		}
		if !idMatches(wantID, env.ID) {
			continue
		}
		if env.Error != nil {
			return nil, fmt.Errorf("mcp rpc error %d: %s", env.Error.Code, env.Error.Message)
		}
		return env.Result, nil
	}
}

// Initialize runs the MCP handshake (initialize + notifications/initialized).
func (s *Session) Initialize(ctx context.Context) error {
	id := s.nextID()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "goclaw",
				"version": "0.1.0",
			},
		},
	}
	if err := s.writeLine(req); err != nil {
		return err
	}
	if _, err := s.readUntilResponse(ctx, id); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	}
	return s.writeLine(notif)
}

type toolsListResult struct {
	Tools []struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema"`
	} `json:"tools"`
}

// ListTools returns tools advertised by the server after Initialize.
func (s *Session) ListTools(ctx context.Context) ([]ToolInfo, error) {
	id := s.nextID()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/list",
		"params":  map[string]any{},
	}
	if err := s.writeLine(req); err != nil {
		return nil, err
	}
	raw, err := s.readUntilResponse(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}
	var tr toolsListResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf("tools/list decode: %w", err)
	}
	out := make([]ToolInfo, 0, len(tr.Tools))
	for _, t := range tr.Tools {
		var schema any
		if len(t.InputSchema) > 0 {
			_ = json.Unmarshal(t.InputSchema, &schema)
		}
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, ToolInfo{
			Name:        t.Name,
			Description: strings.TrimSpace(t.Description),
			InputSchema: schema,
		})
	}
	return out, nil
}

type toolsCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// CallTool invokes tools/call. argumentsJSON must be a JSON object string (e.g. "{}").
func (s *Session) CallTool(ctx context.Context, toolName, argumentsJSON string) (content string, isError bool, err error) {
	if argumentsJSON == "" {
		argumentsJSON = "{}"
	}
	if !json.Valid([]byte(argumentsJSON)) {
		return "", true, fmt.Errorf("mcp: arguments must be valid JSON")
	}
	id := s.nextID()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": json.RawMessage(argumentsJSON),
		},
	}
	if err := s.writeLine(req); err != nil {
		return "", false, err
	}
	raw, err := s.readUntilResponse(ctx, id)
	if err != nil {
		return "", false, fmt.Errorf("tools/call: %w", err)
	}
	var tr toolsCallResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", true, fmt.Errorf("tools/call decode: %w", err)
	}
	var sb strings.Builder
	for _, c := range tr.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String(), tr.IsError, nil
}

// NewPipedSession is used in tests: client writes to serverIn, reads from serverOut.
func NewPipedSession(serverIn io.Writer, serverOut io.Reader) *Session {
	return &Session{
		stdin:  nopCloser{Writer: serverIn},
		stdout: bufio.NewReader(serverOut),
	}
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

// AttachCancel sets a cancel func for Close() in piped test sessions.
func (s *Session) AttachCancel(cancel context.CancelFunc) {
	s.cancel = cancel
}
