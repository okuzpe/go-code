package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// BashTool runs a broad but allowlisted set of shell commands with a timeout.
type BashTool struct {
	// timeoutSec is execution cap in seconds; zero means BashTimeoutSec from limits.go.
	timeoutSec int
}

// NewBash returns the bash tool with the default timeout (BashTimeoutSec).
func NewBash() *BashTool { return &BashTool{} }

// NewBashWithTimeout returns a bash tool with the given timeout in seconds.
// Non-positive values fall back to BashTimeoutSec; values above maxBashTimeoutSec are clamped.
func NewBashWithTimeout(timeoutSec int) *BashTool {
	return &BashTool{timeoutSec: normalizeBashTimeoutSec(timeoutSec)}
}

func normalizeBashTimeoutSec(sec int) int {
	const maxBashTimeoutSec = 3600
	if sec <= 0 {
		return 0
	}
	if sec > maxBashTimeoutSec {
		return maxBashTimeoutSec
	}
	return sec
}

func (b BashTool) bashTimeout() time.Duration {
	sec := b.timeoutSec
	if sec <= 0 {
		sec = BashTimeoutSec
	}
	return time.Duration(sec) * time.Second
}

var _ Tool = (*BashTool)(nil)

func (BashTool) Name() string { return "bash" }

func (BashTool) Description() string {
	return "Run one allowlisted shell command (non-interactive, output capped at 256 KiB, 30 s timeout). " +
		"Allowed categories: filesystem (ls, find, stat, mkdir, cp, mv, rm, touch, chmod, diff, tar, zip…), " +
		"text (cat, head, tail, grep, rg, sed, awk, cut, sort, uniq, jq…), " +
		"build (go, make, cmake, cargo, npm, yarn, pnpm, node, npx, bun, python3, pip, uv, java, mvn, gradle, ruby, gem…), " +
		"network (curl, wget), git (status, log, diff, add, commit, push, pull, fetch, checkout, merge, rebase, stash, clone, remote…), " +
		"and utilities (echo, sleep, pwd, date, env, which, uname, gh). " +
		"Prefer read_file / glob / grep tools for pure read operations."
}

func (BashTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type": "string",
				"description": "Single simple command: one allowlisted binary plus arguments. " +
					"No shell pipes (|), command separators (;, &&), redirections (>, <), subshells, or command substitution — " +
					"quote URLs that contain &. Use read_file / glob / grep instead of cat/find/shell grep when possible.",
			},
			"cwd": map[string]any{
				"type":        "string",
				"description": "Working directory (optional, defaults to process cwd)",
			},
		},
		"required": []string{"command"},
	}
}

type bashInput struct {
	Command string `json:"command"`
	Cwd     string `json:"cwd"`
}

var allowedBinaries = map[string]struct{}{
	// Core utilities (original set)
	"ls": {}, "cat": {}, "head": {}, "tail": {}, "pwd": {}, "echo": {}, "sleep": {},
	"go": {}, "wc": {}, "sort": {}, "uniq": {}, "dirname": {}, "basename": {},
	"file": {},

	// Filesystem ops
	"find": {}, "stat": {}, "which": {}, "type": {},
	"mkdir": {}, "cp": {}, "mv": {}, "rm": {}, "touch": {}, "chmod": {},
	"diff": {}, "patch": {},
	"tar": {}, "gzip": {}, "gunzip": {}, "zip": {}, "unzip": {},

	// Text processing
	"grep": {}, "rg": {}, "sed": {}, "awk": {}, "cut": {}, "tr": {}, "xargs": {}, "tee": {},

	// System info
	"date": {}, "env": {}, "printenv": {}, "uname": {},

	// Data / network
	"jq": {}, "curl": {}, "wget": {},

	// Build
	"make": {}, "cmake": {},

	// JS ecosystem
	"npm": {}, "yarn": {}, "pnpm": {}, "node": {}, "npx": {}, "bun": {},

	// Python
	"python3": {}, "python": {}, "pip": {}, "pip3": {}, "uv": {},

	// Rust
	"cargo": {}, "rustc": {},

	// JVM
	"java": {}, "javac": {}, "mvn": {}, "gradle": {},

	// Ruby
	"ruby": {}, "gem": {},

	// GitHub CLI
	"gh": {},
}

