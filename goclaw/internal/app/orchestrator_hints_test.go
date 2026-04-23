package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/stretchr/testify/require"
)

func TestAugmentOrchestratorErr(t *testing.T) {
	inner := errors.New("ollama: cannot reach Ollama at x (connection refused): x")
	tests := []struct {
		name   string
		msg    string
		err    error
		assert func(t *testing.T, got error)
	}{
		{
			name: "nil error stays nil",
			msg:  "m",
			err:  nil,
			assert: func(t *testing.T, got error) {
				require.NoError(t, got)
			},
		},
		{
			name: "wrap preserves errors.Is for inner sentinel",
			msg:  "m",
			err:  inner,
			assert: func(t *testing.T, got error) {
				require.ErrorIs(t, got, inner)
			},
		},
		{
			name: "wrap preserves context.Canceled",
			msg:  "m",
			err:  context.Canceled,
			assert: func(t *testing.T, got error) {
				require.ErrorIs(t, got, context.Canceled)
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AugmentOrchestratorErr(tt.msg, tt.err)
			tt.assert(t, got)
		})
	}
}

func TestOrchestratorFailureHints(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		err         error
		wantContain string
	}{
		{
			name:        "ollama dial failure suggests serve",
			model:       "qwen2.5-coder:7b",
			err:         errors.New("cannot reach Ollama at http://localhost:11434 (connection refused): x"),
			wantContain: "ollama serve",
		},
		{
			name:        "ollama 404 suggests pull with model id",
			model:       "my-model",
			err:         errors.New("ollama 404: model not found"),
			wantContain: "ollama pull my-model",
		},
		{
			name:        "iteration limit suggests compact",
			model:       "m",
			err:         errors.New("iteration limit (32) reached"),
			wantContain: "/compact",
		},
		{
			name:        "action stalled suggests stronger model or doctor",
			model:       "m",
			err:         orchestrator.ErrActionStalled,
			wantContain: "failed to take real tool-driven action",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := orchestratorFailureHints(tt.model, tt.err)
			require.NotEmpty(t, h)
			require.Contains(t, strings.Join(h, "\n"), tt.wantContain)
		})
	}
}

func TestDedupeHintLines(t *testing.T) {
	t.Parallel()
	got := dedupeHintLines([]string{"hint: a", "hint: a", "hint: b"})
	require.Len(t, got, 2)
}
