package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseModeUnknownReturnsError(t *testing.T) {
	_, err := ParseMode("nope")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nope")
}

func TestApplyConfigModesRejectsInvalidToolMode(t *testing.T) {
	p := NewPolicy()
	err := p.ApplyConfigModes(map[string]string{"bash": "super_allow"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "bash")
}
