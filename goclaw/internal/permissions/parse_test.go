package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		in   string
		want Mode
	}{
		{"allow", ModeAllow},
		{"ASK", ModeAsk},
		{"  deny ", ModeDeny},
	}
	for _, tt := range tests {
		got, err := ParseMode(tt.in)
		require.NoError(t, err)
		require.Equal(t, tt.want, got)
	}
	_, err := ParseMode("nope")
	require.Error(t, err)
}
