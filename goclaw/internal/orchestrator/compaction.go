package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/text"
)

const (
	compactPreserveTail    = 24
	compactionSnippetRunes = 280 // max runes per removed message line in the compaction summary

	// Default context window estimate when ModelContextTokens is unset (non-Ollama-num_ctx path; heuristic only).
	ollamaContextTokens = 32_000

	// compactedToolResult is the placeholder written over large tool-result payloads during phase-1 compaction.
	compactedToolResult = "[compacted]"
)

// ForceCompact runs compaction immediately: keeps the last compactPreserveTail messages
// and prepends a summary user message for the removed prefix. Ignores AutoCompactThreshold.
func (o *Orchestrator) ForceCompact() {
	o.compactToTail(compactPreserveTail)
}

func (o *Orchestrator) maybeCompact(ctx context.Context, sink StreamSink) {
	if o.cfg.AutoCompactThreshold <= 0 {
		return
	}
	budget := o.cfg.EffectiveContextTokens()
	limit := int(float64(budget) * o.cfg.AutoCompactThreshold)
	if o.estimatedSessionTokens(ctx, limit) < limit {
		return
	}

	// Phase 1: clear tool-result payloads in old turns — cheapest, preserves conversation structure.
	if msgs, cleared := clearOldToolResults(o.session.Messages, compactPreserveTail); cleared {
		o.session.ReplaceMessages(msgs)
		if o.estimatedSessionTokens(ctx, limit) < limit {
			return
		}
	}

	// Phase 2: summarize and remove old messages.
	before := len(o.session.Messages)
	if o.cfg.LLMCompaction {
		o.compactToTailWithLLM(ctx, compactPreserveTail)
	} else {
		o.compactToTail(compactPreserveTail)
	}
	if sink != nil {
		removed := before - len(o.session.Messages)
		if removed < 0 {
			removed = 0
		}
		sink.OnCompact(removed)
	}
}

// compactToTailWithLLM summarizes the head of the conversation using the active LLM
// instead of the heuristic placeholder. Falls back to compactToTail on any LLM error.
func (o *Orchestrator) compactToTailWithLLM(ctx context.Context, preserve int) {
	if o.llm == nil {
		o.compactToTail(preserve)
		return
	}
	msgs := o.session.Messages
	if len(msgs) <= preserve {
		return
	}
	head := msgs[:len(msgs)-preserve]

	// Build a plain-text excerpt for the LLM to summarize.
	var excerpt strings.Builder
	for _, m := range head {
		excerpt.WriteString(m.Role)
		excerpt.WriteString(": ")
		if m.Content != "" {
			excerpt.WriteString(m.Content)
		} else if len(m.ToolCalls) > 0 {
			excerpt.WriteString("[tool calls: ")
			for i, tc := range m.ToolCalls {
				if i > 0 {
					excerpt.WriteString(", ")
				}
				excerpt.WriteString(tc.Name)
			}
			excerpt.WriteString("]")
		}
		excerpt.WriteByte('\n')
	}

	req := llm.Request{
		Model:  o.cfg.ModelForCompaction(),
		System: "You are a concise session summarizer for a coding agent. Summarize the excerpt in 3-6 short sentences. " +
			"Preserve concrete file paths, commands run, errors encountered, and explicit user decisions. " +
			"Mention open todos or unfinished work if visible. Omit boilerplate and tool-narration chatter. English only.",
		Messages: []llm.Message{
			llm.PlainMessage("user", "Summarize this conversation:\n\n"+excerpt.String()),
		},
		MaxTokens: 512,
	}
	if o.cfg.Provider == "ollama" && o.cfg.OllamaNumCtx > 0 {
		req.NumCtx = o.cfg.OllamaNumCtx
	}

	events, errc := o.llm.Stream(ctx, req)
	var summary strings.Builder
	for event := range events {
		if delta, ok := event.(llm.TextDelta); ok {
			summary.WriteString(delta.Text)
		}
	}
	if err := <-errc; err != nil {
		slog.Warn("llm compaction: LLM error, falling back to heuristic (context detail may be reduced)",
			"err", err, "head_messages", len(head))
		o.compactToTail(preserve)
		return
	}

	summaryText := strings.TrimSpace(summary.String())
	if summaryText == "" {
		// Empty LLM response — heuristic is more useful than a blank compaction message.
		slog.Warn("llm compaction: empty summary returned, falling back to heuristic",
			"head_messages", len(head))
		o.compactToTail(preserve)
		return
	}

	tail := msgs[len(msgs)-preserve:]
	o.session.ReplaceMessages(tail)
	o.session.PrependMessage(llm.PlainMessage("user", "[session compacted] "+summaryText))
}

