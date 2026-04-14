// Package gitdiff runs local git commands for workspace UX (diff summaries).
package gitdiff

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const (
	// DiffStatTimeout caps how long git diff --stat may run (avoid freezing UI or REPL).
	DiffStatTimeout = 4 * time.Second
	// DiffStatMaxBytes limits stderr/stdout size printed after diff --stat.
	DiffStatMaxBytes = 4000
)

// WorktreeDiffStat runs `git -C workdir diff --stat`. Returns empty if workdir is blank,
// git fails, or there is no diff output.
func WorktreeDiffStat(workdir string) string {
	wd := strings.TrimSpace(workdir)
	if wd == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), DiffStatTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", wd, "diff", "--stat")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return ""
	}
	if len(s) > DiffStatMaxBytes {
		s = s[:DiffStatMaxBytes] + "\n… (truncated)"
	}
	return "goclaw: git diff --stat\n" + s
}
