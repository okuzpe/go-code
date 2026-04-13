package llm

import "strings"

// ChatML message boundary tokens (see tiktoken / Ollama chat templates). Models sometimes
// emit these into decoded assistant text when the template leaks.
var (
	chatMLImStart = "<|im_start|>"
	chatMLImEnd   = "<|" + "im_end" + "|>"
	// Qwen and some local GGUF builds use this spelling instead of plain im_end.
	chatMLRedactedImEnd = "<|" + "redacted_im_end" + "|>"
)

// stripLeakedChatTemplateTokens removes ChatML-/Qwen-style control tokens that sometimes
// appear verbatim in assistant streams when the chat template leaks into decoded output
// or stop sequences miss a boundary.
func stripLeakedChatTemplateTokens(s string) string {
	if s == "" || !strings.Contains(s, "<|") {
		return s
	}
	r := strings.NewReplacer(
		chatMLImStart, "",
		chatMLImEnd, "",
		chatMLRedactedImEnd, "",
		"<|im_sep|>", "",
		"<|endoftext|>", "",
	)
	return strings.TrimSpace(r.Replace(s))
}
