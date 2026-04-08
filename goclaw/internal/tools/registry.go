// Package tools provides the tool registry and execution interface.
package tools

import (
	"context"
	"sort"
)

// Result is the output of a tool execution.
type Result struct {
	Content string
	IsError bool
}

// Tool is the interface every tool must implement.
type Tool interface {
	Name() string
	Description() string
	InputSchema() any
	Execute(ctx context.Context, input string) (Result, error)
}

// Registry holds all registered tools.
type Registry struct {
	tools map[string]Tool
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool. Panics on duplicate names.
func (r *Registry) Register(t Tool) {
	if _, exists := r.tools[t.Name()]; exists {
		panic("goclaw/tools: duplicate tool name: " + t.Name())
	}
	r.tools[t.Name()] = t
}

// Get looks up a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Specs returns the LLM-facing tool descriptions for all registered tools (sorted by name).
func (r *Registry) Specs() []ToolSpec {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]ToolSpec, 0, len(r.tools))
	for _, n := range names {
		t := r.tools[n]
		out = append(out, ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return out
}

// ToolSpec is the wire format passed to the LLM.
type ToolSpec struct {
	Name        string
	Description string
	InputSchema any
}
