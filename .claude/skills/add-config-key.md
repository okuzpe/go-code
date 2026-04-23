---
name: add-config-key
description: Use when adding a new user-configurable setting to goclaw — field, loader merge, EffectiveXxx accessor, and env var.
---

> **Language:** English only. Rule: `.cursor/rules/agent-artifacts-english.mdc`.

## Adding a new config key to goclaw

### 1. Files to touch (in order)

| File | What to add |
|------|-------------|
| `internal/config/config.go` | Field + constant default + `EffectiveXxx()` method |
| `internal/config/loader.go` | `settingsFile` field + merge block |
| Call site | Use `cfg.EffectiveXxx()` — never read the raw field directly |

---

### 2. `config.go` — field + default + accessor

```go
// In Config struct:
FooBarSetting int  // JSON: foo_bar_setting

// Constants (grouped near the field):
const defaultFooBarSetting = 42

// EffectiveFooBarSetting returns the configured value or the default when unset.
func (c Config) EffectiveFooBarSetting() int {
    if c.FooBarSetting > 0 {
        return c.FooBarSetting
    }
    return defaultFooBarSetting
}
```

Rules:
- Default constants use the prefix `default` + CamelCase name.
- The zero value must be the "unset" sentinel; `EffectiveXxx` maps zero → default.
- Use `> 0` for positive integers, `!= ""` for strings, or a named sentinel for booleans.
- Hard upper bounds: add a `maxFooBarSetting` constant and clamp in `EffectiveXxx`.

---

### 3. `loader.go` — settingsFile + merge

```go
// In settingsFile struct:
FooBarSetting *int `json:"foo_bar_setting,omitempty"`

// In applySettingsFile() merge block (after existing fields):
if sf.FooBarSetting != nil && *sf.FooBarSetting > 0 {
    cfg.FooBarSetting = *sf.FooBarSetting
}
```

Rules:
- Always use a pointer in `settingsFile` — missing key leaves the pointer nil (no override).
- Validate before applying: skip zero, negative, or out-of-range values.
- Do not apply the default here; defaults live only in `EffectiveXxx`.

---

### 4. Env var (optional)

If the setting should be overridable via environment:
```go
// In config.Default() or a dedicated env-read block:
if v := os.Getenv("GOCLAW_FOO_BAR"); v != "" {
    if n, err := strconv.Atoi(v); err == nil && n > 0 {
        cfg.FooBarSetting = n
    }
}
```

Document the variable in the **Environment Variables** table in `CLAUDE.md`.

---

### 5. Call site

```go
// CORRECT — always use the EffectiveXxx accessor
limit := cfg.EffectiveFooBarSetting()

// WRONG — bypasses default and validation
limit := cfg.FooBarSetting
```

---

### 6. Verify

```bash
go build ./...
go test ./internal/config/... -count=1 -run TestConfig
```

Add a test case in `config_test.go` covering: zero value → default, explicit value → respected, out-of-range → clamped.
