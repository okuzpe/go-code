package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

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
	tokens := extractAtTokens(userText)
	if len(tokens) == 0 {
		return userText
	}
	var blocks []string
	for _, tok := range tokens {
		path := strings.TrimPrefix(tok, "@")
		path = strings.TrimSuffix(path, "/") // strip trailing slash from dir tokens
		if path == "" {
			continue
		}
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

// extractAtTokens returns all @path tokens found in s where @ is at the start
// or preceded by whitespace. Tokens with ".." are skipped for safety.
func extractAtTokens(s string) []string {
	runes := []rune(s)
	var tokens []string
	seen := map[string]bool{}
	i := 0
	for i < len(runes) {
		if runes[i] == '@' && (i == 0 || runes[i-1] == ' ' || runes[i-1] == '\t' || runes[i-1] == '\n') {
			j := i + 1
			for j < len(runes) && runes[j] != ' ' && runes[j] != '\t' && runes[j] != '\n' {
				j++
			}
			if j > i+1 {
				tok := string(runes[i:j])
				path := strings.TrimPrefix(tok, "@")
				path = strings.TrimSuffix(path, "/")
				// Safety: reject path traversal
				if strings.Contains(path, "..") {
					i = j
					continue
				}
				// Reject absolute paths (workspace-relative only)
				if filepath.IsAbs(path) {
					i = j
					continue
				}
				if !seen[tok] {
					tokens = append(tokens, tok)
					seen[tok] = true
				}
			}
			i = j
			continue
		}
		i++
	}
	return tokens
}
