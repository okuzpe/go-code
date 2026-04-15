package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestListSessionsDetailed_previewAndModTime(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	require.NoError(t, err)

	id := "abc-test-session"
	path := filepath.Join(dir, id+".jsonl")
	body := `{"role":"user","content":"hello world line"}
{"role":"assistant","content":"hi"}
`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	before := time.Now().Add(-2 * time.Second)

	rows, err := st.ListSessionsDetailed()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, id, rows[0].ID)
	require.Contains(t, rows[0].PreviewText, "hello world line")
	require.True(t, !rows[0].ModTime.Before(before))

	line := FormatSessionListTSVLine(rows[0])
	require.Contains(t, line, id)
	require.Contains(t, line, "hello world line")
}
