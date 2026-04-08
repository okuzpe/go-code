---
name: implement-tool
description: Use when the user asks to implement, add, or create a new tool for goclaw — interface through registration and tests.
---

## Implementing a new tool in goclaw

### 1. Read first
- `internal/tools/registry.go` — `Tool` and `Result`
- `internal/orchestrator/orchestrator.go` — execution and permissions
- An existing tool in `internal/tools/` as a template

### 2. Add `internal/tools/<name>.go`

```go
package tools

import (
    "context"
    "encoding/json"
)

// FooInput is the JSON arguments from the model.
type FooInput struct {
    // required fields per contract
}

// FooTool implements Tool.
type FooTool struct{}

func (t *FooTool) Name() string        { return "foo_name" }
func (t *FooTool) Description() string { return "..." }
func (t *FooTool) InputSchema() any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{ /* ... */ },
        "required": []string{ /* ... */ },
    }
}

func (t *FooTool) Execute(ctx context.Context, rawInput string) (Result, error) {
    var in FooInput
    if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
        return Result{IsError: true, Content: "invalid input: " + err.Error()}, nil
    }
    // ...
    return Result{Content: output}, nil
}
```

Use constructor pattern if the tool needs dependencies (e.g. `NewReadFile(root string)`).

### 3. Rules

- Tool **name**: `snake_case` (`read_file`, `web_fetch`)
- **User errors**: `Result{IsError: true, Content: msg}`, `error == nil`
- **System errors**: return non-nil `error`
- **Output caps**: read_file 512 KiB / 200 lines; glob 500 paths; grep 200 matches / 512 KiB per file; bash 256 KiB; web_fetch 1 MiB; web_search per `limits.go`
- **Security**: workspace paths for read_file, glob, grep; bash allowlist; web_fetch SSRF checks

### 4. Register in `internal/app/run.go`

Inside the `if !disableTools { ... }` block:
```go
reg.Register(tools.NewFoo(...))
```

> Registration lives in `internal/app/run.go`, not `main.go`.

### 5. Test `internal/tools/<name>_test.go`

Minimum three cases: happy path, invalid JSON input, boundary/security rejection.

```go
func TestFooTool(t *testing.T) {
    tool := NewFoo(t.TempDir())

    // happy path
    res, err := tool.Execute(context.Background(), `{"field":"value"}`)
    if err != nil { t.Fatal(err) }
    if res.IsError { t.Fatalf("unexpected error: %s", res.Content) }

    // invalid JSON → IsError, no Go error
    res, err = tool.Execute(context.Background(), `not-json`)
    if err != nil { t.Fatal(err) }
    if !res.IsError { t.Fatal("expected error for invalid JSON") }

    // path escape (if applicable)
    res, err = tool.Execute(context.Background(), `{"path":"../../etc/passwd"}`)
    if err != nil { t.Fatal(err) }
    if !res.IsError { t.Fatal("expected workspace boundary error") }
}
```

For tests that require a live Ollama, add a skip guard:
```go
if os.Getenv("OLLAMA_HOST") == "" {
    t.Skip("requires OLLAMA_HOST")
}
```

### 6. Security checklist before marking done
- [ ] User-visible errors use `Result{IsError: true}`, not a Go `error`
- [ ] Path tools: workspace boundary checked twice (before and after `EvalSymlinks`)
- [ ] Write tools: EvalSymlinks on parent dir (not the file); atomic temp+rename
- [ ] Network tools: SSRF guard applied before HTTP request
- [ ] Output capped per `limits.go` constants

### 7. Verify
```bash
go build ./...
go test ./internal/tools/...
go vet ./...
```
