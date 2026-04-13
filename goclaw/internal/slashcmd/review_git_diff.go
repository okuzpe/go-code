package slashcmd

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxReviewGitDiffBytes = 120 << 10 // 120 KiB cap before truncation

// safeReviewGitToken allows git revisions, pathspecs, and a few safe flags for /review.
func safeReviewGitToken(tok string) bool {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return false
	}
	if tok == "--" {
		return true
	}
	if len(tok) >= 3 && tok[0] == '-' && tok[1] == 'U' {
		for _, r := range tok[2:] {
			if r < '0' || r > '9' {
				return false
			}
		}
		return len(tok) > 2
	}
	for _, r := range tok {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '_', r == '~', r == '^', r == '@', r == '/':
		case r == ':', r == '-', r == '*', r == '?', r == '\\':
		default:
			return false
		}
	}
	return true
}

// reviewGitDiffArgv builds a git diff argv slice from slash fields (fields[0] is "/review").
func reviewGitDiffArgv(fields []string) ([]string, error) {
	args := fields[1:]
	if len(args) == 0 {
		return []string{"git", "diff", "HEAD"}, nil
	}
	if len(args) == 1 && (args[0] == "--staged" || args[0] == "--cached") {
		return []string{"git", "diff", "--cached"}, nil
	}
	if len(args) > 4 {
		return nil, fmt.Errorf("too many arguments (max 4 after /review); see docs/goclaw/code-review-workflow.md")
	}
	// git diff <rev1> <rev2>  OR  git diff <rev> -- <path>
	if len(args) == 3 && args[1] == "--" {
		for _, a := range []string{args[0], args[2]} {
			if !safeReviewGitToken(a) {
				return nil, fmt.Errorf("invalid token %q for /review", a)
			}
		}
		return []string{"git", "diff", args[0], "--", args[2]}, nil
	}
	for _, a := range args {
		if !safeReviewGitToken(a) {
			return nil, fmt.Errorf("invalid token %q for /review (allowed: revisions, paths, --, -U<number>)", a)
		}
	}
	out := []string{"git", "diff"}
	out = append(out, args...)
	return out, nil
}

// runReviewGitDiff runs git in workdir and returns the joined command line (for display) and stdout body.
func runReviewGitDiff(ctx context.Context, workdir string, argv []string) (cmdLine string, body string, err error) {
	if len(argv) < 2 || argv[0] != "git" {
		return "", "", fmt.Errorf("internal: expected git argv")
	}
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		return "", "", fmt.Errorf("workspace directory is empty")
	}
	abs, err := filepath.Abs(workdir)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace: %w", err)
	}
	cctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, argv[0], argv[1:]...)
	cmd.Dir = abs
	out, runErr := cmd.Output()
	cmdLine = strings.Join(argv, " ")
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return cmdLine, "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return cmdLine, "", runErr
	}
	s := string(out)
	truncNote := ""
	if len(s) > maxReviewGitDiffBytes {
		s = s[:maxReviewGitDiffBytes]
		truncNote = fmt.Sprintf("\n\n[diff truncated at %d bytes; narrow with /review <rev> -- <path>]\n", maxReviewGitDiffBytes)
	}
	return cmdLine, s + truncNote, nil
}
