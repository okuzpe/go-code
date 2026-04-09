---
name: naming-full-words
description: >-
  Apply goclaw naming rules—full words in identifiers, no lazy abbreviations
  (fun, re, str, etc.). Use when writing or refactoring Go, reviewing names in
  a PR, or when the user wants clearer identifiers.
---

## Naming — full words, no lazy abbreviations

Same policy as `.cursor/rules/naming-full-words.mdc` and `CLAUDE.md` (“Naming — full words”).

### Definition

A **lazy abbreviation** shortens a word by dropping letters **without** a stable Go/stdlib meaning, usually to save typing.

### Before you name something

1. **Call-site test:** Would a reader know what the value represents without scrolling to the definition?
2. **Prefer domain nouns** over type echoes: not `str string`, but `path`, `query`, `markdown`, etc.
3. **Callbacks:** name the role (`handler`, `factory`, `decode`) not `fun` / `fn`.

### Swap table (common mistakes)

| Avoid | Prefer |
|-------|--------|
| `fun`, `fn` | `handler`, `factory`, `parse`, or the actual verb |
| `re`, `res` (result) | `result`, `output`, `summary`, `body`, … |
| `str` (generic) | `text`, `content`, `value`, `raw`, … |
| `num`, `cnt` (vague) | `count`, `total`, `limit`, `offset` |

### Allowed shorts (idiomatic Go)

`ctx`, `err`, `ok`, `i`/`j`/`k`, `t *testing.T`, `tb *testing.B`, `r`/`w` on HTTP handlers, `mu`, tight-scope `buf`, clear single-letter receivers.

### Mini example

```go
// WRONG
func join(re, str string) string { ... }

// CORRECT
func join(prefix, suffix string) string { ... }
```

### After editing

Scan new/changed identifiers for two-letter “half words” and rename unless they match the allowed list above.
