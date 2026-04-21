package orchestrator

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/llm"
)

// turnEndSummary is a single structured snapshot at end of runUserTurn (debug/support).
type turnEndSummary struct {
	Status                     string
	ProfileName                string
	TurnModel                  string
	TaskRole                   string
	WorkspaceWriteOK           bool
	OllamaFunctionToolsDropped bool
	ActionNudges               int
	VerifyNudges               int
	RepairEscalations          int
	ReflectionNudge            bool
	EditFileNotFoundNudges     int
	ToolCalls                  int
	HadToolRound               bool
	TranslationMs              int64
	StreamMs                   int64
	ToolMs                     int64
	TurnMs                     int64
}

func ollamaClientFunctionToolsDropped(client llm.Client) bool {
	oc, ok := client.(*llm.OllamaClient)
	if !ok {
		return false
	}
	return oc.FunctionToolsDropped()
}

func (o *Orchestrator) logTurnEndSummary(ctx context.Context, summary turnEndSummary) {
	if o == nil {
		return
	}
	if !slog.Default().Handler().Enabled(ctx, slog.LevelDebug) {
		return
	}
	tm := strings.TrimSpace(summary.TurnModel)
	if tm == "" {
		tm = "(default)"
	}
	tr := strings.TrimSpace(summary.TaskRole)
	if tr == "" {
		tr = "(none)"
	}
	slog.DebugContext(ctx, "turn summary",
		"status", summary.Status,
		"profile", summary.ProfileName,
		"turn_model", tm,
		"task_role", tr,
		"workspace_write_ok", summary.WorkspaceWriteOK,
		"ollama_function_tools_dropped", summary.OllamaFunctionToolsDropped,
		"action_nudges", summary.ActionNudges,
		"verify_nudges", summary.VerifyNudges,
		"repair_escalations", summary.RepairEscalations,
		"reflection_nudge", summary.ReflectionNudge,
		"edit_file_not_found_nudges", summary.EditFileNotFoundNudges,
		"tool_calls", summary.ToolCalls,
		"had_tool_round", summary.HadToolRound,
		"translation_ms", summary.TranslationMs,
		"stream_ms", summary.StreamMs,
		"tool_ms", summary.ToolMs,
		"turn_ms", summary.TurnMs,
	)
}

// emitTurnEndSummary logs the consolidated turn summary at debug (reads verify nudge count from ut).
func (o *Orchestrator) emitTurnEndSummary(
	ctx context.Context,
	status string,
	metrics turnMetrics,
	toolCalls int,
	workspaceWriteOK bool,
	hadToolRound bool,
	actionNudges, repairEscalations, editFileNotFoundNudges int,
	reflectionFired bool,
) {
	verifyN := 0
	if o != nil && o.ut != nil {
		verifyN = o.ut.verifyNudges
	}
	o.logTurnEndSummary(ctx, o.buildTurnEndSummary(status, metrics, toolCalls, workspaceWriteOK, hadToolRound, actionNudges, verifyN, repairEscalations, editFileNotFoundNudges, reflectionFired))
}

func (o *Orchestrator) buildTurnEndSummary(
	status string,
	metrics turnMetrics,
	toolCalls int,
	workspaceWriteOK bool,
	hadToolRound bool,
	actionNudges, verifyNudges, repairEscalations, editFileNotFoundNudges int,
	reflectionFired bool,
) turnEndSummary {
	s := turnEndSummary{
		Status:                 status,
		ProfileName:            o.profile.Name,
		WorkspaceWriteOK:       workspaceWriteOK,
		OllamaFunctionToolsDropped: ollamaClientFunctionToolsDropped(o.llm),
		ActionNudges:           actionNudges,
		VerifyNudges:           verifyNudges,
		RepairEscalations:      repairEscalations,
		ReflectionNudge:      reflectionFired,
		EditFileNotFoundNudges: editFileNotFoundNudges,
		ToolCalls:              toolCalls,
		HadToolRound:           hadToolRound,
		TranslationMs:          metrics.translation.Milliseconds(),
		StreamMs:               metrics.stream.Milliseconds(),
		ToolMs:                 metrics.tool.Milliseconds(),
		TurnMs:                 time.Since(metrics.start).Milliseconds(),
	}
	if o.ut != nil {
		s.TurnModel = o.ut.turnModel
		s.TaskRole = o.ut.taskRole
	}
	return s
}
