package orchestrator

import (
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/tools"
)

func testOrchestratorForTruthFooter(cfg config.Config, profile agents.Profile) *Orchestrator {
	reg := tools.New()
	reg.Register(fakeTool{name: "write_file"})
	return &Orchestrator{
		cfg:     cfg,
		tools:   reg,
		profile: profile,
	}
}

func TestRecordWorkspaceWriteFromResults(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ok := false
		recordWorkspaceWriteFromResults(&ok, nil)
		if ok {
			t.Fatal("expected false")
		}
	})
	t.Run("read_only", func(t *testing.T) {
		ok := false
		recordWorkspaceWriteFromResults(&ok, []llm.ToolResultRecord{
			{ToolName: "read_file", Content: "x", IsError: false},
		})
		if ok {
			t.Fatal("expected false")
		}
	})
	t.Run("write_error", func(t *testing.T) {
		ok := false
		recordWorkspaceWriteFromResults(&ok, []llm.ToolResultRecord{
			{ToolName: "write_file", Content: "err", IsError: true},
		})
		if ok {
			t.Fatal("expected false on error result")
		}
	})
	t.Run("write_ok", func(t *testing.T) {
		ok := false
		recordWorkspaceWriteFromResults(&ok, []llm.ToolResultRecord{
			{ToolName: "glob", Content: "{}", IsError: false},
			{ToolName: "edit_file", Content: "ok", IsError: false},
		})
		if !ok {
			t.Fatal("expected true after successful edit_file")
		}
	})
}

func TestMaybeAppendNoWorkspaceWriteFooter(t *testing.T) {
	base := "Summary only."
	userFix := "Please fix the nil pointer in bar.go"

	t.Run("nil orchestrator", func(t *testing.T) {
		if got := maybeAppendNoWorkspaceWriteFooter(nil, base, userFix, true, false); got != base {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("appends when eligible", func(t *testing.T) {
		cfg := config.Default()
		cfg.TruthFooterNoWorkspaceWrites = true
		o := testOrchestratorForTruthFooter(cfg, agents.GeneralPurpose)
		got := maybeAppendNoWorkspaceWriteFooter(o, base, userFix, true, false)
		if !strings.Contains(got, "[goclaw] No workspace files were modified this turn") {
			t.Fatalf("expected footer in %q", got)
		}
	})

	t.Run("disabled in config", func(t *testing.T) {
		cfg := config.Default()
		cfg.TruthFooterNoWorkspaceWrites = false
		o := testOrchestratorForTruthFooter(cfg, agents.GeneralPurpose)
		if got := maybeAppendNoWorkspaceWriteFooter(o, base, userFix, true, false); got != base {
			t.Fatalf("expected unchanged, got %q", got)
		}
	})

	t.Run("read_only profile", func(t *testing.T) {
		cfg := config.Default()
		o := testOrchestratorForTruthFooter(cfg, agents.Explore)
		if got := maybeAppendNoWorkspaceWriteFooter(o, base, userFix, true, false); got != base {
			t.Fatalf("expected unchanged, got %q", got)
		}
	})

	t.Run("no tool round", func(t *testing.T) {
		cfg := config.Default()
		o := testOrchestratorForTruthFooter(cfg, agents.GeneralPurpose)
		if got := maybeAppendNoWorkspaceWriteFooter(o, base, userFix, false, false); got != base {
			t.Fatalf("expected unchanged, got %q", got)
		}
	})

	t.Run("workspace write succeeded", func(t *testing.T) {
		cfg := config.Default()
		o := testOrchestratorForTruthFooter(cfg, agents.GeneralPurpose)
		if got := maybeAppendNoWorkspaceWriteFooter(o, base, userFix, true, true); got != base {
			t.Fatalf("expected unchanged, got %q", got)
		}
	})

	t.Run("user message no write intent", func(t *testing.T) {
		cfg := config.Default()
		o := testOrchestratorForTruthFooter(cfg, agents.GeneralPurpose)
		if got := maybeAppendNoWorkspaceWriteFooter(o, base, "Explain how the orchestrator works", true, false); got != base {
			t.Fatalf("expected unchanged, got %q", got)
		}
	})

	t.Run("effective specs omit write tools", func(t *testing.T) {
		cfg := config.Default()
		o := testOrchestratorForTruthFooter(cfg, agents.CodeReview)
		if got := maybeAppendNoWorkspaceWriteFooter(o, base, userFix, true, false); got != base {
			t.Fatalf("expected unchanged, got %q", got)
		}
	})

	t.Run("idempotent when marker present", func(t *testing.T) {
		cfg := config.Default()
		o := testOrchestratorForTruthFooter(cfg, agents.GeneralPurpose)
		once := maybeAppendNoWorkspaceWriteFooter(o, base, userFix, true, false)
		twice := maybeAppendNoWorkspaceWriteFooter(o, once, userFix, true, false)
		if strings.Count(twice, "No workspace files were modified this turn") != 1 {
			t.Fatalf("expected single marker, got %q", twice)
		}
	})
}