// clearOldToolResults replaces the Content of ToolResults in messages outside the preserved tail
// with a short placeholder, freeing context space without removing message structure.
// Returns the (possibly modified) slice and whether any content was cleared.
func clearOldToolResults(msgs []llm.Message, preserve int) ([]llm.Message, bool) {
	if len(msgs) <= preserve {
		return msgs, false
	}
	changed := false
	for i := 0; i < len(msgs)-preserve; i++ {
		for j := range msgs[i].ToolResults {
			if msgs[i].ToolResults[j].Content != "" && msgs[i].ToolResults[j].Content != compactedToolResult {
				msgs[i].ToolResults[j].Content = compactedToolResult
				changed = true
			}
		}
	}
	return msgs, changed
}

// estimatedSessionTokens returns a heuristic token count for compaction decisions.
func (o *Orchestrator) estimatedSessionTokens(_ context.Context, _ int) int {
	return sessionTokenEstimate(o.session.Messages, o.cfg.Provider)
}

// contextBudgetTokens is kept for SessionCompactionFillPercentLive (TUI footer).
// New orchestrator code should use cfg.EffectiveContextTokens() instead.
func contextBudgetTokens(provider string, cfgTokens int) int {
	if cfgTokens > 0 {
		return cfgTokens
	}
	return ollamaContextTokens
}

// sessionTokenEstimateFromChars maps a UTF-8 byte count to an approximate token count (char÷4 heuristic).
func sessionTokenEstimateFromChars(chars int) int {
	if chars < 0 {
		chars = 0
	}
	return (chars + 3) / 4
}

// sessionTokenEstimate approximates tokens from message text (chars ÷ 4 heuristic).
func sessionTokenEstimate(msgs []llm.Message, provider string) int {
	_ = provider
	c := sessionCharEstimate(msgs)
	return sessionTokenEstimateFromChars(c)
}

// SessionMessagesTokenEstimate is a rough context-size hint from stored message payloads (not billed API usage).
// Same heuristics as compaction; safe for TUI footers and status lines.
func SessionMessagesTokenEstimate(msgs []llm.Message, provider string) int {
	return sessionTokenEstimate(msgs, provider)
}

// SessionMessagesTokenEstimateLive includes extraChars (e.g. in-flight assistant UTF-8 bytes) in the estimate for UI hints.
func SessionMessagesTokenEstimateLive(msgs []llm.Message, provider string, extraChars int) int {
	_ = provider
	c := sessionCharEstimate(msgs) + extraChars
	return sessionTokenEstimateFromChars(c)
}

// SessionCompactionFillPercentLive estimates how full the session is relative to the auto-compaction
// trigger (same context budget and char heuristic as maybeCompact). extraChars adds in-flight UTF-8
// bytes (e.g. assistant text still streaming).
// Returns (percent, true) when auto_compact_threshold > 0; otherwise (0, false).
func SessionCompactionFillPercentLive(msgs []llm.Message, cfg config.Config, extraChars int) (int, bool) {
	tok := SessionMessagesTokenEstimateLive(msgs, cfg.Provider, extraChars)
	return sessionCompactionFillPercentFromTokenEstimate(cfg, tok)
}

func sessionCompactionFillPercentFromTokenEstimate(cfg config.Config, tok int) (int, bool) {
	if cfg.AutoCompactThreshold <= 0 {
		return 0, false
	}
	budget := cfg.EffectiveContextTokens()
	if budget <= 0 {
		return 0, false
	}
	limit := int(float64(budget) * cfg.AutoCompactThreshold)
	if limit <= 0 {
		return 0, false
	}
	p := int(int64(tok) * 100 / int64(limit))
	if p < 0 {
		p = 0
	}
	if p > 999 {
		p = 999
	}
	return p, true
}

func sessionCharEstimate(msgs []llm.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Content)
		for _, c := range m.ToolCalls {
			n += len(c.ID) + len(c.Name) + len(c.Input)
		}
		for _, r := range m.ToolResults {
			n += len(r.ToolUseID) + len(r.ToolName) + len(r.Content)
		}
	}
	return n
}

func (o *Orchestrator) compactToTail(preserve int) {
	msgs := o.session.Messages
	if len(msgs) <= preserve {
		return
	}
	head := msgs[:len(msgs)-preserve]
	tail := msgs[len(msgs)-preserve:]
	var sb strings.Builder
	sb.WriteString("[session compacted] Summarized ")
	sb.WriteString(fmt.Sprintf("%d", len(head)))
	sb.WriteString(" earlier message(s); tail of ")
	sb.WriteString(fmt.Sprintf("%d", len(tail)))
	sb.WriteString(" kept. Excerpt: ")
	for i, m := range head {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		snip := text.TruncateRunes(m.Content, compactionSnippetRunes)
		sb.WriteString(snip)
	}
	o.session.ReplaceMessages(tail)
	o.session.PrependMessage(llm.PlainMessage("user", sb.String()))
}
