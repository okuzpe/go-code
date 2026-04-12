package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// --- splitFrontmatter ---

func TestSplitFrontmatter_valid(t *testing.T) {
	content := "---\nname: my-agent\n---\nBody text here.\n"
	fm, body, ok := splitFrontmatter(content)
	require.True(t, ok)
	require.Equal(t, "name: my-agent", fm)
	require.Equal(t, "Body text here.\n", body)
}

func TestSplitFrontmatter_noBody(t *testing.T) {
	content := "---\nname: my-agent\n---\n"
	fm, body, ok := splitFrontmatter(content)
	require.True(t, ok)
	require.Equal(t, "name: my-agent", fm)
	require.Equal(t, "", body)
}

func TestSplitFrontmatter_notStartingWithDelimiter(t *testing.T) {
	_, _, ok := splitFrontmatter("# Title\nsome content\n")
	require.False(t, ok)
}

func TestSplitFrontmatter_empty(t *testing.T) {
	_, _, ok := splitFrontmatter("")
	require.False(t, ok)
}

func TestSplitFrontmatter_noClosingDelimiter(t *testing.T) {
	_, _, ok := splitFrontmatter("---\nname: foo\n")
	require.False(t, ok)
}

func TestSplitFrontmatter_noNewlineAfterOpening(t *testing.T) {
	// "---" without a newline — invalid
	_, _, ok := splitFrontmatter("---")
	require.False(t, ok)
}

func TestSplitFrontmatter_multilineBody(t *testing.T) {
	content := "---\nname: test\n---\nLine 1.\nLine 2.\n"
	_, body, ok := splitFrontmatter(content)
	require.True(t, ok)
	require.Equal(t, "Line 1.\nLine 2.\n", body)
}

func TestSplitFrontmatter_CRLF(t *testing.T) {
	content := "---\r\nname: crlf-agent\r\n---\r\nBody line.\r\n"
	fm, body, ok := splitFrontmatter(content)
	require.True(t, ok)
	require.Equal(t, "name: crlf-agent", fm)
	require.Equal(t, "Body line.\n", body)
}

// --- parseAgentFile ---

func TestParseAgentFile_validMinimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	err := os.WriteFile(path, []byte("---\nname: my-agent\n---\n"), 0o600)
	require.NoError(t, err)

	profile, ok := parseAgentFile(path)
	require.True(t, ok)
	require.Equal(t, "my-agent", profile.Name)
}

func TestParseAgentFile_withBodyAndSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := "---\nname: smart-agent\nsystem_prompt: intro\n---\nExtra context.\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	profile, ok := parseAgentFile(path)
	require.True(t, ok)
	require.Equal(t, "smart-agent", profile.Name)
	require.Contains(t, profile.SystemPrompt, "intro")
	require.Contains(t, profile.SystemPrompt, "Extra context.")
}

func TestParseAgentFile_bodyOnlyNoSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := "---\nname: body-agent\n---\nJust a body.\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	profile, ok := parseAgentFile(path)
	require.True(t, ok)
	require.Equal(t, "Just a body.", profile.SystemPrompt)
}

func TestParseAgentFile_invalidMemoryScopeIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := "---\nname: mem-agent\nmemory: production\n---\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	profile, ok := parseAgentFile(path)
	require.True(t, ok)
	require.Equal(t, "mem-agent", profile.Name)
	require.Equal(t, "", profile.MemoryScope)
}

func TestParseAgentFile_withAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := "---\nname: full-agent\nmodel: gpt-4\ntool_allowlist: [read_file, bash]\nread_only: true\n---\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	profile, ok := parseAgentFile(path)
	require.True(t, ok)
	require.Equal(t, "full-agent", profile.Name)
	require.Equal(t, "gpt-4", profile.ModelOverride)
	require.Equal(t, []string{"read_file", "bash"}, profile.ToolAllowlist)
	require.True(t, profile.ReadOnly)
}

func TestParseAgentFile_missingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	require.NoError(t, os.WriteFile(path, []byte("---\nmodel: gpt-4\n---\n"), 0o600))

	_, ok := parseAgentFile(path)
	require.False(t, ok)
}

