package ide

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromEnv_noEnvVar(t *testing.T) {
	t.Setenv("GOCLAW_IDE_NOTIFY_URL", "")
	n := FromEnv()
	_, ok := n.(Noop)
	require.True(t, ok)
}

func TestFromEnv_remoteURL(t *testing.T) {
	t.Setenv("GOCLAW_IDE_NOTIFY_URL", "http://example.com/notify")
	n := FromEnv()
	_, ok := n.(Noop)
	require.True(t, ok, "remote host must return Noop")
}

func TestFromEnv_loopbackURL(t *testing.T) {
	t.Setenv("GOCLAW_IDE_NOTIFY_URL", "http://127.0.0.1:9999/notify")
	n := FromEnv()
	_, ok := n.(*localHTTPNotify)
	require.True(t, ok)
}

func TestFromEnv_localhostURL(t *testing.T) {
	t.Setenv("GOCLAW_IDE_NOTIFY_URL", "http://localhost:9999/notify")
	n := FromEnv()
	_, ok := n.(*localHTTPNotify)
	require.True(t, ok)
}

func TestFromEnv_invalidURL(t *testing.T) {
	t.Setenv("GOCLAW_IDE_NOTIFY_URL", "://bad-url")
	n := FromEnv()
	_, ok := n.(Noop)
	require.True(t, ok)
}

func TestNoop_afterTool(t *testing.T) {
	// Noop.AfterTool must not panic or error.
	var n Noop
	n.AfterTool("bash", 100, false)
}

func TestLocalHTTPNotify_afterTool(t *testing.T) {
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &localHTTPNotify{url: srv.URL, client: srv.Client()}
	n.AfterTool("read_file", 42, false)

	require.Contains(t, received, "read_file")
	require.Contains(t, received, "42")
	require.Contains(t, received, "false")
}

func TestLocalHTTPNotify_afterTool_isError(t *testing.T) {
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &localHTTPNotify{url: srv.URL, client: srv.Client()}
	n.AfterTool("bash", 0, true)
	require.True(t, strings.Contains(received, "true"))
}

func TestLocalHTTPNotify_unreachableServer(t *testing.T) {
	// Must not panic even if the server is not reachable.
	n := &localHTTPNotify{url: "http://127.0.0.1:1/notify", client: &http.Client{}}
	n.AfterTool("bash", 10, false) // best-effort, no error returned
}
