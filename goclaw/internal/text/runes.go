// Package text holds small string helpers shared by UI and other packages.
package text

// TruncateRunes returns s truncated to max runes, appending an ellipsis when shortened.
func TruncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i] + "…"
		}
		n++
	}
	return s
}
