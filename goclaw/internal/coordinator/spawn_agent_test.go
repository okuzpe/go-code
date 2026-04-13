package coordinator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/okuzpe/goclaw/testutil/mockopenai"
	"github.com/stretchr/testify/require"
)

// buildWorkerReg returns a minimal worker registry (no built-ins needed for most tests).
func buildWorkerReg() *tools.Registry {
	return tools.New()
}

func testCfg() config.Config {
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "mock-model"
	return cfg
}

// TestSpawnAgentSuccess verifies that a successful worker run produces a
// WorkerNotification with status "completed" and the expected result text.
func TestSpawnAgentSuccess(t *testing.T) {
	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "list all Go files", Response: "Found: main.go, run.go"},
	})
	defer srv.Close()

	cfg := testCfg()
	client := llm.NewOpenAICompat("test-key", srv.URL+"/v1")
	workerReg := buildWorkerReg()
	policy := permissions.NewPolicy()
	hookReg := hooks.New()

	spawnTool := New(cfg, client, workerReg, policy, hookReg)

	// Allow for the policy (no tools in registry so nothing to approve).
	_ = agents.Explore // ensure agents package is accessible

	input := `{"profile":"explore","task":"list all Go files in the project"}`
	result, err := spawnTool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, result.IsError, "Execute returned IsError=true: %s", result.Content)

	var notif WorkerNotification
	require.NoError(t, json.Unmarshal([]byte(result.Content), &notif), "content: %s", result.Content)
	require.NotEmpty(t, notif.TaskID)
	require.NotEmpty(t, notif.Summary)

	want := WorkerNotification{
		TaskID:  notif.TaskID,
		Status:  "completed",
		Profile: "explore",
		Result:  "Found: main.go, run.go",
		Summary: notif.Summary, // verified non-empty above; exact text is not a contract
	}
	if diff := cmp.Diff(want, notif); diff != "" {
		t.Fatalf("WorkerNotification mismatch (-want +got):\n%s", diff)
	}
}

// TestSpawnAgentUnknownProfile verifies that an unknown profile returns IsError=true
// without making any LLM calls.
func TestSpawnAgentUnknownProfile(t *testing.T) {
	// No mock server — any LLM call would panic/fail, proving no call is made.
	cfg := testCfg()

	spawnTool := New(cfg, nil, buildWorkerReg(), permissions.NewPolicy(), hooks.New())

	input := `{"profile":"nonexistent","task":"do something"}`
	result, err := spawnTool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, result.IsError, "content: %s", result.Content)
}

// TestSpawnAgentCoordinatorNesting verifies that spawning a coordinator worker is rejected.
func TestSpawnAgentCoordinatorNesting(t *testing.T) {
	cfg := testCfg()

	// Register coordinator profile temporarily for this test.
	spawnTool := New(cfg, nil, buildWorkerReg(), permissions.NewPolicy(), hooks.New())

	input := `{"profile":"coordinator","task":"do something"}`
	result, err := spawnTool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, result.IsError, "content: %s", result.Content)
}

// TestSpawnAgentInvalidJSON verifies that malformed input returns IsError=true.
func TestSpawnAgentInvalidJSON(t *testing.T) {
	spawnTool := New(config.Default(), nil, buildWorkerReg(), permissions.NewPolicy(), hooks.New())

	result, err := spawnTool.Execute(context.Background(), `{invalid json`)
	require.NoError(t, err)
	require.True(t, result.IsError, "content: %s", result.Content)
}

// TestSpawnAgentMissingTask verifies that an empty task returns IsError=true.
func TestSpawnAgentMissingTask(t *testing.T) {
	spawnTool := New(config.Default(), nil, buildWorkerReg(), permissions.NewPolicy(), hooks.New())

	input := `{"profile":"explore","task":""}`
	result, err := spawnTool.Execute(context.Background(), input)
	require.NoError(t, err)
	require.True(t, result.IsError, "content: %s", result.Content)
}

// TestSpawnAgentWorkerFailure verifies that an LLM error produces status "failed".
func TestSpawnAgentWorkerFailure(t *testing.T) {
	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "", StatusCode: 500},
	})
	defer srv.Close()

	cfg := testCfg()
	client := llm.NewOpenAICompat("test-key", srv.URL+"/v1")
	spawnTool := New(cfg, client, buildWorkerReg(), permissions.NewPolicy(), hooks.New())

	input := `{"profile":"explore","task":"do anything"}`
	result, err := spawnTool.Execute(context.Background(), input)
	require.NoError(t, err)
	// Worker failure is reported as a structured result, not a Go error.
	require.False(t, result.IsError, "content: %s", result.Content)
	var notif WorkerNotification
	require.NoError(t, json.Unmarshal([]byte(result.Content), &notif))
	require.Equal(t, "failed", notif.Status)
}
