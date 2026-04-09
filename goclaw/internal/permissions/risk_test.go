package permissions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func marshalInput(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// --- RiskScore ---

func TestRiskScore_readTools(t *testing.T) {
	for _, tool := range []string{"read_file", "glob", "grep", "web_search", "todo_write"} {
		require.Equal(t, 0, RiskScore(tool, "{}"), "tool=%s", tool)
	}
}

func TestRiskScore_webFetch(t *testing.T) {
	require.Equal(t, 5, RiskScore("web_fetch", "{}"))
}

func TestRiskScore_stopTask(t *testing.T) {
	require.Equal(t, 5, RiskScore("stop_task", "{}"))
}

func TestRiskScore_spawnAgent(t *testing.T) {
	require.Equal(t, 20, RiskScore("spawn_agent", "{}"))
}

func TestRiskScore_script(t *testing.T) {
	require.Equal(t, 70, RiskScore("script", "{}"))
}

func TestRiskScore_mcpPrefix(t *testing.T) {
	require.Equal(t, 50, RiskScore("mcp__github__list_prs", "{}"))
}

func TestRiskScore_unknownTool(t *testing.T) {
	require.Equal(t, 50, RiskScore("totally_custom_tool", "{}"))
}

func TestRiskScore_bashReadOnly(t *testing.T) {
	input := marshalInput(t, map[string]string{"command": "ls -la"})
	require.Equal(t, 10, RiskScore("bash", input))
}

func TestRiskScore_bashWrite(t *testing.T) {
	input := marshalInput(t, map[string]string{"command": "git commit -m 'fix'"})
	require.Equal(t, 40, RiskScore("bash", input))
}

func TestRiskScore_bashUnknown(t *testing.T) {
	input := marshalInput(t, map[string]string{"command": "npm install"})
	require.Equal(t, 50, RiskScore("bash", input))
}

func TestRiskScore_writeFileWorkspace(t *testing.T) {
	input := marshalInput(t, map[string]string{"path": "src/main.go"})
	require.Equal(t, 60, RiskScore("write_file", input))
}

func TestRiskScore_editFileWorkspace(t *testing.T) {
	input := marshalInput(t, map[string]string{"path": "internal/foo.go"})
	require.Equal(t, 60, RiskScore("edit_file", input))
}

func TestRiskScore_writeFileAbsPath(t *testing.T) {
	input := marshalInput(t, map[string]string{"path": "/etc/passwd"})
	require.Equal(t, 90, RiskScore("write_file", input))
}

func TestRiskScore_writeFileDotDot(t *testing.T) {
	input := marshalInput(t, map[string]string{"path": "../secret.txt"})
	require.Equal(t, 90, RiskScore("write_file", input))
}

// --- bashCategory ---

func TestBashCategory_readOnlyBinary(t *testing.T) {
	for _, cmd := range []string{"ls -la", "cat file.txt", "grep foo bar", "diff a b"} {
		require.Equal(t, bashRead, bashCategory(cmd), "cmd=%q", cmd)
	}
}

func TestBashCategory_gitReadOnly(t *testing.T) {
	for _, cmd := range []string{"git status", "git log --oneline", "git diff HEAD", "git show HEAD"} {
		require.Equal(t, bashRead, bashCategory(cmd), "cmd=%q", cmd)
	}
}

func TestBashCategory_gitWrite(t *testing.T) {
	for _, cmd := range []string{"git commit -m x", "git push origin main", "git checkout -b feat"} {
		require.Equal(t, bashWrite, bashCategory(cmd), "cmd=%q", cmd)
	}
}

func TestBashCategory_gitNoSub(t *testing.T) {
	require.Equal(t, bashWrite, bashCategory("git"))
}

func TestBashCategory_pathPrefixStripped(t *testing.T) {
	require.Equal(t, bashRead, bashCategory("/usr/bin/ls -la"))
}

func TestBashCategory_unknown(t *testing.T) {
	require.Equal(t, bashUnknown, bashCategory("docker run alpine"))
}

func TestBashCategory_emptyCommand(t *testing.T) {
	require.Equal(t, bashUnknown, bashCategory(""))
}

// --- fileWriteRiskScore ---

func TestFileWriteRiskScore_relativePath(t *testing.T) {
	input := marshalInput(t, map[string]string{"path": "cmd/main.go"})
	require.Equal(t, 60, fileWriteRiskScore(input))
}

func TestFileWriteRiskScore_absolutePath(t *testing.T) {
	input := marshalInput(t, map[string]string{"path": "/home/user/file.go"})
	require.Equal(t, 90, fileWriteRiskScore(input))
}

func TestFileWriteRiskScore_backslashAbsolute(t *testing.T) {
	input := marshalInput(t, map[string]string{"path": `\Windows\System32\file`})
	require.Equal(t, 90, fileWriteRiskScore(input))
}

func TestFileWriteRiskScore_dotDot(t *testing.T) {
	input := marshalInput(t, map[string]string{"path": "../outside/file.go"})
	require.Equal(t, 90, fileWriteRiskScore(input))
}

func TestFileWriteRiskScore_invalidJSON(t *testing.T) {
	// On invalid JSON, returns default workspace risk.
	require.Equal(t, 60, fileWriteRiskScore("not-json"))
}

// --- Evaluate coverage gap (global default) ---

func TestPolicy_evaluateGlobalAsk(t *testing.T) {
	p := NewPolicy()
	// No per-tool setting → global ModeAsk
	require.Equal(t, DecisionAsk, p.Evaluate("any_tool"))
}

func TestPolicy_evaluateDenyOverridesGlobal(t *testing.T) {
	p := NewPolicy()
	p.Set("bash", ModeDeny)
	require.Equal(t, DecisionDeny, p.Evaluate("bash"))
	require.Equal(t, DecisionAsk, p.Evaluate("read_file")) // unset → global
}

func TestPolicy_applyConfigModes_valid(t *testing.T) {
	p := NewPolicy()
	err := p.ApplyConfigModes(map[string]string{"bash": "deny", "read_file": "allow"})
	require.NoError(t, err)
	require.Equal(t, DecisionDeny, p.Evaluate("bash"))
	require.Equal(t, DecisionAllow, p.Evaluate("read_file"))
}
