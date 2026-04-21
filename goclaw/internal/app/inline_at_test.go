package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandInlineAtRefsWithReader(t *testing.T) {
	t.Run("no at refs returns input", func(t *testing.T) {
		in := "hello world"
		got := expandInlineAtRefsWithReader(in, func(path string) (string, bool, error) {
			t.Fatalf("unexpected read for %q", path)
			return "", false, nil
		})
		require.Equal(t, in, got)
	})

	t.Run("successful preload injects context and body", func(t *testing.T) {
		in := "please review @go.mod now"
		got := expandInlineAtRefsWithReader(in, func(path string) (string, bool, error) {
			require.Equal(t, "go.mod", path)
			return "module demo\n", false, nil
		})
		require.Contains(t, got, "[Files loaded via @ references]")
		require.Contains(t, got, "@go.mod\n---\nmodule demo")
		require.Contains(t, got, "[End of @ context]")
		require.True(t, strings.HasSuffix(got, in))
	})

	t.Run("partial failure adds compact warning", func(t *testing.T) {
		in := "mix @ok and '@missing'"
		got := expandInlineAtRefsWithReader(in, func(path string) (string, bool, error) {
			switch path {
			case "ok":
				return "fine", false, nil
			case "missing":
				return "", false, errors.New("not found")
			default:
				t.Fatalf("unexpected path %q", path)
				return "", false, nil
			}
		})
		require.Contains(t, got, "! warning: could not preload @missing")
		require.Contains(t, got, "@ok\n---\nfine")
		require.True(t, strings.HasSuffix(got, in))
	})
}
