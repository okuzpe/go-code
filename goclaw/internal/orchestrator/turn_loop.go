package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/toolpolicy"
)

// runUserTurn drives one user message through the LLM stream, tool execution, and synthetic nudges.
func (o *Orchestrator) runUserTurn(ctx context.Context, userMessage string, sink StreamSink, toolTrace *[]JSONToolCall) (string, error) {
	if o.session == nil {
		return "", fmt.Errorf("orchestrator: session is required")
	}
	defer o.session.ClearStreamingAssistant()

	// Detect the user's original language BEFORE any translation so the reply-language hint in
	// buildRequest can instruct the model to respond in the right language even after the message
	// has been normalized to English. turnInputLang is cleared in the defer below.
	metrics := turnMetrics{start: time.Now()}

	intentMessage := routingUserMessage(userMessage)
	origLang := classifyUserLanguage(intentMessage)
	if origLang == "" {
		origLang = whatlanggoToTag(intentMessage)
	}

	o.ut = &userTurnState{
		turnToolCache:    make(map[string]string),
		tuiInteractApply: o.nextTUIInteractApply,
		tuiInteractMode:  o.nextTUIInteractMode,
	}
	o.nextTUIInteractApply = false
	o.nextTUIInteractMode = ""
	o.ut.turnInputLang = origLang
	defer func() { o.ut = nil }()

	// Translate non-English input to English before the LLM sees it (opt-in or default-on).
	// The language-reply hint uses turnInputLang (the original) so the model answers in the
	// user's language even though the session now holds the English translation.
	if o.cfg.NormalizeInputLanguage && origLang != "" && origLang != "en" {
		translateStart := time.Now()
		userMessage = normalizeInputToEnglish(ctx, o.llm, o.cfg.ModelForCompaction(), userMessage)
		metrics.translation = time.Since(translateStart)
	}

	o.session.Add("user", userMessage)

	o.prepareTurnModel(ctx, intentMessage)

	toolCalls := 0
	iterCap := o.effectiveMaxIterations()
	iterLimit := iterCap
	if o.profile.MaxTurns > 0 && o.profile.MaxTurns < iterCap {
		iterLimit = o.profile.MaxTurns
	}
	// Adaptive budget: lightweight roles (explore, fast) rarely need the full cap.
	if o.ut.taskRole != "" {
		iterLimit = adaptIterBudget(iterLimit, o.ut.taskRole)
	}
	// Chat mode keeps orchestration lighter than full agent loops.
	iterLimit = applyTUIChatIterationCap(o.cfg, o.ut.tuiInteractApply, o.ut.tuiInteractMode, iterLimit)
	o.ut.budgetLimit = iterLimit
	o.ut.turnUserMessage = intentMessage

	actionNudges := 0
	repairEscalations := 0
	editFileNotFoundNudges := 0
	hadToolRound := false
	lastBatchReadOnly := false
	workspaceWriteOK := false
	readOnlyToolRounds := 0  // consecutive tool rounds with no workspace write
	reflectionFired := false // at most one reflection nudge per turn

	for iter := range iterLimit {
		o.ut.budgetIter = iter + 1
		o.ut.budgetToolCalls = toolCalls
		o.ut.turnHadToolRound = hadToolRound
		o.ut.turnWorkspaceWriteOK = workspaceWriteOK
		o.maybeCompact(ctx, sink)

		if sink != nil {
			phase := ThinkingPhaseLine(iter, o.ut.taskRole, PhaseContext{
				HadToolRound:      hadToolRound,
				WorkspaceWriteOK:  workspaceWriteOK,
				LastBatchReadOnly: lastBatchReadOnly,
				BudgetLimit:       o.ut.budgetLimit,
			})
			sink.OnThinkingStart(phase)
		}

		streamStart := time.Now()
		events, errc := o.llm.Stream(ctx, o.buildRequest())

		var response string
		var pendingTools []llm.ToolUse

		for e := range events {
			switch ev := e.(type) {
			case llm.TextDelta:
				response += ev.Text
				if o.session != nil && ev.Text != "" {
					o.session.AddStreamingAssistantChars(len(ev.Text))
				}
				if sink != nil && ev.Text != "" {
					sink.OnTextDelta(ev.Text)
				}
			case llm.ToolUse:
				pendingTools = append(pendingTools, ev)
				slog.Debug("tool use requested",
					"id", ev.ID,
					"name", ev.Name,
					"preview", FormatToolUsePreview(ev.Name, ev.Input),
				)
				if sink != nil {
					sink.OnToolUse(ev.ID, ev.Name, ev.Input)
				}
			case llm.Done:
				// stream finished
			}
		}

		if err := <-errc; err != nil {
			o.emitTurnEndSummary(ctx, "llm_stream_error", metrics, toolCalls, workspaceWriteOK, hadToolRound, actionNudges, repairEscalations, editFileNotFoundNudges, reflectionFired)
			return "", fmt.Errorf("llm stream: %w", err)
		}
		metrics.stream += time.Since(streamStart)
		slog.Debug("llm stream done", "elapsed", time.Since(streamStart), "tools", len(pendingTools))

		if len(pendingTools) == 0 {
			response = sanitizeNarratedToolCallText(response)
			if nudgeMsg, ok := o.pickActionContinueNudge(intentMessage, toolCalls, lastBatchReadOnly, hadToolRound, actionNudges, response); ok {
				o.session.AddAssistant(response, nil)
				actionNudges++
				o.session.Add("user", nudgeMsg)
				slog.Debug("orchestrator: action-continue nudge", "nudge", actionNudges)
				continue
			}
			if o.tryActionRepairEscalation(intentMessage, toolCalls, hadToolRound, lastBatchReadOnly, actionNudges, &repairEscalations) {
				o.session.AddAssistant(response, nil)
				o.session.Add("user", actionRepairModelEscalationMessage)
				slog.Debug("orchestrator: action-repair model escalation")
				continue
			}
			if verifyGateShouldInjectNudge(o.cfg, o.ut, o.profile.ReadOnly) {
				o.session.AddAssistant(response, nil)
				verifyGateApplyNudge(o.ut)
				o.session.Add("user", verifyAfterWriteNudgeMessage)
				slog.Debug("orchestrator: verify-after-write nudge")
				continue
			}
			plain := response
			response = maybeAppendNoWorkspaceWriteFooter(o, response, intentMessage, hadToolRound, workspaceWriteOK)
			if sink != nil && len(response) > len(plain) {
				sink.OnTextDelta(response[len(plain):])
			}
			o.session.AddAssistant(response, nil)
			if sink != nil {
				sink.OnDone(response)
			}
			if toolCalls == 0 {
				extractModel := o.cfg.Model()
				if mo := strings.TrimSpace(o.profile.ModelOverride); mo != "" {
					extractModel = o.cfg.NormalizeModelForProvider(mo)
				} else if tm := strings.TrimSpace(o.ut.turnModel); tm != "" {
					extractModel = tm
				}
				memory.ScheduleSilentTurnLLMExtract(o.cfg, o.llm, o.mem, extractModel, intentMessage, response)
			}
			metrics.toolCalls = toolCalls
			o.emitTurnEndSummary(ctx, "success", metrics, toolCalls, workspaceWriteOK, hadToolRound, actionNudges, repairEscalations, editFileNotFoundNudges, reflectionFired)
			o.logTurnMetrics(metrics)
			return response, nil
		}

		lastBatchReadOnly = !toolUsesIncludeWorkspaceWrite(pendingTools)
		hadToolRound = true

		records := make([]llm.ToolCallRecord, len(pendingTools))
		for i, tu := range pendingTools {
			records[i] = llm.ToolCallRecord(tu)
		}
		o.session.AddAssistant(response, records)

		toolCap := o.effectiveMaxToolCalls()
		if toolCap-toolCalls < len(pendingTools) {
			o.emitTurnEndSummary(ctx, "tool_call_limit", metrics, toolCalls, workspaceWriteOK, hadToolRound, actionNudges, repairEscalations, editFileNotFoundNudges, reflectionFired)
			return "", fmt.Errorf("tool call limit (%d) reached", toolCap)
		}

		names := make([]string, len(pendingTools))
		for i, tu := range pendingTools {
			names[i] = tu.Name
		}
		parallel := len(pendingTools) > 1 &&
			len(pendingTools) <= o.cfg.EffectiveParallelToolBatchMax() &&
			o.allToolsAutoApprove(pendingTools) &&
			!toolpolicy.PendingToolsBlockParallel(names)
		var results []llm.ToolResultRecord

		if parallel {
			parallelStart := time.Now()
			var atomicCalls atomic.Int64
			var parallelErr error
			results, parallelErr = o.executeToolsParallel(ctx, pendingTools, &atomicCalls, sink)
			metrics.tool += time.Since(parallelStart)
			if parallelErr != nil {
				o.emitTurnEndSummary(ctx, "tool_parallel_error", metrics, toolCalls, workspaceWriteOK, hadToolRound, actionNudges, repairEscalations, editFileNotFoundNudges, reflectionFired)
				return "", parallelErr
			}
			toolCalls += int(atomicCalls.Load())
			for i, r := range results {
				input := ""
				if i < len(pendingTools) {
					input = pendingTools[i].Input
				}
				if o.afterTool != nil {
					o.afterTool(r.ToolName, input, len(r.Content), r.IsError)
				}
				if sink != nil {
					sink.OnToolResult(r.ToolUseID, r.ToolName, r.Content, r.IsError)
				}
			}
			recordWorkspaceWriteFromResults(&workspaceWriteOK, results)
			verifyGateProcessToolResults(o.cfg, o.ut, results, intentMessage)
			appendJSONToolTrace(toolTrace, pendingTools, results)
		} else {
			for _, tu := range pendingTools {
				toolCalls++
				toolStart := time.Now()
				out := o.executeTool(ctx, &tu, sink)
				metrics.tool += time.Since(toolStart)
				slog.Debug("tool done", "name", tu.Name, "elapsed", time.Since(toolStart), "bytes", len(out.Content), "isError", out.IsError)
				if out.Err != nil {
					o.emitTurnEndSummary(ctx, "tool_exec_error", metrics, toolCalls, workspaceWriteOK, hadToolRound, actionNudges, repairEscalations, editFileNotFoundNudges, reflectionFired)
					return "", out.Err
				}
				if o.afterTool != nil {
					o.afterTool(tu.Name, tu.Input, len(out.Content), out.IsError)
				}
				if sink != nil {
					sink.OnToolResult(tu.ID, tu.Name, out.Content, out.IsError)
				}
				results = append(results, llm.ToolResultRecord{
					ToolUseID: tu.ID,
					ToolName:  tu.Name,
					Content:   out.Content,
					IsError:   out.IsError,
				})
			}
			recordWorkspaceWriteFromResults(&workspaceWriteOK, results)
			verifyGateProcessToolResults(o.cfg, o.ut, results, intentMessage)
			appendJSONToolTrace(toolTrace, pendingTools, results)
		}
		o.session.AddToolResults(results)

		if toolResultsHaveEditNotFound(results) {
			o.session.Add("user", editFileNotFoundNudgeMessage)
			editFileNotFoundNudges++
			slog.Debug("orchestrator: edit_file not-found recovery nudge injected")
		}

		// Track consecutive read-only rounds for the reflection nudge.
		if lastBatchReadOnly {
			readOnlyToolRounds++
		} else {
			readOnlyToolRounds = 0
		}
		if workspaceWriteOK {
			readOnlyToolRounds = 0
		}
		if !reflectionFired && o.cfg.EnableReflectionNudge &&
			readOnlyToolRounds >= reflectionTriggerRounds &&
			!workspaceWriteOK && !o.profile.ReadOnly &&
			toolSpecsAllowWorkspaceWrite(o.effectiveToolSpecs()) &&
			userMessageWantsWorkspaceWrites(intentMessage) {
			o.session.Add("user", reflectionNudgeMessage)
			reflectionFired = true
			slog.Debug("orchestrator: reflection nudge injected", "readOnlyRounds", readOnlyToolRounds)
		}
	}

	metrics.toolCalls = toolCalls
	metrics.status = "iteration_limit"
	o.emitTurnEndSummary(ctx, "iteration_limit", metrics, toolCalls, workspaceWriteOK, hadToolRound, actionNudges, repairEscalations, editFileNotFoundNudges, reflectionFired)
	o.logTurnMetrics(metrics)
	return "", fmt.Errorf("iteration limit (%d) reached", iterLimit)
}

// applyTUIChatIterationCap returns iterLimit capped for TUI chat mode (see config.EffectiveTUIChatMaxIterations).
func applyTUIChatIterationCap(cfg config.Config, tuiInteractApply bool, tuiInteractMode string, iterLimit int) int {
	if !tuiInteractApply {
		return iterLimit
	}
	if config.NormalizeTUIInteractMode(tuiInteractMode) != config.TUIInteractModeChat {
		return iterLimit
	}
	capVal := cfg.EffectiveTUIChatMaxIterations()
	if iterLimit > capVal {
		return capVal
	}
	return iterLimit
}
