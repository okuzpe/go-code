package mcp

import (
	"context"
	"io"
	"strings"
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

	// Tool must be retrievable by name (Get works regardless of hidden status).
	tool, ok := reg.Get("mcp__demo-srv__echo")
	require.True(t, ok, "registry should contain normalized MCP tool name")
	require.Equal(t, "mcp__demo-srv__echo", tool.Name())

	// MCP tools are registered as hidden: they must not appear in Specs() but must appear in AllSpecs().
	for _, s := range reg.Specs() {
		require.False(t, strings.HasPrefix(s.Name, "mcp__"), "hidden MCP tool %q must not appear in Specs()", s.Name)
	}
	found := false
	for _, s := range reg.AllSpecs() {
		if s.Name == "mcp__demo-srv__echo" {
			found = true
			break
		}
	}
	require.True(t, found, "MCP tool must appear in AllSpecs()")
	require.True(t, reg.IsHidden("mcp__demo-srv__echo"), "MCP tool must be marked as hidden")

	_ = sess.Close()
	_ = c2sW.Close()
	_ = s2cW.Close()
	<-done
}

func TestNewToolAdapterCompactsVerboseDescriptionAndSchema(t *testing.T) {
	info := ToolInfo{
		Name:        "echo",
		Description: strings.Repeat("verbose description ", 30),
		InputSchema: map[string]any{
			"type":        "object",
			"title":       "Very long schema",
			"description": strings.Repeat("schema description ", 30),
			"properties": map[string]any{
				"path": map[string]any{
					"type":                 "string",
					"description":          strings.Repeat("property description ", 30),
					"markdownDescription":  strings.Repeat("markdown description ", 20),
					"default":              "x",
					"example":              "y",
					"additionalProperties": false,
				},
			},
			"required": []any{"path"},
		},
	}

	tool := NewToolAdapter(nil, "demo", info)
	require.Len(t, []rune(tool.Description()), maxToolDescriptionRunes+3)
	schema, ok := tool.InputSchema().(map[string]any)
	require.True(t, ok)
	_, hasTitle := schema["title"]
	require.False(t, hasTitle)
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	pathSchema, ok := props["path"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, pathSchema["description"], "...")
	_, hasMarkdownDescription := pathSchema["markdownDescription"]
	require.False(t, hasMarkdownDescription)
	_, hasDefault := pathSchema["default"]
	require.False(t, hasDefault)
}
