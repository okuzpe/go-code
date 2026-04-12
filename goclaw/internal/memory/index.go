package memory

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	indexWarnBytes = 25 * 1024 // 25 KB — warn before injection truncates context
	indexWarnLines = 200       // ~200 entries is the documented ceiling
)

// WriteIndex regenerates MEMORY.md with a one-line summary per entry.
// It logs a warning when the index exceeds the documented size thresholds so
// the user knows to condense old entries before the injected index gets truncated.
func WriteIndex(st *Store) error {
	list, err := st.List()
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("# Memory index\n\n")
	sb.WriteString("Auto-generated listing of memory entries.\n\n")
	for _, e := range list {
		line := fmt.Sprintf("- `%s` — **%s** (%s)", e.Filename, e.Name, e.Type)
		if e.Description != "" {
			line += ": " + e.Description
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	content := sb.String()

	lines := strings.Count(content, "\n")
	if len(content) > indexWarnBytes || lines > indexWarnLines {
		slog.Warn("memory index is large; consider condensing old entries with /memory delete",
			"bytes", len(content), "lines", lines,
			"threshold_bytes", indexWarnBytes, "threshold_lines", indexWarnLines)
	}

	path := filepath.Join(st.dir, "MEMORY.md")
	return os.WriteFile(path, []byte(content), 0o600)
}
