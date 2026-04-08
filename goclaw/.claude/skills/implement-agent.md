---
name: implement-agent
description: Use when the user asks to add, change, or define an agent profile in goclaw.
---

## Add or change an agent profile

### Read first
- `internal/agents/profile.go` — `Profile` and the six built-ins

### Profile shape

```go
type Profile struct {
    Name           string   // unique id, kebab-case in built-ins
    ModelOverride  string   // empty = use cfg.Model()
    ToolAllowlist  []string // nil = all tools; empty slice = no tools
    ReadOnly       bool     // true = orchestrator strips bash
    SystemPrompt   string   // appended to the base prompt
}
```

### Built-in profiles (do not duplicate names)

| Name | Model override | Tools (allowlist) | ReadOnly | Role |
|------|----------------|-------------------|----------|------|
| `general-purpose` | — | all (`nil`) | false | default |
| `explore` | — | read_file, glob, grep, web_fetch, web_search, todo_write | true | cheap exploration |
| `plan` | — | read_file, glob, grep, web_search, todo_write | true | planning — prefer direct plans for greenfield tasks; repo tools when code-aware; web_search only for external facts/docs |
| `verification` | — | read_file, bash, todo_write | false | PASS/FAIL checks |
| `guide` | — | none (`[]string{}`) | true | Q&A only |
| `statusline` | — | none | true | single-line status |

(Align with `profile.go` if this table drifts.)

### New profile

```go
var MyProfile = Profile{
    Name:          "my-profile",
    ModelOverride: "",
    ToolAllowlist: []string{"read_file", "bash"},
    ReadOnly:      false,
    SystemPrompt:  "You are an expert in X. ...",
}
```

Register in `All()`:

```go
profiles := []Profile{
    GeneralPurpose, Explore, Plan, Verification, Guide, StatusLine,
    MyProfile,
}
```

### Rules
- Empty `ModelOverride` inherits global model (preferred).
- `ToolAllowlist == nil` → all registered tools (subject to orchestrator filters).
- `ToolAllowlist` empty slice → no tools.
- `ReadOnly: true` removes bash from the spec list even if listed.
- `SystemPrompt` is appended, not a full replacement.

### Verify
```bash
go build ./...
```