var allowedGitSub = map[string]struct{}{
	// Read-only (original set)
	"status": {}, "diff": {}, "log": {}, "branch": {}, "rev-parse": {},
	"show": {}, "grep": {},

	// Write operations
	"add": {}, "commit": {}, "push": {}, "pull": {}, "fetch": {},
	"checkout": {}, "switch": {}, "stash": {}, "reset": {},
	"merge": {}, "rebase": {}, "tag": {}, "init": {}, "clone": {},
	"remote": {}, "blame": {}, "shortlog": {}, "describe": {},
	"apply": {}, "cherry-pick": {}, "format-patch": {}, "am": {}, "worktree": {},
}

// rejectDangerousArgs blocks known argument-injection vectors in allowlisted binaries.
// These binaries accept sub-commands as arguments and can bypass the binary allowlist
// even though the first-token check passes.
func rejectDangerousArgs(cmdLine string) error {
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return nil
	}
	bin := filepath.Base(fields[0])
	args := fields[1:]

	switch bin {
	case "find":
		for _, a := range args {
			switch a {
			case "-exec", "-execdir", "-ok", "-okdir":
				return fmt.Errorf("bash: find %s is not allowed (argument injection risk)", a)
			}
		}

	case "xargs":
		// xargs executes a command — validate that the command it would run is on the allowlist.
		if cmd := xargsCommand(args); cmd != "" {
			sub := filepath.Base(cmd)
			if _, ok := allowedBinaries[sub]; !ok {
				return fmt.Errorf("bash: xargs would execute %q which is not on the allowlist", sub)
			}
		}

	case "go":
		if len(args) > 0 && args[0] == "test" {
			for _, a := range args[1:] {
				if a == "-exec" || strings.HasPrefix(a, "-exec=") {
					return fmt.Errorf("bash: go test -exec is not allowed (argument injection risk)")
				}
			}
		}
	}
	return nil
}

// xargsValueFlags is the set of xargs flags that consume the following argument as a value.
var xargsValueFlags = map[string]struct{}{
	"-n": {}, "-L": {}, "-l": {}, "-s": {}, "-P": {},
	"-I": {}, "-i": {}, "-a": {}, "-E": {}, "-e": {},
	"--max-args": {}, "--max-lines": {}, "--max-chars": {},
	"--max-procs": {}, "--replace": {}, "--arg-file": {}, "--eof": {},
}

// xargsCommand returns the command that xargs would execute: the first non-flag argument
// after skipping flags and their values.
func xargsCommand(args []string) string {
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if _, ok := xargsValueFlags[a]; ok {
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return ""
}

// rejectSSRFInNetworkArgs applies the same SSRF IP checks as web_fetch to URLs found
// in curl and wget argument lists. Blocks requests to RFC1918, loopback, link-local,
// and cloud metadata addresses.
func rejectSSRFInNetworkArgs(cmdLine string) error {
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return nil
	}
	bin := filepath.Base(fields[0])
	if bin != "curl" && bin != "wget" {
		return nil
	}
	for _, f := range fields[1:] {
		if !strings.HasPrefix(f, "http://") && !strings.HasPrefix(f, "https://") {
			continue
		}
		u, err := url.Parse(f)
		if err != nil {
			continue
		}
		host := u.Hostname()
		if host == "" {
			continue
		}
		if err := checkHostResolvedIPs(host); err != nil {
			return fmt.Errorf("bash: %s: blocked URL %q: %w", bin, f, err)
		}
	}
	return nil
}

