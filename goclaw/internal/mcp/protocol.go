package mcp

// ProtocolVersion is the MCP protocol version sent in initialize.
const ProtocolVersion = "2024-11-05"

// ToolInfo describes one tool from tools/list.
type ToolInfo struct {
	Name        string
	Description string
	InputSchema any
}
