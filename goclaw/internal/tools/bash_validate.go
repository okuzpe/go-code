package tools

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

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

// allowedWindowsCmdExtras are built-in CMD names allowed when goclaw falls back to cmd.exe /C
// (no verified bash.exe / sh.exe on PATH). Keep this minimal and explicit.
var allowedWindowsCmdExtras = map[string]struct{}{
	"dir": {}, "echo": {}, "type": {}, "where": {},
}

// allowedWindowsFallbackBinaries are non-built-in tools that remain valid in cmd.exe fallback mode.
// These are either commonly cross-platform binaries or external executables whose semantics do not
// depend on POSIX shell quoting.
var allowedWindowsFallbackBinaries = map[string]struct{}{
	"bun": {}, "cargo": {}, "cmake": {}, "curl": {}, "gh": {}, "git": {}, "go": {}, "gradle": {},
	"java": {}, "javac": {}, "jq": {}, "make": {}, "mvn": {}, "node": {}, "npm": {}, "npx": {},
	"pip": {}, "pip3": {}, "pnpm": {}, "python": {}, "python3": {}, "rg": {}, "ruby": {}, "rustc": {},
	"tar": {}, "uv": {}, "wget": {}, "yarn": {}, "zip": {}, "unzip": {}, "gem": {},
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

// allowlistBinaryName normalizes the first argv token for allowlist checks.
// On Windows, "git.exe" / "GIT.EXE" is treated as "git".
func allowlistBinaryName(first string) string {
	b := strings.ToLower(filepath.Base(strings.TrimSpace(first)))
	if strings.HasSuffix(b, ".exe") {
		b = strings.TrimSuffix(b, ".exe")
	}
	return b
}

func validateAllowlistedCommand(cmdLine string, shell shellSpec) error {
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return fmt.Errorf("empty command")
	}
	bin := allowlistBinaryName(fields[0])
	if bin == "git" {
		if len(fields) < 2 {
			return fmt.Errorf("git requires a subcommand")
		}
		if _, ok := allowedGitSub[fields[1]]; !ok {
			return fmt.Errorf("git subcommand not allowed: %s", fields[1])
		}
		return validateWindowsFallbackBinary(bin, shell)
	}
	if _, ok := allowedBinaries[bin]; !ok {
		err := fmt.Errorf("command not on allowlist: %s", bin)
		if bashRuntimeGOOS == "windows" && shell.Kind == shellKindCMD {
			return fmt.Errorf("%w\n%s", err, windowsBashToolHint())
		}
		return err
	}
	return validateWindowsFallbackBinary(bin, shell)
}

func validateWindowsFallbackBinary(bin string, shell shellSpec) error {
	if bashRuntimeGOOS != "windows" || shell.Kind != shellKindCMD {
		return nil
	}
	if _, ok := allowedWindowsCmdExtras[bin]; ok {
		return nil
	}
	if _, ok := allowedWindowsFallbackBinaries[bin]; ok {
		return nil
	}
	return fmt.Errorf("bash: %q requires a working POSIX shell on Windows\n%s", bin, windowsBashToolHint())
}

func windowsBashToolHint() string {
	return "Tip: install Git for Windows (Git Bash) or another POSIX shell for ls/cat/grep-style commands. Without it, goclaw uses cmd.exe with a reduced command surface."
}

// rejectDangerousArgs blocks known argument-injection vectors in allowlisted binaries.
// These binaries accept sub-commands as arguments and can bypass the binary allowlist
// even though the first-token check passes.
func rejectDangerousArgs(cmdLine string) error {
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return nil
	}
	bin := allowlistBinaryName(fields[0])
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
		// xargs executes a command - validate that the command it would run is on the allowlist.
		if cmd := xargsCommand(args); cmd != "" {
			sub := allowlistBinaryName(cmd)
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
	bin := allowlistBinaryName(fields[0])
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

// rejectShellMetacharacters blocks syntax that would let a first-token allowlist check
// bypass the intended binary set (e.g. "curl ... | sh"). Scan is quote-aware. In Windows
// cmd.exe fallback, only double quotes protect metacharacters; single quotes are literal text.
func rejectShellMetacharacters(s string, kind shellKind) error {
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
				if kind == shellKindPOSIX {
					state = singleQuoted
				}
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
					return fmt.Errorf("shell syntax not allowed: command substitution $(...)")
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
