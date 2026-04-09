package coordinator

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/stretchr/testify/require"
)

// TestCoordinatorTools_interfaceCoverage exercises Description and InputSchema
// on all coordinator tools so go cover does not flag them as dead code.
func TestCoordinatorTools_interfaceCoverage(t *testing.T) {
	spawnTool := New(config.Default(), nil, tools.New(), permissions.NewPolicy(), hooks.New())
	stopTool := NewStopTask()

	for _, tool := range []interface {
		Name() string
		Description() string
		InputSchema() any
	}{spawnTool, stopTool} {
		require.NotEmpty(t, tool.Name())
		require.NotEmpty(t, tool.Description())
		require.NotNil(t, tool.InputSchema())
	}
}
