package inputprefix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTryPasteAsAtPaths_singleFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("x"), 0o600))

	got, ok := TryPasteAsAtPaths(root, filepath.Join(root, "go.mod"))
	require.True(t, ok)
	require.Equal(t, "@go.mod", got)
}

func TestTryPasteAsAtPaths_directory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal"), 0o700))

	got, ok := TryPasteAsAtPaths(root, filepath.Join(root, "internal"))
	require.True(t, ok)
	require.Equal(t, "@internal/", got)
}

func TestTryPasteAsAtPaths_quotedMultiple(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("x"), 0o600))

	paste := `"` + filepath.Join(root, "a.go") + `" "` + filepath.Join(root, "b.go") + `"`
	got, ok := TryPasteAsAtPaths(root, paste)
	require.True(t, ok)
	require.Equal(t, "@a.go @b.go", got)
}

func TestTryPasteAsAtPaths_newlineSeparated(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "x.go"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "y.go"), []byte("x"), 0o600))

	paste := filepath.Join(root, "x.go") + "\n" + filepath.Join(root, "y.go")
	got, ok := TryPasteAsAtPaths(root, paste)
	require.True(t, ok)
	require.Equal(t, "@x.go @y.go", got)
}

func TestTryPasteAsAtPaths_regularText(t *testing.T) {
	root := t.TempDir()
	_, ok := TryPasteAsAtPaths(root, "hello world")
	require.False(t, ok)
}

func TestTryPasteAsAtPaths_outsideWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o600))

	_, ok := TryPasteAsAtPaths(root, filepath.Join(outside, "secret.txt"))
	require.False(t, ok)
}

func TestTryPasteAsAtPaths_nonExistentPath(t *testing.T) {
	root := t.TempDir()
	_, ok := TryPasteAsAtPaths(root, filepath.Join(root, "nope.go"))
	require.False(t, ok)
}

func TestTryPasteAsAtPaths_fileURL(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "from_uri.go")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
	abs, err := filepath.Abs(f)
	require.NoError(t, err)
	var uri string
	if len(abs) > 0 && abs[0] == '/' {
		uri = "file://" + filepath.ToSlash(abs)
	} else {
		uri = "file:///" + filepath.ToSlash(abs)
	}
	got, ok := TryPasteAsAtPaths(root, uri)
	require.True(t, ok, "uri=%q abs=%q", uri, abs)
	require.Equal(t, "@from_uri.go", got)
}

func TestTryPasteAsAtPaths_relativeUnderWorkspace(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "pkg", "rel.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(sub), 0o700))
	require.NoError(t, os.WriteFile(sub, []byte("x"), 0o600))
	got, ok := TryPasteAsAtPaths(root, filepath.ToSlash(filepath.Join("pkg", "rel.go")))
	require.True(t, ok)
	require.Equal(t, "@pkg/rel.go", got)
}
