package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
)

const (
	taskModelLLMMaxTokens = 128
	taskModelLLMTimeout   = 15 * time.Second
)

var errTaskModelLLMEmpty = errors.New("router returned empty role")

// classifyTaskRoleLLM asks a small model for a single JSON object {"role":"…"}.
func classifyTaskRoleLLM(ctx context.Context, client llm.Client, userMsg string, cfg config.Config) (role string, detail string, err error) {
	if client == nil {
		return "", "", fmt.Errorf("nil client")
	}
	ctx, cancel := context.WithTimeout(ctx, taskModelLLMTimeout)
	defer cancel()

	system := `You route the user's message to one task category. Reply with ONLY a JSON object, no markdown, no other text.
Shape: {"role":"ONE"} where ONE is exactly one of: default, reasoning, code, fast, explore, creative.
Definitions:
- code: programming, debugging, refactoring, tests, stack traces, implementation
- reasoning: multi-step analysis, tradeoffs, architecture decisions, proofs
- fast: trivial or very short factual questions
- explore: finding symbols, locating code in the repo, search/navigation questions
- creative: prose, marketing copy, brainstorming, stories
- default: none of the above fit well`

	req := llm.Request{
		Model:     cfg.RouterModelForLLM(),
		System:    system,
		MaxTokens: taskModelLLMMaxTokens,
		Messages: []llm.Message{
			{Role: "user", Content: userMsg},
		},
	}

	events, errc := client.Stream(ctx, req)
	var b strings.Builder
	for ev := range events {
		switch t := ev.(type) {
		case llm.TextDelta:
			b.WriteString(t.Text)
		case llm.Done:
		default:
		}
	}
	if err = <-errc; err != nil {
		return "", "", err
	}
	raw := strings.TrimSpace(b.String())
	role, jerr := parseRouterRoleJSON(raw)
	if jerr != nil {
		return "", raw, jerr
	}
	if role == "" {
		return "", raw, errTaskModelLLMEmpty
	}
	return role, "json_ok", nil
}

func parseRouterRoleJSON(text string) (string, error) {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return "", fmt.Errorf("no json object in router response")
	}
	s = s[i : j+1]

	var wrap struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal([]byte(s), &wrap); err != nil {
		return "", err
	}
	return normalizeTaskRole(wrap.Role), nil
}