func TestParseAgentFile_invalidName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	// Capital letters are not allowed
	require.NoError(t, os.WriteFile(path, []byte("---\nname: MyAgent\n---\n"), 0o600))

	_, ok := parseAgentFile(path)
	require.False(t, ok)
}

func TestParseAgentFile_noFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	require.NoError(t, os.WriteFile(path, []byte("# Just a markdown file\n"), 0o600))

	_, ok := parseAgentFile(path)
	require.False(t, ok)
}

func TestParseAgentFile_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	require.NoError(t, os.WriteFile(path, []byte("---\n: bad: yaml: [\n---\n"), 0o600))

	_, ok := parseAgentFile(path)
	require.False(t, ok)
}

func TestParseAgentFile_notExist(t *testing.T) {
	_, ok := parseAgentFile("/nonexistent/path/agent.md")
	require.False(t, ok)
}

// --- LoadCustomProfiles ---

func TestLoadCustomProfiles_nonExistentDir(t *testing.T) {
	profiles, err := LoadCustomProfiles("/nonexistent/dir/agents")
	require.NoError(t, err)
	require.Nil(t, profiles)
}

func TestLoadCustomProfiles_emptyDir(t *testing.T) {
	dir := t.TempDir()
	profiles, err := LoadCustomProfiles(dir)
	require.NoError(t, err)
	require.Empty(t, profiles)
}

func TestLoadCustomProfiles_skipsNonMDFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignored"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o600))

	profiles, err := LoadCustomProfiles(dir)
	require.NoError(t, err)
	require.Empty(t, profiles)
}

func TestLoadCustomProfiles_loadsValidSkipsInvalid(t *testing.T) {
	dir := t.TempDir()
	// Valid
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "good.md"),
		[]byte("---\nname: good-agent\n---\n"),
		0o600,
	))
	// Invalid: no name
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "bad.md"),
		[]byte("---\nmodel: gpt-4\n---\n"),
		0o600,
	))

	profiles, err := LoadCustomProfiles(dir)
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	require.Equal(t, "good-agent", profiles[0].Name)
}

func TestLoadCustomProfiles_skipsSubdirs(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	require.NoError(t, os.Mkdir(subdir, 0o700))

	profiles, err := LoadCustomProfiles(dir)
	require.NoError(t, err)
	require.Empty(t, profiles)
}

// --- AllWithCustom ---

func TestAllWithCustom_emptyDirs(t *testing.T) {
	userDir := t.TempDir()
	projectDir := t.TempDir()

	result, err := AllWithCustom(userDir, projectDir)
	require.NoError(t, err)
	// Must contain at least the built-in profiles
	require.Contains(t, result, "general-purpose")
	require.Contains(t, result, "explore")
	require.Contains(t, result, "coordinator")
}

func TestAllWithCustom_customOverridesBuiltIn(t *testing.T) {
	userDir := t.TempDir()
	projectDir := t.TempDir()

	// Override the "explore" built-in
	content := "---\nname: explore\nsystem_prompt: custom explore\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "explore.md"), []byte(content), 0o600))

	result, err := AllWithCustom(userDir, projectDir)
	require.NoError(t, err)
	require.Equal(t, "custom explore", result["explore"].SystemPrompt)
}

func TestAllWithCustom_projectOverridesUser(t *testing.T) {
	userDir := t.TempDir()
	projectDir := t.TempDir()

	userContent := "---\nname: my-agent\nsystem_prompt: from-user\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "my.md"), []byte(userContent), 0o600))

	projectContent := "---\nname: my-agent\nsystem_prompt: from-project\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "my.md"), []byte(projectContent), 0o600))

	result, err := AllWithCustom(userDir, projectDir)
	require.NoError(t, err)
	require.Equal(t, "from-project", result["my-agent"].SystemPrompt)
}

func TestAllWithCustom_nonExistentDirs(t *testing.T) {
	result, err := AllWithCustom("/nonexistent/user", "/nonexistent/project")
	require.NoError(t, err)
	// Built-ins still present
	require.Contains(t, result, "general-purpose")
}
