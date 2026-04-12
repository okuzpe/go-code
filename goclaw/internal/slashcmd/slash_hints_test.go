package slashcmd

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFormatSessionModAge(t *testing.T) {
	require.Equal(t, "unknown", formatSessionModAge(time.Time{}))
	require.Contains(t, formatSessionModAge(time.Now().Add(-2*time.Minute)), "minute")
	require.Contains(t, formatSessionModAge(time.Now().Add(-3*time.Hour)), "hour")
	require.Contains(t, formatSessionModAge(time.Now().Add(-40*time.Hour)), "hour")
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	got := formatSessionModAge(old)
	require.True(t, strings.Contains(got, "2020") || strings.Contains(got, "T"))
}
