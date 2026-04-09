// Package skills discovers SKILL.md files under conventional roots and builds a bounded prompt block.
package skills

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const defaultMaxRunes = 24000
const maxPerFileRunes = 4096

// Collect walks each root (if it exists) for files named SKILL.md (any depth under that root).
// Stops appending once total runes exceed maxRunes.
func Collect(roots []string, maxRunes int) (string, error) {
	if maxRunes <= 0 {
		maxRunes = defaultMaxRunes
	}
	var b strings.Builder
	seen := map[string]struct{}{}
	total := 0
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		st, err := os.Stat(root)
		if err != nil || !st.IsDir() {
			continue
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !strings.EqualFold(d.Name(), "skill.md") {
				return nil
			}
			if _, ok := seen[path]; ok {
				return nil
			}
			seen[path] = struct{}{}
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			body := strings.TrimSpace(string(raw))
			if body == "" {
				return nil
			}
			rs := []rune(body)
			if len(rs) > maxPerFileRunes {
				body = string(rs[:maxPerFileRunes]) + "\n…(truncated)"
			}
			heading := "### " + path + "\n"
			chunk := heading + body + "\n\n"
			add := utf8.RuneCountInString(chunk)
			if total+add > maxRunes {
				return fs.SkipAll
			}
			b.WriteString(chunk)
			total += add
			return nil
		})
		if err != nil {
			return b.String(), err
		}
	}
	return strings.TrimSpace(b.String()), nil
}
