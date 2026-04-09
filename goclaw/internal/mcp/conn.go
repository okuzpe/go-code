package mcp

import "context"

// Conn is an MCP client session (stdio subprocess or streamable HTTP).
type Conn interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]ToolInfo, error)
	CallTool(ctx context.Context, toolName, argumentsJSON string) (content string, isError bool, err error)
	Close() error
}
