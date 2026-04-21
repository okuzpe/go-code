package chat

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/ui/icons"
	"github.com/stretchr/testify/require"
)

func TestAgentLaneHint(t *testing.T) {
	t.Parallel()
	require.Equal(t, "hub", agentLaneHint("coordinator"))
	require.Equal(t, "draft", agentLaneHint("plan"))
	require.Equal(t, "", agentLaneHint("custom-unknown-profile"))
}

func TestDeckBrackets(t *testing.T) {
	t.Parallel()
	o, c := deckBrackets(icons.ASCII)
	require.Equal(t, "<", o)
	require.Equal(t, ">", c)
	o2, c2 := deckBrackets(icons.Unicode)
	require.NotEqual(t, o2, "<")
	require.NotEmpty(t, c2)
}
