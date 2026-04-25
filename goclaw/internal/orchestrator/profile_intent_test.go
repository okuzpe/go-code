package orchestrator

import "testing"

func TestClassifyProfileIntentRules_hubDelegate(t *testing.T) {
	got := ClassifyProfileIntentRules("Delegate to two workers: one fixes auth, one updates docs. Run them in parallel.")
	if got.Intent != ProfileIntentHubDelegate {
		t.Fatalf("intent = %q, want hub_delegate", got.Intent)
	}
}

func TestClassifyProfileIntentRules_directCode(t *testing.T) {
	cases := []struct {
		msg string
	}{
		{"Implement token refresh in internal/oauth/session.go"},
		{"Fix the nil pointer in runUserTurn when sink is nil"},
	}
	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			got := ClassifyProfileIntentRules(tc.msg)
			if got.Intent != ProfileIntentDirectCode {
				t.Fatalf("intent = %q conf=%q, want direct_code", got.Intent, got.Confidence)
			}
		})
	}
}

func TestClassifyProfileIntentRules_readOnlyScan(t *testing.T) {
	got := ClassifyProfileIntentRules("Where is PrepareChatRuntime defined? No code changes.")
	if got.Intent != ProfileIntentReadOnlyScan {
		t.Fatalf("intent = %q, want read_only_scan", got.Intent)
	}
}

func TestClassifyProfileIntentRules_stay(t *testing.T) {
	got := ClassifyProfileIntentRules("Thanks, that helped.")
	if got.Intent != ProfileIntentStay {
		t.Fatalf("intent = %q, want stay", got.Intent)
	}
}

func TestClassifyProfileIntentRules_repoWideAuditToPlan(t *testing.T) {
	got := ClassifyProfileIntentRules("Find all gaps in the whole repo and align docs with code.")
	if got.Intent != ProfileIntentPlanOnly {
		t.Fatalf("intent = %q, want plan_only", got.Intent)
	}
}

func TestFusedDirectCodeIntent(t *testing.T) {
	if !FusedDirectCodeIntent("Please refactor the error handling in chat_wiring.go") {
		t.Fatal("expected true")
	}
	if FusedDirectCodeIntent("What is Go?") {
		t.Fatal("expected false")
	}
}
