package permissions

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// Risk score bands (0 = safest; higher = more likely to require confirmation).
const (
	riskScoreNone        = 0
	riskScoreNetwork     = 5
	riskScoreBashRead    = 10
	riskScoreSpawnAgent  = 20
	riskScoreBashWrite   = 40
	riskScoreUnknownTool = 50
	riskScoreMCP         = 50
	riskScoreFileWrite   = 60
	riskScoreScript      = 70
	riskScoreFileAbs     = 90
)

// RiskScore returns a score from 0 (safe, auto-approve) to 100 (dangerous, always prompt).
// Tool calls with score <= Config.YoloThreshold are auto-approved without user interaction.
//
// Scoring table:
//
//	read_file, glob, grep, web_search, todo_write → 0  (pure reads, no side effects)
//	web_fetch                                      → 5  (reads, but external network call)
//	bash (read-only category: ls, cat, git status) → 10
//	spawn_agent                                    → 20 (worker inherits parent approval)
//	bash (write category: git commit, make, go build) → 40
//	mcp__* tools                                   → 50
//	bash (unrecognized command)                    → 50
//	write_file, edit_file, patch (relative / typical paths) → 60
//	script                                         → 70
//	write_file, edit_file, patch (absolute/outside paths) → 90
func RiskScore(toolName, toolInput string) int {
	switch toolName {
	case "read_file", "glob", "grep", "web_search", "todo_write":
		return riskScoreNone

	case "web_fetch":
		return riskScoreNetwork

	case "bash", "run_command":
		return bashRiskScore(toolInput)

	case "spawn_agent":
		return riskScoreSpawnAgent

	case "stop_task":
		return riskScoreNetwork

	case "write_file", "edit_file", "patch":
		return fileWriteRiskScore(toolInput)

	case "script":
		return riskScoreScript
	}

	if strings.HasPrefix(toolName, "mcp__") {
		return riskScoreMCP
	}

	// Unknown tool: treat as medium risk.
	return riskScoreUnknownTool
}

// bashRiskCategory classifies the bash command first token into read-only, write, or unknown.
type bashRiskCategory int

const (
	bashRead    bashRiskCategory = iota // ls, cat, git status, git log, git diff …
	bashWrite                           // git commit, git push, make, go build …
	bashUnknown                         // anything else on the allowlist
)

// readOnlyBinaries are commands that only observe the system and have no persistent side effects.
var readOnlyBinaries = map[string]struct{}{
	"ls": {}, "cat": {}, "head": {}, "tail": {}, "pwd": {}, "echo": {},
	"find": {}, "stat": {}, "which": {}, "type": {}, "file": {},
	"wc": {}, "sort": {}, "uniq": {}, "grep": {}, "rg": {},
	"diff": {}, "basename": {}, "dirname": {},
	"env": {}, "printenv": {}, "uname": {}, "date": {},
	"jq": {},
}

// readOnlyGitSubs are git subcommands with no persistent side effects.
var readOnlyGitSubs = map[string]struct{}{
	"status": {}, "diff": {}, "log": {}, "show": {}, "blame": {},
	"branch": {}, "rev-parse": {}, "shortlog": {}, "describe": {},
	"grep": {}, "remote": {},
}

func bashCategory(command string) bashRiskCategory {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return bashUnknown
	}
	bin := fields[0]
	// Strip path prefix (e.g. /usr/bin/git → git).
	if idx := strings.LastIndexByte(bin, '/'); idx >= 0 {
		bin = bin[idx+1:]
	}

	if _, ok := readOnlyBinaries[bin]; ok {
		return bashRead
	}

	if bin == "git" {
		if len(fields) < 2 {
			return bashWrite
		}
		if _, ok := readOnlyGitSubs[fields[1]]; ok {
			return bashRead
		}
		return bashWrite
	}

	return bashUnknown
}

func bashRiskScore(toolInput string) int {
	var command string
	var parsed struct {
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(toolInput), &parsed) == nil {
		command = parsed.Command
	}

	switch bashCategory(command) {
	case bashRead:
		return riskScoreBashRead
	case bashWrite:
		return riskScoreBashWrite
	default:
		return riskScoreUnknownTool
	}
}

func fileWriteRiskScore(toolInput string) int {
	var parsed struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(toolInput), &parsed) == nil {
		path := strings.TrimSpace(parsed.Path)
		// Absolute paths or explicit parent traversal are higher risk than normal workspace-relative writes.
		// filepath.IsAbs catches Windows "C:\..." paths; explicit prefix checks handle Unix-style
		// absolute paths ("/path") and UNC/root-relative paths ("\path") on Windows where
		// filepath.IsAbs returns false for those forms.
		if filepath.IsAbs(path) || strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) || strings.HasPrefix(path, "..") {
			return riskScoreFileAbs
		}
	}
	return riskScoreFileWrite
}
