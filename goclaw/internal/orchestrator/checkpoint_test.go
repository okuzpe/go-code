package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/stretchr/testify/require"
)

func TestUndoLastCheckpointRestoresEditedFile(t *testing.T) {
	workdir := t.TempDir()
	launchDir := workdir
	scratch := filepath.Join(t.TempDir(), "scratch")
	require.NoError(t, os.MkdirAll(scratch, 0o700))
	target := filepath.Join(workdir, "demo.txt")
	require.NoError(t, os.WriteFile(target, []byte("before"), 0o600))

	reg := tools.New()
	reg.Register(tools.NewWriteFileScope(tools.PathScope{Root: workdir, RelativeBase: launchDir}))
	pol := permissions.NewPolicy()
	pol.Set("write_file", permissions.ModeAllow)
	orch := New(config.Default(), nil, session.New(), reg, pol, hooks.New(), agents.GeneralPurpose,
		WithWorkdir(workdir),
		WithLaunchDir(launchDir),
		WithScratchDir(scratch),
	)

	out := orch.executeTool(context.Background(), &llm.ToolUse{
		Name:  "write_file",
		Input: `{"path":"demo.txt","content":"after"}`,
	}, nil)
	require.False(t, out.IsError)

	body, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "after", string(body))

	msg, err := orch.UndoLastCheckpoint()
	require.NoError(t, err)
	require.Contains(t, msg, "write_file")

	body, err = os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "before", string(body))
}

func TestUndoLastCheckpointRemovesNewFile(t *testing.T) {
	workdir := t.TempDir()
	launchDir := workdir
	scratch := filepath.Join(t.TempDir(), "scratch")
	require.NoError(t, os.MkdirAll(scratch, 0o700))

	reg := tools.New()
	reg.Register(tools.NewWriteFileScope(tools.PathScope{Root: workdir, RelativeBase: launchDir}))
	pol := permissions.NewPolicy()
	pol.Set("write_file", permissions.ModeAllow)
	orch := New(config.Default(), nil, session.New(), reg, pol, hooks.New(), agents.GeneralPurpose,
		WithWorkdir(workdir),
		WithLaunchDir(launchDir),
		WithScratchDir(scratch),
	)

	out := orch.executeTool(context.Background(), &llm.ToolUse{
		Name:  "write_file",
		Input: `{"path":"new.txt","content":"fresh"}`,
	}, nil)
	require.False(t, out.IsError)

	_, err := os.Stat(filepath.Join(workdir, "new.txt"))
	require.NoError(t, err)

	_, err = orch.UndoLastCheckpoint()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(workdir, "new.txt"))
	require.True(t, os.IsNotExist(err))
}

func TestUndoLastCheckpointRestoresWriteFilesBatch(t *testing.T) {
	workdir := t.TempDir()
	launchDir := workdir
	scratch := filepath.Join(t.TempDir(), "scratch")
	require.NoError(t, os.MkdirAll(scratch, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "a.txt"), []byte("old-a"), 0o600))

	reg := tools.New()
	reg.Register(tools.NewWriteFilesScope(tools.PathScope{Root: workdir, RelativeBase: launchDir}))
	pol := permissions.NewPolicy()
	pol.Set("write_files", permissions.ModeAllow)
	orch := New(config.Default(), nil, session.New(), reg, pol, hooks.New(), agents.GeneralPurpose,
		WithWorkdir(workdir),
		WithLaunchDir(launchDir),
		WithScratchDir(scratch),
	)

	out := orch.executeTool(context.Background(), &llm.ToolUse{
		Name: "write_files",
		Input: `{"files":[` +
			`{"path":"a.txt","content":"new-a"},` +
			`{"path":"b.txt","content":"new-b"}` +
			`]}`,
	}, nil)
	require.False(t, out.IsError)

	_, err := orch.UndoLastCheckpoint()
	require.NoError(t, err)

	body, err := os.ReadFile(filepath.Join(workdir, "a.txt"))
	require.NoError(t, err)
	require.Equal(t, "old-a", string(body))
	_, err = os.Stat(filepath.Join(workdir, "b.txt"))
	require.True(t, os.IsNotExist(err))
}
