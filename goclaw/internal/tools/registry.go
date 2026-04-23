// Package tools provides the tool registry and execution interface.
package tools

import (
	"context"
	"fmt"
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
	tools       map[string]Tool
	hiddenTools map[string]bool
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		tools:       make(map[string]Tool),
		hiddenTools: make(map[string]bool),
	}
}

// Register adds a tool. Panics on duplicate names.
func (r *Registry) Register(t Tool) {
	if err := r.Add(t); err != nil {
		panic(err.Error())
	}
}

// RegisterHidden adds a tool marked as hidden from the LLM prompt. Panics on duplicate names.
// Hidden tools are excluded from Specs() but discoverable via AllSpecs() and tool_search.
func (r *Registry) RegisterHidden(t Tool) {
	if err := r.AddHidden(t); err != nil {
		panic(err.Error())
	}
}

// Add adds a tool and returns an error if it already exists or if the tool is nil.
func (r *Registry) Add(t Tool) error {
	if t == nil {
		return fmt.Errorf("goclaw/tools: nil tool")
	}
	if _, exists := r.tools[t.Name()]; exists {
		return fmt.Errorf("goclaw/tools: duplicate tool name: %s", t.Name())
	}
	r.tools[t.Name()] = t
	return nil
}

// AddHidden adds a tool marked as hidden from the LLM prompt.
// Returns an error if the tool already exists or is nil.
func (r *Registry) AddHidden(t Tool) error {
	if err := r.Add(t); err != nil {
		return err
	}
	r.hiddenTools[t.Name()] = true
	return nil
}

// IsHidden reports whether the named tool is registered as hidden.
func (r *Registry) IsHidden(name string) bool {
	return r.hiddenTools[name]
}

// Get looks up a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Specs returns the LLM-facing tool descriptions for visible (non-hidden) tools, sorted by name.
// Use AllSpecs to include hidden tools (e.g. for tool_search).
func (r *Registry) Specs() []ToolSpec {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		if !r.hiddenTools[n] {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	out := make([]ToolSpec, 0, len(names))
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

// AllSpecs returns the LLM-facing tool descriptions for all registered tools including hidden ones,
// sorted by name. Used by tool_search to allow the model to discover hidden MCP tools.
func (r *Registry) AllSpecs() []ToolSpec {
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
