package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
)

const (
	compactPreserveTail    = 24
	compactionSnippetRunes = 280 // max runes per removed message line in the compaction summary

	// Per-provider context window estimates in tokens.
	// Anthropic claude-sonnet-4-6 / claude-opus-4: 200k token context.
	// Ollama default: conservative 32k (covers most local models; override via ModelContextTokens).
	anthropicContextTokens = 200_000
	ollamaContextTokens    = 32_000

	// compactedToolResult is the placeholder written over large tool-result payloads during phase-1 compaction.
	compactedToolResult = "[compacted]"
)

// ForceCompact runs compaction immediately: keeps the last compactPreserveTail messages
// and prepends a summary user message for the removed prefix. Ignores AutoCompactThreshold.
func (o *Orchestrator) ForceCompact() {
	o.compactToTail(compactPreserveTail)
}

func (o *Orchestrator) maybeCompact(ctx context.Context) {
	if o.cfg.AutoCompactThreshold <= 0 {
		return
	}
	budget := contextBudgetTokens(o.cfg.Provider, o.cfg.ModelContextTokens)
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
	if o.cfg.LLMCompaction {
		o.compactToTailWithLLM(ctx, compactPreserveTail)
	} else {
		o.compactToTail(compactPreserveTail)
	}
}

// compactToTailWithLLM summarizes the head of the conversation using the active LLM
// instead of the heuristic placeholder. Falls back to compactToTail on any LLM error.
func (o *Orchestrator) compactToTailWithLLM(ctx context.Context, preserve int) {
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
		System: "You are a concise summarizer. Summarize the following conversation excerpt in 3-5 sentences. Focus on key decisions, file paths modified, and outcomes. Be brief.",
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
		slog.Warn("llm compaction failed, falling back to heuristic", "err", err)
		o.compactToTail(preserve)
		return
	}

	tail := msgs[len(msgs)-preserve:]
	o.session.ReplaceMessages(tail)
	o.session.PrependMessage(llm.PlainMessage("user", "[session compacted] "+strings.TrimSpace(summary.String())))
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

// estimatedSessionTokens returns token count for compaction decisions. For Anthropic with
// InputTokenCounter and token_count_mode auto, uses the count_tokens API once the heuristic
// estimate reaches 70% of the compaction threshold.
func (o *Orchestrator) estimatedSessionTokens(ctx context.Context, compactLimit int) int {
	heuristic := sessionTokenEstimate(o.session.Messages, o.cfg.Provider)
	if o.inputTokenCounter == nil || o.cfg.Provider != "anthropic" {
		return heuristic
	}
	mode := strings.ToLower(strings.TrimSpace(o.cfg.TokenCountMode))
	if mode == "heuristic" {
		return heuristic
	}
	soft := int(float64(compactLimit) * 0.7)
	if heuristic < soft {
		return heuristic
	}
	req := o.buildRequest()
	n, err := o.inputTokenCounter.CountInputTokens(ctx, req)
	if err != nil || n <= 0 {
		slog.Debug("count_tokens failed, using heuristic", "err", err)
		return heuristic
	}
	return n
}

// contextBudgetTokens returns the effective context window in estimated tokens.
// cfgTokens > 0 overrides the per-provider default (set via ModelContextTokens in settings.json).
func contextBudgetTokens(provider string, cfgTokens int) int {
	if cfgTokens > 0 {
		return cfgTokens
	}
	switch provider {
	case "anthropic":
		return anthropicContextTokens
	case "openai_compatible":
		return ollamaContextTokens
	default:
		return ollamaContextTokens
	}
}

// sessionTokenEstimateFromChars maps a UTF-8 byte count to an approximate token count (same divisors as compaction).
func sessionTokenEstimateFromChars(chars int, providerLower string) int {
	if chars < 0 {
		chars = 0
	}
	switch providerLower {
	case "anthropic":
		return (chars + 2) / 3
	case "openai_compatible":
		return (chars + 3) / 4
	default:
		return (chars + 3) / 4
	}
}

// sessionTokenEstimate approximates tokens from message text (chars ÷ divisor by provider).
func sessionTokenEstimate(msgs []llm.Message, provider string) int {
	p := strings.ToLower(strings.TrimSpace(provider))
	c := sessionCharEstimate(msgs)
	return sessionTokenEstimateFromChars(c, p)
}

// SessionMessagesTokenEstimate is a rough context-size hint from stored message payloads (not billed API usage).
// Same heuristics as compaction; safe for TUI footers and status lines.
func SessionMessagesTokenEstimate(msgs []llm.Message, provider string) int {
	return sessionTokenEstimate(msgs, provider)
}

// SessionMessagesTokenEstimateLive includes extraChars (e.g. in-flight assistant UTF-8 bytes) in the estimate for UI hints.
func SessionMessagesTokenEstimateLive(msgs []llm.Message, provider string, extraChars int) int {
	p := strings.ToLower(strings.TrimSpace(provider))
	c := sessionCharEstimate(msgs) + extraChars
	return sessionTokenEstimateFromChars(c, p)
}

// SessionCompactionFillPercent estimates how full the session is relative to the auto-compaction
// trigger (same context budget and char heuristic as maybeCompact). It does not call Anthropic
// count_tokens. Returns (percent, true) when auto_compact_threshold > 0; otherwise (0, false).
func SessionCompactionFillPercent(msgs []llm.Message, cfg config.Config) (int, bool) {
	if cfg.AutoCompactThreshold <= 0 {
		return 0, false
	}
	budget := contextBudgetTokens(cfg.Provider, cfg.ModelContextTokens)
	if budget <= 0 {
		return 0, false
	}
	limit := int(float64(budget) * cfg.AutoCompactThreshold)
	if limit <= 0 {
		return 0, false
	}
	tok := SessionMessagesTokenEstimate(msgs, cfg.Provider)
	p := int(int64(tok) * 100 / int64(limit))
	if p < 0 {
		p = 0
	}
	if p > 999 {
		p = 999
	}
	return p, true
}

// SessionCompactionFillPercentLive is like SessionCompactionFillPercent but includes extraChars in the token estimate
// (e.g. assistant text still streaming into the session).
func SessionCompactionFillPercentLive(msgs []llm.Message, cfg config.Config, extraChars int) (int, bool) {
	if cfg.AutoCompactThreshold <= 0 {
		return 0, false
	}
	budget := contextBudgetTokens(cfg.Provider, cfg.ModelContextTokens)
	if budget <= 0 {
		return 0, false
	}
	limit := int(float64(budget) * cfg.AutoCompactThreshold)
	if limit <= 0 {
		return 0, false
	}
	tok := SessionMessagesTokenEstimateLive(msgs, cfg.Provider, extraChars)
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
		snip := m.Content
		if len([]rune(snip)) > compactionSnippetRunes {
			rs := []rune(snip)
			snip = string(rs[:compactionSnippetRunes]) + "…"
		}
		sb.WriteString(snip)
	}
	o.session.ReplaceMessages(tail)
	o.session.PrependMessage(llm.PlainMessage("user", sb.String()))
}
