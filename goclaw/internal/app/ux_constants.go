package app

// OllamaNumCtxBannerWarnBelow is the threshold used by the non-TTY startup banner (when printed):
// when ollama_num_ctx is set and strictly below this value, we print a one-line warning
// (tool schemas + system prompt need context headroom).
const OllamaNumCtxBannerWarnBelow = 8192
