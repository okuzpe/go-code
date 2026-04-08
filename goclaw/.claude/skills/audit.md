---
name: audit
description: Use at the end of any development phase, before closing a milestone, or when the user asks for a quality/security audit of goclaw.
---

## End-of-Phase Audit for goclaw

Run this checklist before closing any development phase. Fix every failing item before marking the phase done.

---

### 1. Build & Tests

```bash
# Must pass with zero errors and zero warnings
go build ./...

# Must pass — including race detector
go test -race ./...

# Vet — must be clean
go vet ./...
```

If any of these fail, stop the audit and fix first.

---

### 2. Language Compliance
- [ ] No Spanish (or any non-English) in code comments
- [ ] No Spanish in error messages or log strings
- [ ] No Spanish in variable/function/type names
- [ ] Commit messages in English

Quick check:
```bash
# Search for common Spanish words in Go files
grep -rn "// .*[áéíóúñü]" --include="*.go" .
```

---

### 3. Code Quality

```bash
# Install staticcheck if not present: go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...

# Check for goroutine leaks in packages that spawn goroutines
# (add go.uber.org/goleak to test files when needed)
```

- [ ] No unused variables or imports
- [ ] No `fmt.Println` in non-main packages
- [ ] No `log.Fatal` / `log.Panic` outside of `cmd/`
- [ ] All exported symbols have doc comments
- [ ] No `TODO` or `FIXME` left from the current phase (previous phases are ok)

---

### 4. Error Handling
- [ ] No ignored errors (`_ = someFunc()`)
- [ ] All errors wrapped with context (`fmt.Errorf("action: %w", err)`)
- [ ] Error strings: lowercase, no trailing punctuation
- [ ] Errors matched with `errors.Is` / `errors.As`, not `==`

```bash
# Find ignored errors
grep -rn "_ = " --include="*.go" .
# Find bare "return err" without wrapping (heuristic)
grep -n "return err$" --include="*.go" -r .
```

---

### 5. Security Checklist

**read_file / glob / grep:**
- [ ] Path canonicalized and workspace boundary checked (before AND after `EvalSymlinks`)
- [ ] Symlinks resolved and re-validated against workspace root
- [ ] Output capped: 512 KiB or 200 lines (read_file); 500 matches (glob); 200 matches (grep)

**bash:**
- [ ] Command validated against binary allowlist (`allowedBinaries` / `allowedGitSub`) — not denylist
- [ ] `rejectShellMetacharacters` called (blocks `|`, `;`, `&&`, `>`, `<`, `$(...)`, unquoted `&`, subshells)
- [ ] Timeout applied (default 30s or `bash_timeout_sec` from config)
- [ ] Output truncated at 256 KiB
- [ ] Argument-injection risk noted for `find -exec`, `xargs`, `go test -exec` — do not add `-exec`/`-execdir`/`--exec` flags without further validation

**write_file / edit_file:**
- [ ] Path boundary checked via `EvalSymlinks` on **parent directory** (file may not exist yet)
- [ ] Content capped at 1 MiB
- [ ] Atomic write used (temp file → chmod → rename)
- [ ] Stripped from ReadOnly profiles (both spec list and `executeTool` guard)

**web_fetch:**
- [ ] RFC1918 IPs blocked (10/8, 172.16/12, 192.168/16)
- [ ] 169.254.169.254 and IPv6 link-local blocked
- [ ] Redirect re-validation (max 5 hops)
- [ ] Output capped at 1 MiB

**MCP:**
- [ ] `json.Valid()` checked on arguments before writing to subprocess stdin
- [ ] Per-call `context.WithTimeout` used in `CallTool` (not only session-level context)
- [ ] MCP tools stripped from ReadOnly profiles (`stripMCPNames`)

**Permissions:**
- [ ] Global default is `ModeAsk` (not `ModeAllow`)
- [ ] `bypassPermissions` not present in any code

---

### 6. Interface Contracts
- [ ] Every new type that implements an interface has a compile-time check:
  ```go
  var _ Tool = (*MyTool)(nil)
  ```
- [ ] All new `Tool` implementations registered in **`internal/app/run.go`** (not `main.go`)
- [ ] All new `Profile` entries added to `agents.All()`

---

### 7. Test Coverage
- [ ] Each new package has at least one test file
- [ ] Each new `Tool` has at least: happy path + invalid input + boundary case
- [ ] Mock server scenarios updated if new LLM interactions added

```bash
go test -cover ./...
# Target: >70% coverage on internal/ packages
```

---

### 8. Configuration & Paths
- [ ] No hardcoded absolute paths
- [ ] No hardcoded token counts (use `cfg.AutoCompactThreshold`)
- [ ] New config fields have defaults in `config.Default()`
- [ ] New env vars documented in `CLAUDE.md`

---

### 9. Roadmap Update
- [ ] Mark completed items in the roadmap section of `CLAUDE.md`
- [ ] Update "Current Phase" section to reflect next phase
- [ ] Update any TODO comments that reference the completed phase

---

### Audit Sign-off

Phase is considered **done** when all checkboxes above pass.

```
Audit date:    ___________
Phase:         ___________
go test -race: PASS / FAIL
staticcheck:   PASS / FAIL
Security:      PASS / FAIL
Notes:         ___________
```
