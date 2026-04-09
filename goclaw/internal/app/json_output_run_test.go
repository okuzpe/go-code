package app

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadSingleLineStdin(t *testing.T) {
	t.Parallel()
	s, err := readSingleLineStdin(strings.NewReader("hello world\n"))
	require.NoError(t, err)
	require.Equal(t, "hello world", s)
}

func TestReadSingleLineStdinEOFNoNewline(t *testing.T) {
	t.Parallel()
	s, err := readSingleLineStdin(strings.NewReader("only"))
	require.NoError(t, err)
	require.Equal(t, "only", s)
}

func TestAutomationOutputToolApprover(t *testing.T) {
	t.Parallel()
	_, err := automationOutputToolApprover(context.Background(), "bash", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "automation output")
}
