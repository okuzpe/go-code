package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalToolInputJSON(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		var in struct {
			Pattern string `json:"pattern"`
		}
		require.NoError(t, UnmarshalToolInputJSON(`{"pattern":"*.go"}`, &in))
		require.Equal(t, "*.go", in.Pattern)
	})

	t.Run("invalid_json", func(t *testing.T) {
		var in struct{}
		err := UnmarshalToolInputJSON(`{`, &in)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid json input")
	})
}
