package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/tools"
)

// MCPToolCallTimeout is the default per-invocation timeout for tools/call.
const MCPToolCallTimeout = 60 * time.Second

// ToolAdapter implements tools.Tool for one remote MCP tool.
type ToolAdapter struct {
	sess     Conn
	serverID string
	remote   string
	desc     string
	schema   any
}

var _ tools.Tool = (*ToolAdapter)(nil)

// NewToolAdapter wraps a listed tool from the MCP server.
func NewToolAdapter(sess Conn, serverID string, info ToolInfo) *ToolAdapter {
	return &ToolAdapter{
		sess:     sess,
		serverID: serverID,
		remote:   info.Name,
		desc:     compactToolDescription(info.Description),
		schema:   compactJSONSchema(info.InputSchema),
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
	start := time.Now()
	content, isErr, err := t.sess.CallTool(callCtx, t.remote, input)
	slog.Debug("mcp call done", "server", t.serverID, "tool", t.remote, "elapsed", time.Since(start))
	if err != nil {
		return tools.Result{Content: "", IsError: true}, fmt.Errorf("mcp tools/call: %w", err)
	}
	return tools.Result{Content: content, IsError: isErr}, nil
}

// RegisterSessionTools registers every tool from ListTools under normalized names.
func RegisterSessionTools(ctx context.Context, reg *tools.Registry, sess Conn, serverID string) error {
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
		if err := reg.AddHidden(NewToolAdapter(sess, serverID, info)); err != nil {
			return err
		}
	}
	return nil
}

const maxToolDescriptionRunes = 220
const maxSchemaDescriptionRunes = 160

func compactToolDescription(desc string) string {
	desc = strings.Join(strings.Fields(strings.TrimSpace(desc)), " ")
	if len(desc) == 0 {
		return ""
	}
	runes := []rune(desc)
	if len(runes) <= maxToolDescriptionRunes {
		return desc
	}
	return string(runes[:maxToolDescriptionRunes]) + "..."
}

func compactJSONSchema(schema any) any {
	switch typed := schema.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			switch key {
			case "description":
				if s, ok := value.(string); ok {
					out[key] = compactSchemaDescription(s)
				}
			case "properties":
				if props, ok := value.(map[string]any); ok {
					trimmed := make(map[string]any, len(props))
					for propName, propSchema := range props {
						trimmed[propName] = compactJSONSchema(propSchema)
					}
					out[key] = trimmed
				}
			case "items", "additionalProperties":
				out[key] = compactJSONSchema(value)
			case "oneOf", "anyOf", "allOf":
				out[key] = compactJSONSchema(value)
			case "title", "examples", "example", "default", "$comment", "markdownDescription":
				// Drop verbose metadata that bloats prompts but does not help tool invocation.
				continue
			default:
				out[key] = compactJSONSchema(value)
			}
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, compactJSONSchema(item))
		}
		return out
	default:
		return schema
	}
}

func compactSchemaDescription(desc string) string {
	desc = strings.Join(strings.Fields(strings.TrimSpace(desc)), " ")
	runes := []rune(desc)
	if len(runes) <= maxSchemaDescriptionRunes {
		return desc
	}
	return string(runes[:maxSchemaDescriptionRunes]) + "..."
}
