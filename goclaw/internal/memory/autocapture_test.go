package memory

import (
	"fmt"
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMaybeAutoCaptureFromTool(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	cfg := config.Config{MemoryAutoExtract: true}
	MaybeAutoCaptureFromTool(cfg, st, t.Name(), "write_file", `{"path":"foo/bar.go","content":"x"}`, false)
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list: %+v", list)
	}
	if list[0].Type != TypeProject {
		t.Fatalf("type %s", list[0].Type)
	}
}

func TestMaybeAutoCaptureFromTool_NoOpWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	cfg := config.Config{MemoryAutoExtract: false}
	MaybeAutoCaptureFromTool(cfg, st, t.Name(), "write_file", `{"path":"x"}`, false)
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no entries")
	}
}

func TestMaybeAutoCaptureFromTool_SignalFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st := New(dir)
	cfg := config.Config{MemoryAutoExtract: true}

	tests := []struct {
		name    string
		tool    string
		input   string
		wantLen int
	}{
		{
			name:    "write_file empty content skipped",
			tool:    "write_file",
			input:   `{"path":"a.txt","content":"   "}`,
			wantLen: 0,
		},
		{
			name:    "edit_file unchanged skipped",
			tool:    "edit_file",
			input:   `{"path":"a.txt","old_string":"same","new_string":"same"}`,
			wantLen: 0,
		},
		{
			name:    "patch empty diff skipped",
			tool:    "patch",
			input:   `{"path":"a.txt","diff":"\n\t "}`,
			wantLen: 0,
		},
		{
			name:    "edit_file with change captured",
			tool:    "edit_file",
			input:   `{"path":"a.txt","old_string":"old","new_string":"new"}`,
			wantLen: 1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			sessionID := fmt.Sprintf("%s-%s", t.Name(), testCase.name)
			MaybeAutoCaptureFromTool(cfg, st, sessionID, testCase.tool, testCase.input, false)
			list, err := st.List()
			require.NoError(t, err)
			require.Len(t, list, testCase.wantLen)
			// cleanup between cases for deterministic length assertions
			for _, entry := range list {
				require.NoError(t, st.Delete(entry.Filename))
			}
		})
	}
}

func TestMaybeAutoCaptureFromTool_DedupByPayloadSignature(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st := New(dir)
	cfg := config.Config{MemoryAutoExtract: true}
	sessionID := t.Name()

	MaybeAutoCaptureFromTool(cfg, st, sessionID, "edit_file", `{"path":"a.txt","old_string":"x","new_string":"y"}`, false)
	MaybeAutoCaptureFromTool(cfg, st, sessionID, "edit_file", `{"path":"a.txt","old_string":"x","new_string":"y"}`, false)
	MaybeAutoCaptureFromTool(cfg, st, sessionID, "edit_file", `{"path":"a.txt","old_string":"x","new_string":"yz"}`, false)

	list, err := st.List()
	require.NoError(t, err)
	require.Len(t, list, 2)
}
