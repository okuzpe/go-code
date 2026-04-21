package orchestrator

import (
	"path/filepath"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/stretchr/testify/require"
)

func TestWriteTargetsUnderScratch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	scratch, err := filepath.Abs(filepath.Join(dir, "scratch"))
	require.NoError(t, err)
	cfg := config.Default()
	orch := New(cfg, nil, session.New(), tools.New(), permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose,
		WithWorkdir(dir),
		WithLaunchDir(dir),
		WithScratchDir(scratch),
	)

	noteRel := filepath.ToSlash(filepath.Join("scratch", "note.txt"))
	tests := []struct {
		name     string
		toolName string
		input    string
		want     bool
	}{
		{
			name:     "write_file relative path under scratch",
			toolName: "write_file",
			input:    `{"path":"` + noteRel + `","content":"x"}`,
			want:     true,
		},
		{
			name:     "write_file rejects path outside scratch tree",
			toolName: "write_file",
			input:    `{"path":"../outside.txt","content":"x"}`,
			want:     false,
		},
		{
			name:     "edit_file allows target under scratch",
			toolName: "edit_file",
			input:    `{"path":"` + noteRel + `","old_string":"a","new_string":"b"}`,
			want:     true,
		},
		{
			name:     "unknown tool is never auto-approved for scratch",
			toolName: "bash",
			input:    `{"command":"echo"}`,
			want:     false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, orch.writeTargetsUnderScratch(tt.toolName, tt.input))
		})
	}
}
