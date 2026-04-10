package config

import (
	"testing"

	"github.com/charmbracelet/glamour"
	"github.com/stretchr/testify/require"
)

func TestGlamourTermRendererOptions_buildsRenderer(t *testing.T) {
	for _, raw := range []string{
		"",
		"auto",
		"dark",
		"light",
		"dark_colorblind",
		"light_colorblind",
		"dark_ansi",
		"light_ansi",
	} {
		t.Run(raw, func(t *testing.T) {
			opts := GlamourTermRendererOptions(raw, 72)
			require.GreaterOrEqual(t, len(opts), 2)
			_, err := glamour.NewTermRenderer(opts...)
			require.NoError(t, err)
		})
	}
}
