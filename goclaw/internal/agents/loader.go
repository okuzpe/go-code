package agents

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// customFrontmatter is the YAML structure parsed from a *.md agent file's frontmatter.
type customFrontmatter struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	ModelOverride   string   `yaml:"model"`
	ToolAllowlist   []string `yaml:"tool_allowlist"`
	DisallowedTools []string `yaml:"disallowed_tools"`
	ReadOnly        bool     `yaml:"read_only"`
	SystemPrompt    string   `yaml:"system_prompt"`
	// MaxTurns caps the orchestrator loop for this agent (0 = use the configured default iteration cap).
	MaxTurns int `yaml:"max_turns"`
	// Memory selects a per-agent memory scope: "user", "project", or "local".
	// Empty means use the global user memory store.
	Memory string `yaml:"memory"`
}

// validAgentName accepts lowercase letters, digits, and hyphens only.
var validAgentName = regexp.MustCompile(`^[a-z0-9-]+$`)

// LoadCustomProfiles scans dir for *.md files with YAML frontmatter and returns parsed Profile instances.
// Files without valid frontmatter are logged and skipped; errors from individual files do not abort the scan.
// Returns an empty slice (not an error) if dir does not exist.
func LoadCustomProfiles(dir string) ([]Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(paths)

	profiles := make([]Profile, 0, len(paths))
	for _, path := range paths {
		profile, ok := parseAgentFile(path)
		if !ok {
			continue
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// parseAgentFile reads a single *.md file, extracts frontmatter, and returns a Profile.
// Returns false if the file cannot be parsed or the profile is invalid.
func parseAgentFile(path string) (Profile, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("custom agent: failed to read file", "path", path, "err", err)
		return Profile{}, false
	}

	content := string(raw)
	frontmatter, body, ok := splitFrontmatter(content)
	if !ok {
		slog.Warn("custom agent: no YAML frontmatter found (must start with ---)", "path", path)
		return Profile{}, false
	}

	var fm customFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		slog.Warn("custom agent: invalid YAML frontmatter", "path", path, "err", err)
		return Profile{}, false
	}

	name := strings.TrimSpace(fm.Name)
	if name == "" {
		slog.Warn("custom agent: missing 'name' field in frontmatter", "path", path)
		return Profile{}, false
	}
	if !validAgentName.MatchString(name) {
		slog.Warn("custom agent: name must match [a-z0-9-]+", "path", path, "name", name)
		return Profile{}, false
	}
	if CanonicalProfileName(name) != strings.ToLower(name) {
		slog.Warn("custom agent: reserved profile alias name", "path", path, "name", name, "use", CanonicalProfileName(name))
		return Profile{}, false
	}

	memRaw := strings.TrimSpace(fm.Memory)
	memScope := normalizeMemoryScope(memRaw)
	if memRaw != "" && memScope == "" {
		slog.Warn("custom agent: unknown memory scope (use user, project, or local)", "path", path, "memory", memRaw)
	}

	maxTurns := fm.MaxTurns
	if maxTurns < 0 {
		maxTurns = 0
	}

	profile := Profile{
		Name:            name,
		Description:     strings.TrimSpace(fm.Description),
		ModelOverride:   strings.TrimSpace(fm.ModelOverride),
		ToolAllowlist:   sanitizeToolNameList(fm.ToolAllowlist),
		DisallowedTools: sanitizeToolNameList(fm.DisallowedTools),
		ReadOnly:        fm.ReadOnly,
		SystemPrompt:    mergeAgentSystemPrompt(fm.SystemPrompt, body),
		MaxTurns:        maxTurns,
		MemoryScope:     memScope,
	}
	slog.Debug("custom agent loaded", "name", name, "path", path)
	return profile, true
}

// mergeAgentSystemPrompt joins YAML system_prompt with the markdown body (after frontmatter).
func mergeAgentSystemPrompt(yamlPrompt, body string) string {
	yamlPrompt = strings.TrimSpace(yamlPrompt)
	body = strings.TrimSpace(body)
	if body == "" {
		return yamlPrompt
	}
	if yamlPrompt == "" {
		return body
	}
	return yamlPrompt + "\n\n" + body
}

// sanitizeToolNameList trims entries and drops blanks. Preserves YAML nil vs empty slice:
// nil → nil (field absent); empty [] → []string{} (explicit no tools); only junk entries → []string{}.
func sanitizeToolNameList(in []string) []string {
	if in == nil {
		return nil
	}
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

// normalizeMemoryScope validates and returns a canonical memory scope value.
// Valid values are "user", "project", "local" (case-insensitive); anything else maps to "".
func normalizeMemoryScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "user":
		return "user"
	case "project":
		return "project"
	case "local":
		return "local"
	default:
		return ""
	}
}

// splitFrontmatter splits a Markdown file into YAML frontmatter and body.
// The file must start with "---" then a newline; the next "---" line closes the frontmatter block.
// CRLF in the source is normalized to LF before parsing so the closing delimiter is found reliably.
// Returns (frontmatter, body, true) on success; ("", "", false) if no valid frontmatter is found.
func splitFrontmatter(content string) (string, string, bool) {
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")

	const delimiter = "---"
	if !strings.HasPrefix(content, delimiter) {
		return "", "", false
	}
	// Find the end of the first "---" line.
	firstEnd := strings.IndexByte(content, '\n')
	if firstEnd < 0 {
		return "", "", false
	}
	rest := content[firstEnd+1:]

	// Find the closing "---".
	closing := strings.Index(rest, "\n"+delimiter)
	if closing < 0 {
		// Try at start of rest (edge case: frontmatter immediately closed).
		if !strings.HasPrefix(rest, delimiter) {
			return "", "", false
		}
		closing = 0
	}

	frontmatter := rest[:closing]
	afterDelimiter := rest[closing+1+len(delimiter):]
	// Skip the rest of the closing "---" line.
	bodyStart := strings.IndexByte(afterDelimiter, '\n')
	var body string
	if bodyStart >= 0 {
		body = afterDelimiter[bodyStart+1:]
	}
	return frontmatter, body, true
}

// AllWithCustom returns built-in profiles merged with user and project custom profiles.
// Priority: project > user > built-in (later wins for the same name).
// The error is always nil; scan failures are logged and skipped.
func AllWithCustom(userAgentsDir, projectAgentsDir string) (map[string]Profile, error) {
	result := All()
	mergeCustomProfilesFromDir(result, userAgentsDir, "user agents dir")
	mergeCustomProfilesFromDir(result, projectAgentsDir, "project agents dir")
	return result, nil
}

func mergeCustomProfilesFromDir(dst map[string]Profile, dir, label string) {
	list, err := LoadCustomProfiles(dir)
	if err != nil {
		slog.Warn("custom agent: failed to scan agents directory", "scope", label, "dir", dir, "err", err)
		return
	}
	for _, p := range list {
		dst[p.Name] = p
	}
}
