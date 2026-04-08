package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteIndex regenerates MEMORY.md with a one-line summary per entry.
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
	path := filepath.Join(st.dir, "MEMORY.md")
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}