// Execute implements Tool.
func (b BashTool) Execute(ctx context.Context, input string) (Result, error) {
	var in bashInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: "", IsError: true}, fmt.Errorf("invalid json input: %w", err)
	}
	cmdLine := strings.TrimSpace(in.Command)
	if cmdLine == "" {
		return Result{Content: "command is required", IsError: true}, nil
	}
	if err := validateAllowlistedCommand(cmdLine); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}
	if err := rejectShellMetacharacters(cmdLine); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}
	if err := rejectDangerousArgs(cmdLine); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}
	if err := rejectSSRFInNetworkArgs(cmdLine); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}

	cwd := strings.TrimSpace(in.Cwd)
	if cwd != "" {
		var err error
		cwd, err = filepath.Abs(cwd)
		if err != nil {
			return Result{Content: fmt.Sprintf("cwd: %v", err), IsError: true}, nil
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, b.bashTimeout())
	defer cancel()

	name, args := shellInvocation(cmdLine)
	c := exec.CommandContext(timeoutCtx, name, args...)
	if cwd != "" {
		c.Dir = cwd
	}
	c.Env = os.Environ()
	out, err := c.CombinedOutput()
	if len(out) > MaxBashOutput {
		out = out[:MaxBashOutput]
		out = append(out, []byte("\n[output truncated]")...)
	}
	s := string(out)
	if timeoutCtx.Err() == context.DeadlineExceeded {
		return Result{Content: s + "\n[timeout]", IsError: true}, nil
	}
	if err != nil {
		if s == "" {
			s = err.Error()
		} else {
			s += "\n" + err.Error()
		}
		return Result{Content: s, IsError: true}, nil
	}
	return Result{Content: s, IsError: false}, nil
}

func shellInvocation(commandLine string) (string, []string) {
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("bash"); err == nil {
			return p, []string{"-c", commandLine}
		}
		if p, err := exec.LookPath("sh"); err == nil {
			return p, []string{"-c", commandLine}
		}
		return "cmd", []string{"/C", commandLine}
	}
	return "/bin/bash", []string{"-c", commandLine}
}

func validateAllowlistedCommand(cmdLine string) error {
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return fmt.Errorf("empty command")
	}
	bin := filepath.Base(fields[0])
	if bin == "git" {
		if len(fields) < 2 {
			return fmt.Errorf("git requires a subcommand")
		}
		if _, ok := allowedGitSub[fields[1]]; !ok {
			return fmt.Errorf("git subcommand not allowed: %s", fields[1])
		}
		return nil
	}
	if _, ok := allowedBinaries[bin]; ok {
		return nil
	}
	return fmt.Errorf("command not on allowlist: %s", bin)
}

// rejectShellMetacharacters blocks syntax that would let a first-token allowlist check
// bypass the intended binary set (e.g. "curl … | sh"). Scan is quote-aware so semicolons
// inside git -m "…" and & inside quoted URLs are allowed.
func rejectShellMetacharacters(s string) error {
	const (
		normal = iota
		singleQuoted
		doubleQuoted
		doubleQuotedEscape
	)
	state := normal
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch state {
		case normal:
			switch b {
			case '\'':
				state = singleQuoted
			case '"':
				state = doubleQuoted
			case '|':
				return fmt.Errorf("shell syntax not allowed: pipe (|); use one allowlisted command without pipes")
			case ';':
				return fmt.Errorf("shell syntax not allowed: semicolon (;); use a single command")
			case '\n', '\r':
				return fmt.Errorf("shell syntax not allowed: newlines in command")
			case '`':
				return fmt.Errorf("shell syntax not allowed: backticks")
			case '$':
				if i+1 < len(s) && s[i+1] == '(' {
					return fmt.Errorf("shell syntax not allowed: command substitution $(...) ")
				}
			case '>':
				return fmt.Errorf("shell syntax not allowed: redirection (>); omit redirects in bash tool commands")
			case '<':
				return fmt.Errorf("shell syntax not allowed: redirection (<)")
			case '&':
				if i+1 < len(s) && s[i+1] == '&' {
					return fmt.Errorf("shell syntax not allowed: &&")
				}
				return fmt.Errorf("shell syntax not allowed: & (quote URLs that contain ampersands)")
			case '(':
				return fmt.Errorf("shell syntax not allowed: subshell or grouping ( )")
			}
		case singleQuoted:
			if b == '\'' {
				state = normal
			}
		case doubleQuoted:
			if b == '\\' {
				state = doubleQuotedEscape
				continue
			}
			if b == '"' {
				state = normal
			}
		case doubleQuotedEscape:
			state = doubleQuoted
		}
	}
	return nil
}
