package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// ExpandInlineAtRefs scans userText for @path tokens mixed with other text,
// silently reads each path via the read_file tool, and prepends the file
// contents as a structured context block before the original message.
//
// Only activates when the message would be KindPassthrough (i.e. not a pure
// standalone @path prefix). Pure @path messages are handled by RunLocalPrefixToolIfAny.
//
// If no @path tokens are found, or all reads fail, the original text is returned unchanged.
func ExpandInlineAtRefs(ctx context.Context, orch *orchestrator.Orchestrator, userText string) string {
	tokens := inputprefix.ExtractAtTokens(userText)
	if len(tokens) == 0 {
		return userText
	}

	// Deduplicate by normalized path so @go.mod and @go.mod/ do not run read_file twice.
	seenPath := make(map[string]struct{}, len(tokens))
	blocks := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		path := normalizeInlineAtPath(tok)
		if path == "" {
			continue
		}
		if _, dup := seenPath[path]; dup {
			continue
		}
		seenPath[path] = struct{}{}

		payload, err := json.Marshal(map[string]string{"path": path})
		if err != nil {
			continue
		}
		content, isErr, runErr := orch.RunToolInvocation(ctx, "read_file", string(payload), nil)
		if runErr != nil || isErr {
			continue
		}
		content = strings.TrimRight(content, "\n")
		blocks = append(blocks, fmt.Sprintf("%s\n---\n%s", tok, content))
	}
	if len(blocks) == 0 {
		return userText
	}
	var b strings.Builder
	b.WriteString("[Files loaded via @ references]\n\n")
	b.WriteString(strings.Join(blocks, "\n\n"))
	b.WriteString("\n\n[End of @ context]\n\n")
	b.WriteString(userText)
	return b.String()
}

func normalizeInlineAtPath(tok string) string {
	path := strings.TrimPrefix(strings.TrimSpace(tok), "@")
	return strings.TrimSuffix(strings.TrimSpace(path), "/")
}
