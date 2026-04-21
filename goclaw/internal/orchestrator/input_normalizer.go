package orchestrator

import (
	"context"
	"log/slog"
	"strings"

	"github.com/okuzpe/goclaw/internal/llm"
)

const (
	// normalizeInputMinLength is the minimum character count before translation is attempted.
	// Very short messages (greetings, single words) do not justify an extra LLM call.
	normalizeInputMinLength = 12

	// normalizeInputMaxTokens caps the output of the translation call.
	normalizeInputMaxTokens = 1024

	normalizeInputSystemPrompt = "Translate the following user message to English. " +
		"Output only the translated text — no explanations, no quotes. " +
		"Preserve technical terms, file paths, code symbols, package names, " +
		"and proper nouns exactly as-is. If the message is already in English, output it unchanged."
)

// normalizeInputToEnglish translates a non-English user message to English using a short Ollama
// call. The translation uses the provided model (ideally a lightweight one such as the compaction
// model) and is purely best-effort: on any failure it returns the original message unchanged.
//
// Call this only when language detection has already confirmed that the message is non-English.
func normalizeInputToEnglish(ctx context.Context, client llm.Client, model, message string) string {
	message = strings.TrimSpace(message)
	if len(message) < normalizeInputMinLength {
		return message
	}

	req := llm.Request{
		Model:     model,
		System:    normalizeInputSystemPrompt,
		Messages:  []llm.Message{{Role: "user", Content: message}},
		MaxTokens: normalizeInputMaxTokens,
	}

	events, errc := client.Stream(ctx, req)
	var output strings.Builder
	for event := range events {
		if delta, ok := event.(llm.TextDelta); ok {
			output.WriteString(delta.Text)
		}
	}
	if err := <-errc; err != nil {
		slog.Debug("input normalizer: translation call failed, using original", "err", err)
		return message
	}

	translated := strings.TrimSpace(output.String())
	if translated == "" {
		slog.Debug("input normalizer: empty translation result, using original")
		return message
	}

	slog.Debug("input normalizer: translated input", "lang_from", "non-en", "chars_in", len(message), "chars_out", len(translated))
	return translated
}
