package memory

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
)

const silentExtractMaxOutRunes = 4000

const silentTurnExtractSystem = `You propose at most one durable memory entry from a short chat exchange.
Reply with a single JSON object only (no markdown fences, no commentary):
{"save":false}
OR
{"save":true,"type":"user|feedback|project|reference","name":"short_title","description":"one line summary","body":"optional detail"}

Rules:
- Save only stable facts worth recalling in future sessions (preferences, validated corrections, deadlines, external pointers).
- Do not save code, file trees, git facts, or anything already derivable from the repository.
- If nothing qualifies, return {"save":false}.
- "name" must be short (under 80 characters). "description" one line. "body" optional; keep it brief.`

// ScheduleSilentTurnLLMExtract starts a background LLM call after a fully tool-free user turn
// (see memory-system.md §6). It is best-effort and never blocks the caller.
// model is the resolved main-turn model id (including profile override / task routing when applicable).
func ScheduleSilentTurnLLMExtract(cfg config.Config, client llm.Client, store *Store, model, userMessage, assistantReply string) {
	if !cfg.MemoryLLMSilentExtract || store == nil || client == nil {
		return
	}
	um := strings.TrimSpace(userMessage)
	ar := strings.TrimSpace(assistantReply)
	if um == "" || ar == "" {
		return
	}
	if strings.HasPrefix(um, "/") {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("memory silent LLM extract panic", "recover", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		runSilentTurnLLMExtract(ctx, cfg, client, store, strings.TrimSpace(model), um, ar)
	}()
}

type silentExtractPayload struct {
	Save        bool   `json:"save"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

func runSilentTurnLLMExtract(ctx context.Context, cfg config.Config, client llm.Client, store *Store, model, userMsg, assistantMsg string) {
	if model == "" {
		model = strings.TrimSpace(cfg.Model())
	}
	if model == "" {
		slog.Debug("memory silent extract skipped: empty model")
		return
	}
	req := llm.Request{
		Model:     model,
		System:    silentTurnExtractSystem,
		Messages:  []llm.Message{llm.PlainMessage("user", "USER:\n"+userMsg+"\n\nASSISTANT:\n"+assistantMsg)},
		MaxTokens: 512,
	}
	if cfg.Provider == "ollama" && cfg.OllamaNumCtx > 0 {
		req.NumCtx = cfg.OllamaNumCtx
	}
	events, errc := client.Stream(ctx, req)
	var sb strings.Builder
	for e := range events {
		switch ev := e.(type) {
		case llm.TextDelta:
			sb.WriteString(ev.Text)
		}
	}
	if err := <-errc; err != nil {
		slog.Warn("memory silent extract: llm stream failed", "err", err)
		return
	}
	raw := strings.TrimSpace(sb.String())
	if raw == "" {
		return
	}
	var p silentExtractPayload
	if err := json.Unmarshal(firstJSONObject(raw), &p); err != nil {
		slog.Debug("memory silent extract: json parse failed", "err", err, "preview", truncateForLog(raw, 120))
		return
	}
	if !p.Save {
		return
	}
	t := Type(strings.TrimSpace(strings.ToLower(p.Type)))
	switch t {
	case TypeUser, TypeFeedback, TypeProject, TypeReference:
	default:
		slog.Debug("memory silent extract: invalid type", "type", p.Type)
		return
	}
	name := strings.TrimSpace(p.Name)
	desc := strings.TrimSpace(p.Description)
	body := strings.TrimSpace(p.Body)
	if name == "" || desc == "" {
		slog.Debug("memory silent extract: missing name or description")
		return
	}
	if utf8.RuneCountInString(name) > 200 {
		name = string([]rune(name)[:200])
	}
	if utf8.RuneCountInString(desc) > 500 {
		desc = string([]rune(desc)[:500])
	}
	if utf8.RuneCountInString(body) > silentExtractMaxOutRunes {
		body = string([]rune(body)[:silentExtractMaxOutRunes])
	}
	_, err := store.Save(Entry{
		Name:        name,
		Description: desc,
		Type:        t,
		Body:        body,
	})
	if err != nil {
		slog.Warn("memory silent extract: save failed", "err", err)
		return
	}
	slog.Info("memory silent extract: saved entry", "type", t, "name", name)
}

func firstJSONObject(s string) []byte {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 {
		s = s[:j+1]
	}
	return []byte(s)
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
