package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// minimalTool is a test fixture that satisfies the Tool interface.
type minimalTool struct{ name string }

func (m minimalTool) Name() string        { return m.name }
func (m minimalTool) Description() string { return "desc-" + m.name }
func (m minimalTool) InputSchema() any    { return map[string]any{"type": "object"} }
func (m minimalTool) Execute(_ context.Context, _ string) (Result, error) {
	return Result{Content: "ok"}, nil
}

func TestRegistry_registerAndGet(t *testing.T) {
	reg := New()
	reg.Register(minimalTool{"alpha"})

	tool, ok := reg.Get("alpha")
	require.True(t, ok)
	require.Equal(t, "alpha", tool.Name())
}

func TestRegistry_getMissing(t *testing.T) {
	reg := New()
	_, ok := reg.Get("nonexistent")
	require.False(t, ok)
}

func TestRegistry_duplicatePanics(t *testing.T) {
	reg := New()
	reg.Register(minimalTool{"beta"})
	require.Panics(t, func() {
		reg.Register(minimalTool{"beta"})
	})
}

func TestRegistry_addDuplicateReturnsError(t *testing.T) {
	reg := New()
	require.NoError(t, reg.Add(minimalTool{"beta"}))
	require.Error(t, reg.Add(minimalTool{"beta"}))
}

func TestRegistry_addNilReturnsError(t *testing.T) {
	reg := New()
	require.Error(t, reg.Add(nil))
}

func TestRegistry_specs_sortedByName(t *testing.T) {
	reg := New()
	reg.Register(minimalTool{"zeta"})
	reg.Register(minimalTool{"alpha"})
	reg.Register(minimalTool{"mango"})

	specs := reg.Specs()
	require.Len(t, specs, 3)
	require.Equal(t, "alpha", specs[0].Name)
	require.Equal(t, "mango", specs[1].Name)
	require.Equal(t, "zeta", specs[2].Name)
}

func TestRegistry_specs_includesDescriptionAndSchema(t *testing.T) {
	reg := New()
	reg.Register(minimalTool{"tool1"})

	specs := reg.Specs()
	require.Len(t, specs, 1)
	require.Equal(t, "desc-tool1", specs[0].Description)
	require.NotNil(t, specs[0].InputSchema)
}

func TestRegistry_emptySpecs(t *testing.T) {
	reg := New()
	require.Empty(t, reg.Specs())
}
