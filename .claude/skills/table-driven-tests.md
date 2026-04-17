---
name: table-driven-tests
description: Use when adding or refactoring Go tests in goclaw, especially when the user asks for table tests, many cases, functional/integration tests, golden files, or E2E CLI smoke tests.
---

> **Language:** Author and maintain this file in English only. Rule: `.cursor/rules/agent-artifacts-english.mdc` (paths from the repository root).

## Testing strategy (recommended for goclaw)

- Prefer a **pyramid**:
  - **Unit tests (most)**: pure logic (parsing, formatting, validation, small policy decisions).
  - **Integration tests (some)**: flows across `internal/...` with local fakes/mocks and temp dirs.
  - **E2E smoke tests (few)**: run the CLI process and validate exit codes + stdout/stderr for the main wiring.

Rule of thumb:
- If it can be tested without I/O or processes, make it a **unit** table test.
- If it’s about wiring between packages, make it an **integration** test.
- If it only fails when running the real binary/command parsing/config resolution, add a small **E2E smoke** test.

See also: skill `write-tests` for the mock Anthropic server and orchestrator integration test pointers.

---

## Package choice: `package x` vs `package x_test`

Default for this repo:
- Prefer **`package x`** (same package) for unit + integration tests under `internal/...`.
  - Keeps tests simpler and enables testing internal behavior without exporting test-only APIs.
  - Avoids extra import wiring and tends to reduce boilerplate.

Use **`package x_test`** only when you explicitly want a black-box test that exercises the public API as an external consumer, or when it helps avoid import cycles.

---

## Table-driven tests (default shape)

Use table tests whenever there are multiple cases to cover.

```go
func TestThing(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {name: "empty", input: "", want: "", wantErr: false},
        {name: "bad_input", input: "???", want: "", wantErr: true},
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            got, err := Thing(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            require.Equal(t, tt.want, got)
        })
    }
}
```

Guidelines:
- **Case names**: describe behavior, not numbers (`"rejects_path_outside_workspace"`, not `"case1"`).
- **Avoid test duplication**: expand the table, not copy/paste new test functions.
- **Helper functions**: mark helpers with `t.Helper()` and return useful errors.
- **Temp filesystem**: use `t.TempDir()`; avoid shared global temp paths.

---

## Prefer `cmp.Diff` for complex comparisons

When comparing structs, slices, or maps, prefer `cmp.Diff` for readable failures:

```go
want := SomeStruct{ /* ... */ }
got := BuildSomeStruct()

if diff := cmp.Diff(want, got); diff != "" {
    t.Fatalf("SomeStruct mismatch (-want +got):\n%s", diff)
}
```

---

## Many cases without unreadable tests

When a single test would need dozens of large expected strings:
- Prefer a **golden file** (snapshot) pattern.
- Or keep the table’s `want` small by asserting **key properties** (prefix/suffix/contains/JSON keys) instead of full string equality.

---

## Golden files (snapshot testing) for large outputs

Use golden files for:
- CLI output (stdout/stderr) that is long but stable.
- Tool outputs that are large or multi-line.

Keep golden updates explicit:
- Only rewrite goldens when an opt-in env var is set (example: `UPDATE_GOLDEN=1`).

Conventions:
- Store goldens under `testdata/` in the same package directory.
- Prefer deterministic output (stable ordering, trimmed trailing whitespace, normalized newlines if needed).

---

## Anti-flakiness rules (strict)

- No sleeps/timeouts in tests unless unavoidable.
- No network calls in default `go test ./...`.
- Tests that require external dependencies must `t.Skip` unless explicitly enabled via env vars.
- Avoid package-level mutable globals in tests; if unavoidable, use `t.Cleanup` to restore state.

If production code has configurable delays (like a streaming mock), provide a test helper to set delays to zero and restore via `t.Cleanup` (see existing patterns in the repo).

---

## `t.Parallel()` (use carefully)

Safe to use only when:
- The test case is isolated (no shared env vars, no shared globals, no shared files).
- All filesystem writes are under `t.TempDir()`.

If in doubt, don’t parallelize.

---

## E2E CLI smoke tests (small and stable)

Goal: catch broken wiring (flags, command dispatch, config paths) with a handful of tests.

Guidelines:
- Keep E2E count small (2–5).
- Assert **exit code** and a few stable output substrings (not the entire output).
- Isolate filesystem with `t.TempDir()` and env vars.
- Never require real network or LLM access in E2E by default.

