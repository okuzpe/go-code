package coordinator

import "testing"

// TestMockParityHarness groups coordinator + mock-Anthropic scenarios for scripted runs
// (see scripts/run_mock_parity_harness.sh and scripts/mock_parity_scenarios.json).
func TestMockParityHarness(t *testing.T) {
	scenarios := []struct {
		name string
		fn   func(*testing.T)
	}{
		{"spawn_agent_success", TestSpawnAgentSuccess},
		{"spawn_agent_worker_failure", TestSpawnAgentWorkerFailure},
	}
	for _, s := range scenarios {
		s := s
		t.Run(s.name, func(t *testing.T) {
			s.fn(t)
		})
	}
}
