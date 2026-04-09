package llm

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrapOllamaDialErr_connectionRefused(t *testing.T) {
	err := &net.OpError{Err: syscall.ECONNREFUSED}
	w := wrapOllamaDialErr("http://127.0.0.1:11434", err)
	require.Error(t, w)
	require.Contains(t, w.Error(), "Is Ollama running")
	require.Contains(t, w.Error(), "ollama serve")
}

func TestWrapOllamaDialErr_connectionRefusedString(t *testing.T) {
	w := wrapOllamaDialErr("http://127.0.0.1:11434", errors.New("dial tcp: connection refused"))
	require.Contains(t, w.Error(), "Is Ollama running")
}

func TestWrapOllamaDialErr_timeout(t *testing.T) {
	w := wrapOllamaDialErr("http://127.0.0.1:11434", timeoutStub{})
	require.Contains(t, w.Error(), "did not respond in time")
}

func TestWrapOllamaDialErr_noSuchHost(t *testing.T) {
	w := wrapOllamaDialErr("http://bogus.invalid.example:11434", errors.New("lookup bogus.invalid.example: no such host"))
	require.Contains(t, w.Error(), "could not be resolved")
}

func TestWrapOllamaDialErr_connectex(t *testing.T) {
	w := wrapOllamaDialErr("http://127.0.0.1:11434", errors.New("connectex: No connection could be made because the target machine actively refused it"))
	require.Contains(t, w.Error(), "goclaw doctor")
}

func TestWrapOllamaDialErr_preservesCancel(t *testing.T) {
	ctxErr := context.Canceled
	w := wrapOllamaDialErr("http://x", ctxErr)
	require.ErrorIs(t, w, context.Canceled)
}

type timeoutStub struct{}

func (timeoutStub) Error() string   { return "i/o timeout" }
func (timeoutStub) Timeout() bool   { return true }
func (timeoutStub) Temporary() bool { return false }
