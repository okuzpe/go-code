package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/okuzpe/goclaw/internal/tools"
)

// MCPToolCallTimeout is the default per-invocation timeout for tools/call.
const MCPToolCallTimeout = 60 * time.Second

// ToolAdapter implements tools.Tool for one remote MCP tool.
type ToolAdapter struct {
	sess     *Session
	serverID string
	remote   string
	desc     string
	schema   any
}

var _ tools.Tool = (*ToolAdapter)(nil)

// NewToolAdapter wraps a listed tool from the MCP server.
func NewToolAdapter(sess *Session, serverID string, info ToolInfo) *ToolAdapter {
	return &ToolAdapter{
		sess:     sess,
		serverID: serverID,
		remote:   info.Name,
		desc:     info.Description,
		schema:   info.InputSchema,
	}
}

// Name returns the normalized name exposed to the LLM.
func (t *ToolAdapter) Name() string {
	return NormalizeMCPToolName(t.serverID, t.remote)
}

func (t *ToolAdapter) Description() string {
	if t.desc != "" {
		return t.desc
	}
	return fmt.Sprintf("MCP tool %q from server %q", t.remote, t.serverID)
}

func (t *ToolAdapter) InputSchema() any { return t.schema }

func (t *ToolAdapter) Execute(ctx context.Context, input string) (tools.Result, error) {
	callCtx, cancel := context.WithTimeout(ctx, MCPToolCallTimeout)
	defer cancel()
	content, isErr, err := t.sess.CallTool(callCtx, t.remote, input)
	if err != nil {
		return tools.Result{Content: "", IsError: true}, fmt.Errorf("mcp tools/call: %w", err)
	}
	return tools.Result{Content: content, IsError: isErr}, nil
}

// RegisterSessionTools registers every tool from ListTools under normalized names.
func RegisterSessionTools(ctx context.Context, reg *tools.Registry, sess *Session, serverID string) error {
	if serverID == "" {
		return fmt.Errorf("mcp: empty server id")
	}
	infos, err := sess.ListTools(ctx)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if info.Name == "" {
			continue
		}
		reg.Register(NewToolAdapter(sess, serverID, info))
	}
	return nil
}
