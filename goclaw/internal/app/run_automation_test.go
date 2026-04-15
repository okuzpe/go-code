package app

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/cli"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func testRootForAutomation(t *testing.T) *cobra.Command {
	t.Helper()
	return cli.NewRootCmd("dev",
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command) error { return nil },
		func(*cobra.Command, []string) error { return nil },
	)
}

func TestAutomationUsesJSON(t *testing.T) {
	t.Parallel()

	t.Run("json_output_flag", func(t *testing.T) {
		t.Parallel()
		root := testRootForAutomation(t)
		require.NoError(t, root.ParseFlags([]string{"--json-output"}))
		v, err := automationUsesJSON(root)
		require.NoError(t, err)
		require.True(t, v)
	})

	t.Run("output_format_json", func(t *testing.T) {
		t.Parallel()
		root := testRootForAutomation(t)
		require.NoError(t, root.ParseFlags([]string{"--output-format", "json"}))
		v, err := automationUsesJSON(root)
		require.NoError(t, err)
		require.True(t, v)
	})

	t.Run("output_format_text", func(t *testing.T) {
		t.Parallel()
		root := testRootForAutomation(t)
		require.NoError(t, root.ParseFlags([]string{"--output-format", "text"}))
		v, err := automationUsesJSON(root)
		require.NoError(t, err)
		require.False(t, v)
	})

	t.Run("json_output_overrides_text_format", func(t *testing.T) {
		t.Parallel()
		root := testRootForAutomation(t)
		require.NoError(t, root.ParseFlags([]string{"--output-format", "text", "--json-output"}))
		v, err := automationUsesJSON(root)
		require.NoError(t, err)
		require.True(t, v)
	})

	t.Run("invalid_output_format", func(t *testing.T) {
		t.Parallel()
		root := testRootForAutomation(t)
		require.NoError(t, root.ParseFlags([]string{"--output-format", "yaml"}))
		_, err := automationUsesJSON(root)
		require.Error(t, err)
	})
}
