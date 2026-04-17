---
name: write-tests
description: Use when the user asks to write tests, add coverage, or test a specific goclaw package.
---

> **Language:** Author and maintain this file in English only. Rule: `.cursor/rules/agent-artifacts-english.mdc` (paths from the repository root).

## Writing tests in goclaw

### Mock OpenAI-compatible server

`testutil/mockopenai/` implements `/v1/chat/completions` SSE without API tokens.

```go
import "github.com/okuzpe/goclaw/testutil/mockopenai"

func TestExample(t *testing.T) {
    srv := mockopenai.New([]mockopenai.Scenario{
        {Match: "hello", Response: "Hi there."},
        {Match: "", Response: "default"},
    })
    defer srv.Close()

    client := llm.NewOpenAICompat("test-key", srv.URL+"/v1")
    // ...
}
```

Scenarios match the **last** request message (text or `tool_result` bodies). Use `Tool` or `Tools` for tool streams.

### Scenario ideas (grow coverage over time)

| # | Name | Exercises |
|---|------|-----------|
| 1 | text_only | Plain assistant text |
| 2 | streaming | Multiple `TextDelta` before `Done` |
| 3 | http_500 | Error returned, no panic |
| 4 | tool_roundtrip | Tool call → execute → next assistant turn |
| 5 | permission_denied | Deny / decline → `is_error` tool result, loop continues |
| 6 | read_file_roundtrip | Real file + mock LLM |
| 7 | bash_stdout | Allowlisted command |
| 8 | web_fetch / SSRF | Network tool tests |
| 9 | compaction | Large session triggers summary + tail preserve |
| 10 | session_store | JSONL save / load |

### Tool unit test shape

```go
package tools

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

func TestReadFileTool(t *testing.T) {
    tool := NewReadFile(t.TempDir())

    t.Run("reads existing file", func(t *testing.T) {
        path := filepath.Join(t.TempDir(), "a.txt")
        if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
            t.Fatal(err)
        }
        res, err := tool.Execute(context.Background(), `{"path":"a.txt"}`)
        if err != nil {
            t.Fatal(err)
        }
        if res.IsError {
            t.Fatalf("unexpected IsError: %s", res.Content)
        }
        if res.Content != "hello" {
            t.Errorf("content: %q", res.Content)
        }
    })

    t.Run("rejects path outside workspace", func(t *testing.T) {
        res, err := tool.Execute(context.Background(), `{"path":"/etc/passwd"}`)
        if err != nil {
            t.Fatal(err)
        }
        if !res.IsError {
            t.Error("want IsError for path outside workspace")
        }
    })
}
```
(Adapt to the real `ReadFile` constructor and relative path rules.)

### Orchestrator integration test shape

See `internal/orchestrator/orchestrator_test.go` — `newOrch`, mock server URL in OpenAI-compat base URL, `permissions` + `tools` wired.

### Commands
```bash
go test ./...
go test -v ./internal/tools/...
go test -run TestReadFile ./...
```
