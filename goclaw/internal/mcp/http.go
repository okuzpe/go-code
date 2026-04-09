package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
)

// streamHTTPProtocolVersion is sent in initialize and MCP-Protocol-Version for streamable HTTP.
const streamHTTPProtocolVersion = "2025-03-26"

// HTTPSession is a Streamable HTTP MCP client (spec 2025-03-26+).
type HTTPSession struct {
	endpoint  string
	extraHdr  http.Header
	client    *http.Client
	sessionID string
	idGen     atomic.Int64
}

// NewHTTPSession creates a client for an MCP endpoint URL (e.g. https://host/mcp).
// Call ValidateHTTPURL on the parsed URL before this constructor if you enforce host policy.
func NewHTTPSession(endpointURL string, headers map[string]string) (*HTTPSession, error) {
	u, err := url.Parse(strings.TrimSpace(endpointURL))
	if err != nil {
		return nil, fmt.Errorf("mcp http: parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("mcp http: url must be http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("mcp http: url needs host")
	}
	ep := strings.TrimRight(u.String(), "/")

	h := make(http.Header)
	for k, v := range headers {
		h.Set(k, v)
	}
	return &HTTPSession{
		endpoint: ep,
		extraHdr: h,
		client:   &http.Client{},
	}, nil
}

// ValidateHTTPURL returns nil if an MCP HTTP endpoint URL is allowed.
// By default only loopback hosts are allowed; set allowRemote to permit any host (user-opt-in).
func ValidateHTTPURL(u *url.URL, allowRemote bool) error {
	if u == nil {
		return fmt.Errorf("mcp: nil url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("mcp: url scheme must be http or https")
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("mcp: url needs host")
	}
	if allowRemote {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return nil
	}
	return fmt.Errorf("mcp: host %q is not allowed (loopback only unless \"mcp_allow_remote_urls\": true)", host)
}

func (h *HTTPSession) nextID() int64 {
	return h.idGen.Add(1)
}

func (h *HTTPSession) baseHeaders() http.Header {
	hdr := make(http.Header)
	hdr.Set("Accept", "application/json, text/event-stream")
	hdr.Set("Content-Type", "application/json")
	hdr.Set("MCP-Protocol-Version", streamHTTPProtocolVersion)
	for k, vals := range h.extraHdr {
		for _, v := range vals {
			hdr.Add(k, v)
		}
	}
	if h.sessionID != "" {
		hdr.Set("Mcp-Session-Id", h.sessionID)
	}
	return hdr
}

func (h *HTTPSession) captureSession(resp *http.Response) {
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		h.sessionID = sid
	}
}

func (h *HTTPSession) postJSON(ctx context.Context, payload map[string]any) (*http.Response, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(b) > MaxMessageBytes {
		return nil, fmt.Errorf("mcp http: request exceeds max size")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header = h.baseHeaders()
	return h.client.Do(req)
}

// Initialize runs initialize + notifications/initialized over HTTP.
func (h *HTTPSession) Initialize(ctx context.Context) error {
	id := h.nextID()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": streamHTTPProtocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "goclaw",
				"version": "0.1.0",
			},
		},
	}
	if _, err := h.exchange(ctx, req, id); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	}
	return h.postNotification(ctx, notif)
}

func (h *HTTPSession) postNotification(ctx context.Context, msg map[string]any) error {
	resp, err := h.postJSON(ctx, msg)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	h.captureSession(resp)
	switch resp.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusOK:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	default:
		return fmt.Errorf("mcp http: notification expected 202 or 200, got %d", resp.StatusCode)
	}
}

func (h *HTTPSession) exchange(ctx context.Context, msg map[string]any, wantID int64) (json.RawMessage, error) {
	resp, err := h.postJSON(ctx, msg)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	h.captureSession(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp http: status %d", resp.StatusCode)
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	switch {
	case strings.Contains(ct, "application/json"):
		var env rpcEnvelope
		if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
			return nil, err
		}
		if env.Error != nil {
			return nil, fmt.Errorf("mcp rpc error %d: %s", env.Error.Code, env.Error.Message)
		}
		if len(env.ID) == 0 || !idMatches(wantID, env.ID) {
			return nil, fmt.Errorf("mcp http: missing or mismatched response id")
		}
		return env.Result, nil
	case strings.Contains(ct, "text/event-stream"):
		return h.readSSEUntilResponse(ctx, resp.Body, wantID)
	default:
		return nil, fmt.Errorf("mcp http: unexpected content-type %q", resp.Header.Get("Content-Type"))
	}
}

func (h *HTTPSession) readSSEUntilResponse(ctx context.Context, body io.Reader, wantID int64) (json.RawMessage, error) {
	sc := bufio.NewScanner(body)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, MaxMessageBytes)
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		var env rpcEnvelope
		if err := json.Unmarshal([]byte(data), &env); err != nil {
			continue
		}
		if env.Method != "" && len(env.ID) > 0 {
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
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("mcp http: SSE ended without response for id %d", wantID)
}

// ListTools calls tools/list.
func (h *HTTPSession) ListTools(ctx context.Context) ([]ToolInfo, error) {
	id := h.nextID()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/list",
		"params":  map[string]any{},
	}
	raw, err := h.exchange(ctx, req, id)
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

// CallTool invokes tools/call.
func (h *HTTPSession) CallTool(ctx context.Context, toolName, argumentsJSON string) (string, bool, error) {
	if argumentsJSON == "" {
		argumentsJSON = "{}"
	}
	if !json.Valid([]byte(argumentsJSON)) {
		return "", true, fmt.Errorf("mcp: arguments must be valid JSON")
	}
	id := h.nextID()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": json.RawMessage(argumentsJSON),
		},
	}
	raw, err := h.exchange(ctx, req, id)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", false, fmt.Errorf("mcp: connection closed while waiting for tools/call: %w", err)
		}
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

// Close releases resources (no-op for HTTP).
func (h *HTTPSession) Close() error {
	return nil
}
