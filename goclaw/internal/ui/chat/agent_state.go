package chat

import (
	"fmt"
	"strings"
)

// AgentState is the canonical state of the agent pipeline, driving all animated UI elements.
// A single AgentState value completely determines what the status line renders.
type AgentState int

const (
	// AgentStateIdle — no active model work; static prompt, no spinner.
	AgentStateIdle AgentState = iota
	// AgentStateThinking — LLM prefill / "thinking" phase; pulsing braille spinner.
	AgentStateThinking
	// AgentStateExecuting — a tool call is in flight; gear icon + tool label + elapsed.
	AgentStateExecuting
	// AgentStateWriting — assistant tokens are streaming in; dot spinner + byte hint.
	AgentStateWriting
	// AgentStateDone — turn completed cleanly; static ✓ (cleared after next user send).
	AgentStateDone
	// AgentStateError — turn ended with an error; static ✖ in error color.
	AgentStateError
)

// String returns the human-readable label for display.
func (s AgentState) String() string {
	switch s {
	case AgentStateThinking:
		return "THINKING"
	case AgentStateExecuting:
		return "EXECUTING"
	case AgentStateWriting:
		return "WRITING"
	case AgentStateDone:
		return "DONE"
	case AgentStateError:
		return "ERROR"
	default:
		return ""
	}
}

// animTickMsg is sent by the 80ms animation tick when the spinner is active.
// It is distinct from footerTickMsg (600ms) so footer-stats recompute stays cheap.
type animTickMsg struct{}

// RenderAgentStatus is the single source-of-truth for the footer status line.
// All state-to-text mapping lives here; callers simply pass the current model state.
//
//	AgentStateIdle      → "" (nothing shown)
//	AgentStateThinking  → "[⠋] THINKING  → phase_label (Ns)"
//	AgentStateExecuting → "[⠙] EXECUTING  ⚙ tool: summary (Ns)"
//	AgentStateWriting   → "[·] WRITING    outputting…"
//	AgentStateDone      → "[✓] DONE"
//	AgentStateError     → "[✖] ERROR  message"
func RenderAgentStatus(th *Theme, state AgentState, spinnerView string,
	thinkingPhase string, thinkingElapsed int,
	toolLabel string, toolSummary string, toolElapsed int,
	streamBytes int,
	errMsg string,
) string {
	if th == nil {
		th = DefaultTheme()
	}

	switch state {
	case AgentStateIdle:
		return ""

	case AgentStateThinking:
		phase := strings.TrimSpace(thinkingPhase)
		if phase == "" {
			phase = "Thinking"
		}
		// Parse "[N/M] label" format from orchestrator phase strings.
		var iterPart, labelPart string
		if strings.HasPrefix(phase, "[") {
			if close := strings.Index(phase, "]"); close > 0 {
				iterPart = phase[1:close]
				labelPart = strings.TrimSpace(phase[close+1:])
			}
		}
		if iterPart == "" {
			labelPart = phase
		}
		elapsed := ""
		if thinkingElapsed >= 1 {
			elapsed = fmt.Sprintf(" (%ds)", thinkingElapsed)
		}
		var iterLabel string
		if iterPart != "" {
			iterLabel = " " + th.FooterDim.Render("["+iterPart+"]")
		}
		stateTag := th.FooterDim.Render("thinking")
		label := th.StatusBarLabel.Render(labelPart) + th.FooterDim.Render(elapsed)
		return spinnerView + iterLabel + " " + stateTag + "  " + label

	case AgentStateExecuting:
		tool := strings.TrimSpace(toolLabel)
		glyph := th.Tool.Render("⚙")
		stateTag := th.FooterDim.Render("running")
		line := spinnerView + " " + stateTag + "  " + glyph
		if tool != "" {
			line += " " + th.Tool.Render(tool)
		}
		if sum := strings.TrimSpace(toolSummary); sum != "" {
			line += th.FooterDim.Render(": " + sum)
		}
		if toolElapsed >= 1 {
			line += th.FooterDim.Render(fmt.Sprintf(" (%ds)", toolElapsed))
		}
		return line

	case AgentStateWriting:
		stateTag := th.FooterDim.Render("writing")
		hint := "streaming"
		if streamBytes > 0 {
			hint = humanBytes(streamBytes)
		}
		return spinnerView + " " + stateTag + "  " + th.StatusBarLabel.Render(hint)

	case AgentStateDone:
		ok := th.ToolResultOk.Render("✓")
		return ok + "  " + th.FooterDim.Render("done")

	case AgentStateError:
		errGlyph := th.ToolResultErr.Render("✖")
		msg := strings.TrimSpace(errMsg)
		if msg == "" {
			msg = "error"
		}
		return errGlyph + "  " + th.ErrorStyle.Render(msg)
	}
	return ""
}
