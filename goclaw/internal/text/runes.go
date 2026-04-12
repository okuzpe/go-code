// Package text holds small string helpers shared by UI and other packages.
package text

// TruncateRunes returns s truncated to max runes, appending an ellipsis when shortened.
func TruncateRunes(s string, max int) string {
	return TruncateRunesWithSuffix(s, max, "…")
}

// TruncateRunesWithSuffix returns s truncated to max runes; if shortened, appends suffix (e.g. "\n…(truncated)").
// An empty suffix defaults to the same single-rune ellipsis as TruncateRunes.
func TruncateRunesWithSuffix(s string, max int, suffix string) string {
	if max <= 0 || s == "" {
		return s
	}
	if suffix == "" {
		suffix = "…"
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i] + suffix
		}
		n++
	}
	return s
}

// TruncateUTF8ByBytes returns a prefix of s with byte length <= maxBytes, never splitting a UTF-8 codepoint.
func TruncateUTF8ByBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for len(s) > 0 && s[len(s)-1]&0b1100_0000 == 0b1000_0000 {
		s = s[:len(s)-1]
	}
	return s
}
