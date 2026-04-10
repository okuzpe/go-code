package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatchModifyFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("line1\nline2\nline3\n"), 0o644))

	diff := "" +
		"diff --git a/a.txt b/a.txt\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,3 +1,3 @@\n" +
		" line1\n" +
		"-line2\n" +
		"+line2edited\n" +
		" line3\n"
	payload, err := json.Marshal(map[string]string{"path": "a.txt", "diff": diff})
	require.NoError(t, err)
	var round map[string]string
	require.NoError(t, json.Unmarshal(payload, &round))
	require.Equal(t, diff, round["diff"], "json round-trip must preserve diff text")

	tool := NewPatch(dir)
	res, err := tool.Execute(context.Background(), string(payload))
	require.NoError(t, err)
	require.False(t, res.IsError, res.Content)
	raw, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	require.NoError(t, err)
	require.Equal(t, "line1\nline2edited\nline3\n", string(raw))
}

func TestPatchCreateFile(t *testing.T) {
	dir := t.TempDir()
	diff := `diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..9daeafb
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+hello
+world
`
	payload, err := json.Marshal(map[string]string{"path": "new.txt", "diff": diff})
	require.NoError(t, err)
	tool := NewPatch(dir)
	res, err := tool.Execute(context.Background(), string(payload))
	require.NoError(t, err)
	require.False(t, res.IsError, res.Content)
	raw, err := os.ReadFile(filepath.Join(dir, "new.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello\nworld\n", string(raw))
}

func TestPatchPathMismatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644))
	diff := `diff --git a/other.txt b/other.txt
--- a/other.txt
+++ b/other.txt
@@ -1 +1 @@
-x
+y
`
	payload, err := json.Marshal(map[string]string{"path": "a.txt", "diff": diff})
	require.NoError(t, err)
	tool := NewPatch(dir)
	res, err := tool.Execute(context.Background(), string(payload))
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestPatchMultiFileRejected(t *testing.T) {
	dir := t.TempDir()
	diff := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1 +1 @@
-a
+b
diff --git a/b.txt b/b.txt
--- a/b.txt
+++ b/b.txt
@@ -1 +1 @@
-c
+d
`
	payload, err := json.Marshal(map[string]string{"path": "a.txt", "diff": diff})
	require.NoError(t, err)
	tool := NewPatch(dir)
	res, err := tool.Execute(context.Background(), string(payload))
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestPatchDeleteFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gone.txt"), []byte("only\n"), 0o644))
	diff := `diff --git a/gone.txt b/gone.txt
deleted file mode 100644
--- a/gone.txt
+++ /dev/null
@@ -1 +0,0 @@
-only
`
	payload, err := json.Marshal(map[string]string{"path": "gone.txt", "diff": diff})
	require.NoError(t, err)
	tool := NewPatch(dir)
	res, err := tool.Execute(context.Background(), string(payload))
	require.NoError(t, err)
	require.False(t, res.IsError, res.Content)
	_, statErr := os.Stat(filepath.Join(dir, "gone.txt"))
	require.True(t, os.IsNotExist(statErr))
}
