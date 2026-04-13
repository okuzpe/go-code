package orchestrator

import "testing"

// TestMockParityHarness groups OpenAI-compatible mock scenarios for scripted CI runs
// (see scripts/run_mock_parity_harness.sh and scripts/mock_parity_scenarios.json).
func TestMockParityHarness(t *testing.T) {
	scenarios := []struct {
		name string
		fn   func(*testing.T)
	}{
		{"streaming_text", TestOrchestratorTextOnly},
		{"read_file_roundtrip", TestOrchestratorReadFileRoundTrip},
		{"multi_tool_turn_roundtrip", TestOrchestratorMultiToolRoundTrip},
		{"tool_ask_requires_approver", TestOrchestratorAskRequiresApprover},
		{"tool_user_declines", TestOrchestratorUserDeclinesTool},
		{"mcp_tool_roundtrip", TestOrchestratorMCPRoundTripRecordsToolResultInSession},
	}
	for _, s := range scenarios {
		s := s
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			s.fn(t)
		})
	}
}
