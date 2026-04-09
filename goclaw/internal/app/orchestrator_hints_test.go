package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAugmentOrchestratorErr_nil(t *testing.T) {
	require.NoError(t, AugmentOrchestratorErr("ollama", "m", nil))
}

func TestAugmentOrchestratorErr_preservesErrorsIs(t *testing.T) {
	inner := errors.New("ollama: cannot reach Ollama at x (connection refused): x")
	aug := AugmentOrchestratorErr("ollama", "m", inner)
	require.ErrorIs(t, aug, inner)
}

func TestAugmentOrchestratorErr_preservesContextCanceled(t *testing.T) {
	w := AugmentOrchestratorErr("ollama", "m", context.Canceled)
	require.ErrorIs(t, w, context.Canceled)
}

func TestOrchestratorFailureHints_ollamaDial(t *testing.T) {
	h := orchestratorFailureHints("ollama", "qwen2.5-coder:7b", errors.New("cannot reach Ollama at http://localhost:11434 (connection refused): x"))
	require.Len(t, h, 1)
	require.Contains(t, h[0], "ollama serve")
}

func TestOrchestratorFailureHints_ollama404(t *testing.T) {
	h := orchestratorFailureHints("ollama", "my-model", errors.New("ollama 404: model not found"))
	require.Len(t, h, 1)
	require.Contains(t, h[0], "ollama pull my-model")
}

func TestOrchestratorFailureHints_anthropic401(t *testing.T) {
	h := orchestratorFailureHints("anthropic", "", errors.New("anthropic 401: invalid api key"))
	require.Len(t, h, 1)
	require.Contains(t, h[0], "ANTHROPIC_API_KEY")
}

func TestOrchestratorFailureHints_iterationLimit(t *testing.T) {
	h := orchestratorFailureHints("ollama", "m", errors.New("iteration limit (32) reached"))
	require.Len(t, h, 1)
	require.Contains(t, h[0], "/compact")
}

func TestDedupeHintLines(t *testing.T) {
	got := dedupeHintLines([]string{"hint: a", "hint: a", "hint: b"})
	require.Len(t, got, 2)
}
